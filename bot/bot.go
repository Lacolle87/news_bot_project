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
		sendMessage(message.Chat.ID, reply, bot, redis, logger)
	}
}

// sendMessage отправляет сообщение указанному chatID.
func sendMessage(chatID int64, message string, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке сообщения: " + err.Error())
		return
	}

	err = saveSentNews(context.Background(), redis, chatID, message, logger)
	if err != nil {
		logger.Log("Ошибка при сохранении отправленного сообщения: " + err.Error())
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
		sendMessage(chatID, reply, bot, redis, logger)
		return
	}

	// Заносим идентификатор чата в Redis
	_, err = redis.SAdd(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		logger.Log("Ошибка при сохранении идентификатора чата в Redis: " + err.Error())
		return
	}

	reply := "Добро пожаловать! Вы успешно подписались на новостного бота."
	sendMessage(chatID, reply, bot, redis, logger)

	sendInitialNews(chatID, bot, redis, logger)
}

// sendInitialNews отправляет пользователю одну случайную новость при подписке.
func sendInitialNews(chatID int64, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
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
	sendMessage(chatID, news, bot, redis, logger)

	// Создание снимка идентификаторов чатов, если снимок не был создан ранее
	err = createChatIDSnapshot(redis, logger)
	if err != nil {
		logger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
	}
}

// handleGetNews обрабатывает команду /getnews.
func handleGetNews(chatID int64, bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	news, err := redis.SRandMember(context.Background(), "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		return
	}

	if news == "" {
		reply := "Извините, пока нет доступных новостей."
		sendMessage(chatID, reply, bot, redis, logger)
		return
	}

	sendMessage(chatID, news, bot, redis, logger)
}

// sendRandomNews отправляет случайную новость каждому подписанному пользователю.
func sendRandomNews(bot *tgbotapi.BotAPI, redis *redis.Client, logger *logger.Logger) {
	chatIDs, err := redis.SMembers(context.Background(), "chat_ids").Result()
	if err != nil {
		logger.Log("Ошибка при получении идентификаторов чатов из Redis: " + err.Error())
		return
	}

	news, err := redis.SRandMember(context.Background(), "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении новости из Redis: " + err.Error())
		return
	}

	for _, chatIDStr := range chatIDs {
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)
		sendMessage(chatID, news, bot, redis, logger)
	}
	logger.Log("Отправлены новости всем зарегистрированным чатам.")
}

// saveSentNews сохраняет отправленную новость в Redis.
func saveSentNews(ctx context.Context, redis *redis.Client, chatID int64, news string, logger *logger.Logger) error {
	_, err := redis.RPush(ctx, fmt.Sprintf("sent_news:%d", chatID), news).Result()
	if err != nil {
		logger.Log("Ошибка при сохранении отправленной новости в Redis: " + err.Error())
		return err
	}

	return nil
}

// createChatIDSnapshot создает снимок идентификаторов чатов в Redis, если снимок еще не создан.
func createChatIDSnapshot(redis *redis.Client, logger *logger.Logger) error {
	exists, err := redis.Exists(context.Background(), "chat_ids_snapshot").Result()
	if err != nil {
		logger.Log("Ошибка при проверке существования снимка идентификаторов чатов: " + err.Error())
		return err
	}

	if exists == 0 {
		_, err = redis.SUnionStore(context.Background(), "chat_ids_snapshot", "chat_ids").Result()
		if err != nil {
			logger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
			return err
		}
	}

	return nil
}
