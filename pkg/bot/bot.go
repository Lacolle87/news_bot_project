package bot

import (
	"context"
	"fmt"
	"math/rand"
	"news_bot_project/pkg/loader"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// StartBot запускает новостного бота.
func StartBot(redis *redis.Client, botToken string) error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		loader.BotLogger.Log("Ошибка при создании Telegram бота: " + err.Error())
		return err
	}

	bot.Debug = false
	loader.BotLogger.Log("Бот запущен.")

	go func() {
		for {
			sendRandomNews(bot, redis, false)
			time.Sleep(45 * time.Minute)
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении обновлений от Telegram: " + err.Error())
		return err
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			handleCommand(update.Message, bot, redis)
		}
	}

	return nil
}

// handleCommand обрабатывает команды от пользователя.
func handleCommand(message *tgbotapi.Message, bot *tgbotapi.BotAPI, redis *redis.Client) {
	switch message.Command() {
	case "start":
		handleStart(message, bot, redis)
	case "getnews":
		handleGetNews(message.Chat.ID, bot, redis)
	default:
		reply := "Неизвестная команда. Доступные команды: /start, /getnews"
		sendMessage(message.Chat.ID, reply, bot, redis, false)
	}
}

// sendMessage отправляет сообщение указанному chatID.
func sendMessage(chatID int64, message string, bot *tgbotapi.BotAPI, redis *redis.Client, save bool) {
	msg := tgbotapi.NewMessage(chatID, message)
	_, err := bot.Send(msg)
	if err != nil {
		loader.BotLogger.Log("Ошибка при отправке сообщения: " + err.Error())
		return
	}

	if save {
		err = saveSentNews(context.Background(), redis, chatID, message)
		if err != nil {
			loader.BotLogger.Log("Ошибка при сохранении отправленного сообщения: " + err.Error())
		}
	}
}

func handleStart(message *tgbotapi.Message, bot *tgbotapi.BotAPI, redis *redis.Client) {
	chatID := message.Chat.ID

	exists, err := redis.SIsMember(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при проверке идентификатора чата в Redis: " + err.Error())
		return
	}

	if exists {
		reply := "Вы уже подписаны на новостного бота."
		sendMessage(chatID, reply, bot, redis, false)
		return
	}

	_, err = redis.SAdd(context.Background(), "chat_ids", strconv.FormatInt(chatID, 10)).Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при сохранении идентификатора чата в Redis: " + err.Error())
		return
	}

	err = createChatIDSnapshot(redis) // Вызов функции createChatIDSnapshot
	if err != nil {
		loader.BotLogger.Log("Ошибка при создании снимка идентификаторов чатов: " + err.Error())
	}

	reply := "Добро пожаловать! Вы успешно подписались на новостного бота."
	sendMessage(chatID, reply, bot, redis, false)

	time.Sleep(5 * time.Second)

	sendRandomNews(bot, redis, true)
}

// handleGetNews обрабатывает команду /getnews.
func handleGetNews(chatID int64, bot *tgbotapi.BotAPI, redis *redis.Client) {
	news, err := redis.SRandMember(context.Background(), "news").Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении новости из Redis: " + err.Error())
		reply := "Извините, пока нет доступных новостей."
		sendMessage(chatID, reply, bot, redis, false)
		return
	}

	sendMessage(chatID, news, bot, redis, true)
}

// sendRandomNews отправляет случайную новость всем зарегистрированным чатам.
func sendRandomNews(bot *tgbotapi.BotAPI, redis *redis.Client, initial bool) {
	ctx := context.Background()

	// Получаем все новости из Redis
	allNews, err := redis.SMembers(ctx, "news").Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении всех новостей из Redis: " + err.Error())
		return
	}

	// Получаем уже отправленные новости для каждого чата
	chatIDs, err := redis.SMembers(ctx, "chat_ids").Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении идентификаторов чатов из Redis: " + err.Error())
		return
	}

	// Выбираем случайную новость из доступных и инициализируем ее как пустую строку
	var randomNews string

	for _, chatIDStr := range chatIDs {
		chatID, _ := strconv.ParseInt(chatIDStr, 10, 64)

		// Получаем уже отправленные новости для данного чата
		sentNews, err := redis.SMembers(ctx, fmt.Sprintf("sent_news:%d", chatID)).Result()
		if err != nil {
			loader.BotLogger.Log("Ошибка при получении отправленных новостей из Redis: " + err.Error())
			continue
		}

		// Составляем список доступных новостей, исключая уже отправленные
		availableNews := make([]string, 0)
		for _, news := range allNews {
			if !contains(sentNews, news) {
				availableNews = append(availableNews, news)
			}
		}

		// Проверяем, есть ли доступные новости для отправки
		if len(availableNews) == 0 {
			loader.BotLogger.Log(fmt.Sprintf("Нет доступных новостей для отправки в чат %d.", chatID))
			continue
		}

		// Если нет отправленной новости для данного чата, выбираем случайную из доступных
		if randomNews == "" {
			randomNews = availableNews[rand.Intn(len(availableNews))]
		}

		// Отправляем новость чату
		sendMessage(chatID, randomNews, bot, redis, true)

		// Добавляем идентификатор новости в таблицу отправленных новостей для данного чата
		redis.SAdd(ctx, fmt.Sprintf("sent_news:%d", chatID), randomNews)
	}

	if randomNews == "" {
		loader.BotLogger.Log("Нет доступных новостей для отправки.")
		return
	}

	if initial {
		loader.BotLogger.Log("Отправлена новость при подписке на бота.")
	} else {
		loader.BotLogger.Log("Отправлена новость всем зарегистрированным чатам.")
	}
}

// Вспомогательная функция для проверки наличия элемента в срезе
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// saveSentNews сохраняет отправленную новость в Redis с TTL 48 часов.
func saveSentNews(ctx context.Context, redis *redis.Client, chatID int64, news string) error {
	key := fmt.Sprintf("sent_news:%d", chatID)
	duration := 96 * time.Hour

	_, err := redis.SAdd(ctx, key, news).Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при сохранении отправленной новости в Redis: " + err.Error())
		return err
	}

	// Устанавливаем время жизни ключа
	_, err = redis.Expire(ctx, key, duration).Result()
	if err != nil {
		loader.BotLogger.Log("Ошибка при установке TTL для ключа в Redis: " + err.Error())
		return err
	}
	return nil
}

// createChatIDSnapshot создает снимок идентификаторов чатов, используя команду BGSAVE Redis.
func createChatIDSnapshot(redis *redis.Client) error {
	ctx := context.Background()

	// Запускаем фоновое сохранение набора данных
	err := redis.BgSave(ctx).Err()
	if err != nil {
		return err
	}

	loader.BotLogger.Log("Создан снимок идентификаторов чатов.")

	return nil
}
