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
			sendRandomNews(bot, redis, logger, false)
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
		sendMessage(message.Chat.ID, reply, bot, redis, logger, false)
	}
}

// sendMessage отправляет сообщение указанному chatID.
func sendMessage(chatID int64, message string, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger, save bool) {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке сообщения: " + err.Error())
		return
	}

	if save {
		err = saveSentNews(context.Background(), redis, chatID, message, logger)
		if err != nil {
			logger.Log("Ошибка при сохранении отправленного сообщения: " + err.Error())
		}
	}
}

func handleStart(message *tgbotapi.Message, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	chatID := message.Chat.ID

	exists, err := redis.SIsMember(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		logger.Log("Ошибка при проверке идентификатора чата в Redis: " + err.Error())
		return
	}

	if exists {
		reply := "Вы уже подписаны на новостного бота."
		sendMessage(chatID, reply, bot, redis, logger, false)
		return
	}

	_, err = redis.SAdd(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		logger.Log("Ошибка при сохранении идентификатора чата в Redis: " + err.Error())
		return
	}

	err = createChatIDSnapshot(redis, logger) // Вызов функции createChatIDSnapshot
	if err != nil {
		logger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
	}

	reply := "Добро пожаловать! Вы успешно подписались на новостного бота."
	sendMessage(chatID, reply, bot, redis, logger, false)

	time.Sleep(5 * time.Second)

	sendRandomNews(bot, redis, logger, true)
}

// handleGetNews обрабатывает команду /getnews.
func handleGetNews(chatID int64, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	news, err := redis.SRandMember(context.Background(), "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		reply := "Извините, пока нет доступных новостей."
		sendMessage(chatID, reply, bot, redis, logger, false)
		return
	}

	sendMessage(chatID, news, bot, redis, logger, true)
}

func sendRandomNews(bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger, initial bool) {
	// Получаем одну новость из Redis
	ctx := context.Background()
	news, err := redis.SRandMember(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		return
	}

	chatIDs, err := redis.SMembers(ctx, "chat_ids").Result()
	if err != nil {
		logger.Log("Ошибка при получении идентификаторов чатов из Redis: " + err.Error())
		return
	}

	for _, chatIDStr := range chatIDs {
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
		sendMessage(chatID, news, bot, redis, logger, true)
	}

	if initial {
		logger.Log("Отправлена новость при подписке на бота.")
	} else {
		logger.Log("Отправлены новости всем зарегистрированным чатам.")
	}
}

// saveSentNews сохраняет отправленную новость в Redis с TTL 48 часов.
func saveSentNews(ctx context.Context, redis *redis.Client, chatID int64, news string, logger *logger.Logger) error {
	key := fmt.Sprintf("sent_news:%d", chatID)
	duration := 48 * time.Hour

	_, err := redis.SAdd(ctx, key, news).Result()
	if err != nil {
		logger.Log("Ошибка при сохранении отправленной новости в Redis: " + err.Error())
		return err
	}

	// Устанавливаем время жизни ключа
	_, err = redis.Expire(ctx, key, duration).Result()
	if err != nil {
		logger.Log("Ошибка при установке TTL для ключа в Redis: " + err.Error())
		return err
	}

	return nil
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
