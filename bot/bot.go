package bot

import (
	"context"
	"news_bot_project/logger"
	"strconv"

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
		reply := "Вы уже подписаны на новостной бот."
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

	// Создание снимка идентификаторов чатов, если снимок не был создан ранее
	err = TakeSnapshotIfNeeded(redis, logger)
	if err != nil {
		logger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
	}

	reply := "Добро пожаловать! Вы успешно подписались на новостной бот."
	msg := tgbotapi.NewMessage(chatID, reply)
	_, err = bot.Send(msg)
	if err != nil {
		logger.Log("Ошибка при отправке ответа на команду: " + err.Error())
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

// TakeSnapshotIfNeeded создает снимок идентификаторов чатов, если снимок не был создан ранее.
func TakeSnapshotIfNeeded(redis *redis.Client, logger *logger.Logger) error {
	ctx := context.Background()

	// Проверяем, существует ли снимок идентификаторов чатов
	exists, err := redis.Exists(ctx, "chat_ids_snapshot").Result()
	if err != nil {
		return err
	}

	if exists == 0 {
		// Создаем снимок идентификаторов чатов
		err := createChatIDSnapshot(ctx, redis, logger)
		if err != nil {
			return err
		}
	}

	return nil
}

// createChatIDSnapshot создает снимок идентификаторов чатов.
func createChatIDSnapshot(ctx context.Context, redis *redis.Client, logger *logger.Logger) error {
	// Создаем снимок идентификаторов чатов
	err := redis.SUnionStore(ctx, "chat_ids_snapshot", "chat_ids").Err()
	if err != nil {
		return err
	}

	logger.Log("Создан снимок идентификаторов чатов.")

	return nil
}
