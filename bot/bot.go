package bot

import (
	"context"
	"fmt"
	"news_bot_project/logger"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartBot запускает новостного бота.
func StartBot(redis *redis.Client, botToken string, logger *logger.Logger) error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		logger.Log("Ошибка при создании Telegram бота: " + err.Error())
		return err
	}

	bot.Debug = false
	logger.Log("Бот запущен.")

	go func() {
		for {
			sendRandomNews(bot, redis, logger)
			time.Sleep(1 * time.Minute)
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		logger.Log("Ошибка при получении обновлений от Telegram: " + err.Error())
		return err
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(update.Message, bot, redis, logger)
		}
	}

	return nil
}

// handleCommand обрабатывает команды от пользователя.
func handleCommand(message *tgbotapi.Message, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	switch message.Command() {
	case "start":
		handleStart(message, bot, redis, logger)
	case "getnews":
		handleGetNews(message.Chat.ID, bot, redis, logger)
	default:
		reply := "Неизвестная команда. Доступные команды: /start, /getnews"
		msg := tgbotapi.NewMessage(message.Chat.ID, reply)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Log("Ошибка при отправке ответа на команду: " + err.Error())
		}
	}
}

// handleStart обрабатывает команду /start.
func handleStart(message *tgbotapi.Message, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	chatID := message.Chat.ID

	// Проверяем, существует ли идентификатор чата уже в Redis
	exists, err := redis.SIsMember(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		logger.Log("Ошибка при проверке идентификатора чата в Redis: " + err.Error())
		return
	}

	if exists {
		reply := "Вы уже подписаны на новостного бота."
		msg := tgbotapi.NewMessage(chatID, reply)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Log("Ошибка при отправке ответа на команду: " + err.Error())
		}
		return
	}

	// Заносим идентификатор чата в Redis
	_, err = redis.SAdd(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		logger.Log("Ошибка при сохранении идентификатора чата в Redis: " + err.Error())
		return
	}

	reply := "Добро пожаловать! Вы успешно подписались на новостного бота."
	msg := tgbotapi.NewMessage(chatID, reply)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке ответа на команду: " + err.Error())
		return
	}

	// Задержка 5 секунд
	time.Sleep(5 * time.Second)

	// Получаем одну новость из Redis
	ctx := context.Background()
	news, err := redis.SRandMember(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		return
	}

	// Отправляем новость пользователю
	msg = tgbotapi.NewMessage(chatID, news)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке новости: " + err.Error())
		return
	}

	// Создаем первую запись новостей для данного chatID
	err = redis.SAdd(ctx, fmt.Sprintf("sent_news:%d", chatID), news).Err()
	if err != nil {
		logger.Log("Ошибка при создании записи новостей: " + err.Error())
		return
	}

	// Создание снимка идентификаторов чатов, если снимок не был создан ранее
	err = createChatIDSnapshot(redis, logger)
	if err != nil {
		logger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
	}
}

// handleGetNews обрабатывает команду /getnews.
func handleGetNews(chatID int64, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	// Получаем одну новость из Redis
	ctx := context.Background()
	news, err := redis.SRandMember(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		return
	}

	// Отправляем новость пользователю
	msg := tgbotapi.NewMessage(chatID, news)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке новости: " + err.Error())
	}
}

// createChatIDSnapshot создает снимок идентификаторов чатов, используя команду BGSAVE Redis.
func createChatIDSnapshot(redis *redis.Client, logger *logger.Logger) error {
	ctx := context.Background()

	// Запускаем фоновое сохранение набора данных
	err := redis.BgSave(ctx).Err()
	if err != nil {
		return err
	}

	logger.Log("Создан снимок идентификаторов чатов.")

	return nil
}

// sendRandomNews отправляет каждому зарегистрированному chatID свою случайную новость.
// sendRandomNews отправляет случайную новость всем зарегистрированным чатам.
func sendRandomNews(bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	ctx := context.Background()

	// Получаем все зарегистрированные chatID из Redis
	chatIDs, err := redis.SMembers(ctx, "chat_ids").Result()
	if err != nil {
		logger.Log("Ошибка при получении зарегистрированных chatID из Redis: " + err.Error())
		return
	}

	// Получаем случайную новость из Redis
	news, err := redis.SRandMember(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении случайной новости из Redis: " + err.Error())
		return
	}

	// Отправляем новость каждому зарегистрированному chatID
	for _, chatIDStr := range chatIDs {
		chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
		if err != nil {
			logger.Log("Ошибка при преобразовании chatID из строки в число: " + err.Error())
			continue
		}

		// Проверяем, была ли уже отправлена эта новость для данного chatID
		exists, err := redis.SIsMember(ctx, fmt.Sprintf("sent_news:%s", chatIDStr), news).Result()
		if err != nil {
			logger.Log("Ошибка при проверке отправки новости для chatID: " + err.Error())
			continue
		}

		if exists {
			// Новость уже отправлена, пропускаем отправку
			continue
		}

		// Отправляем новость пользователю
		msg := tgbotapi.NewMessage(chatID, news)
		_, err = bot.Send(msg)
		if err != nil {
			logger.Log("Ошибка при отправке новости: " + err.Error())
			continue
		}

		// Записываем информацию о отправленной новости для данного chatID
		err = redis.SAdd(ctx, fmt.Sprintf("sent_news:%s", chatIDStr), news).Err()
		if err != nil {
			logger.Log("Ошибка при записи информации о отправленной новости: " + err.Error())
			continue
		}
	}

	logger.Log("Отправка новостей завершена.")
}
