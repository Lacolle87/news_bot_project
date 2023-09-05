package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"news_bot_project/loader"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"news_bot_project/bot"
)

// Замените эти значения на свои
var (
	redisHost     string
	redisPassword string
)

// RSS Замените на свои структуры, соответствующие структуре XML-данных RSS-канала
type RSS struct {
	Channel Channel `xml:"channel"`
}

type Channel struct {
	Items []Item `xml:"item"`
}

type Item struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
}

func main() {

	// Загрузка логгера
	err := loader.LoadLoggerFromConfig()
	if err != nil {
		log.Println(err)
	}

	err = godotenv.Load()
	if err != nil {
		loader.BotLogger.Log("Ошибка при загрузке файла .env: " + err.Error())
		return
	}

	loader.BotLogger.Log("Программа запущена. Новостной бот начинает работу.")

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	redisClient := setupRedisClient()
	defer func(redisClient *redis.Client) {
		err := redisClient.Close()
		if err != nil {
			loader.BotLogger.Log("Ошибка закрытия соединения Redis" + err.Error())
		}
	}(redisClient)

	processNews(redisClient)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			processNews(redisClient)
		}
	}()

	go func() {
		err := bot.StartBot(redisClient, botToken)
		if err != nil {
			loader.BotLogger.Log("Ошибка при запуске бота: " + err.Error())
			return
		}
	}()

	select {}
}

func setupRedisClient() *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:            redisHost,
		Password:        redisPassword,
		DB:              0,
		MaxRetries:      3,
		MinRetryBackoff: 500 * time.Millisecond,
		MaxRetryBackoff: 3 * time.Second,
		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			_, err := cn.Ping(ctx).Result()
			return err
		},
	})

	return redisClient
}

// processNews обрабатывает новостные данные из RSS-ленты и сохраняет их в Redis.
func processNews(redisClient *redis.Client) {
	ctx := context.Background()

	// Получаем данные RSS
	rssData, err := fetchRSS()
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении данных RSS: " + err.Error())
		return
	}

	// Разбираем данные RSS
	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		loader.BotLogger.Log("Ошибка при разборе данных RSS: " + err.Error())
		return
	}

	// Инициализируем счетчик добавленных новостей
	addedCount := 0

	// Обрабатываем каждый элемент в RSS-ленте
	for _, item := range rss.Channel.Items {
		// Проверяем, существует ли новость уже в Redis
		newsKey := item.Title + ". " + item.Description
		if strings.HasSuffix(item.Title, ".") || strings.HasSuffix(item.Title, "!") || strings.HasSuffix(item.Title, "?") {
			newsKey = item.Title + " " + item.Description
		}

		exists, err := redisClient.SIsMember(ctx, "news", newsKey).Result()
		if err != nil {
			loader.BotLogger.Log("Ошибка Redis SIsMember: " + err.Error())
			continue
		}

		// Если новость не существует в Redis, сохраняем её
		if !exists {
			err := saveNewsToRedis(ctx, redisClient, item)
			if err != nil {
				loader.BotLogger.Log("Ошибка при сохранении новости в Redis: " + err.Error())
				continue
			}
			addedCount++ // Увеличиваем счетчик добавленных новостей
		}
	}

	// Выводим количество добавленных новостей
	if addedCount > 0 {
		loader.BotLogger.Log(fmt.Sprintf("Добавлено новостей: %d", addedCount))
	}
}

func fetchRSS() (string, error) {
	resp, err := http.Get("https://news.mail.ru/rss/")
	if err != nil {
		loader.BotLogger.Log("Ошибка при получении данных RSS: " + err.Error())
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			loader.BotLogger.Log("Ошибка при закрытии тела ответа: " + err.Error())
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		loader.BotLogger.Log("Ошибка при чтении тела ответа RSS: " + err.Error())
		return "", err
	}

	return string(body), nil
}

func saveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item) error {
	newsText := item.Title
	if len(newsText) > 0 {
		lastChar := newsText[len(newsText)-1]
		if lastChar == '.' || lastChar == '!' || lastChar == '?' {
			newsText += " "
		} else {
			newsText += ". "
		}
	}

	newsText += item.Description

	err := redisClient.SAdd(ctx, "news", newsText).Err()
	if err != nil {
		loader.BotLogger.Log("Ошибка при сохранении новости в Redis: " + err.Error())
		return err
	}

	expiration := 48 * time.Hour
	err = redisClient.Expire(ctx, "news", expiration).Err()
	if err != nil {
		loader.BotLogger.Log("Ошибка при установке срока годности для новости в Redis: " + err.Error())
		return err
	}

	//botLogger.Log("Новость добавлена в Redis: " + newsText)

	return nil
}
