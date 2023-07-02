package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"news_bot_project/logger" // Путь к вашему пакету logger
)

// Замените эти значения на свои
var (
	redisHost     string
	redisPassword string
)

// Замените на свои структуры, соответствующие структуре XML-данных RSS-канала
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
	configFile := "config/logger_config.json"

	loggerConfig, err := logger.LoadLoggerConfig(configFile)
	if err != nil {
		log.Fatal("Ошибка при загрузке конфигурации логгера:", err)
	}

	logger, err := logger.SetupLogger(loggerConfig)
	if err != nil {
		log.Fatal("Ошибка при инициализации логгера:", err)
	}
	defer logger.Close()

	err = godotenv.Load()
	if err != nil {
		logger.Log("Ошибка при загрузке файла .env: " + err.Error())
		logger.Close()
		return
	}

	logger.Log("Программа запущена. Новостной бот начинает работу.")

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")

	redisClient := setupRedisClient()
	defer redisClient.Close()

	processNews(redisClient, logger)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		go func() {
			processNews(redisClient, logger)
		}()
	}
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
func processNews(redisClient *redis.Client, logger *logger.Logger) {
	ctx := context.Background()

	// Получаем данные RSS
	rssData, err := fetchRSS(logger)
	if err != nil {
		logger.Log("Ошибка при получении данных RSS: " + err.Error())
		return
	}

	// Разбираем данные RSS
	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		logger.Log("Ошибка при разборе данных RSS: " + err.Error())
		return
	}

	// Получаем начальное количество новостей из Redis
	initialCount, err := redisClient.SCard(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении количества новостей из Redis: " + err.Error())
		return
	}

	// Обрабатываем каждый элемент в RSS-ленте
	for _, item := range rss.Channel.Items {
		// Проверяем, существует ли новость уже в Redis
		exists, err := redisClient.SIsMember(ctx, "news", item.Title+item.Description).Result()
		if err != nil {
			logger.Log("Ошибка Redis SIsMember: " + err.Error())
			continue
		}

		// Если новость не существует в Redis, сохраняем её
		if !exists {
			err := saveNewsToRedis(ctx, redisClient, item, logger)
			if err != nil {
				logger.Log("Ошибка при сохранении новости в Redis: " + err.Error())
			}
		}
	}

	// Получаем обновленное количество новостей после добавления
	updatedCount, err := redisClient.SCard(ctx, "news").Result()
	if err != nil {
		logger.Log("Ошибка при получении общего количества новостей из Redis: " + err.Error())
		return
	}

	// Рассчитываем количество добавленных новостей
	addedCount := updatedCount - initialCount
	if addedCount > 0 {
		logger.Log(fmt.Sprintf("Добавлено новостей: %d", addedCount))
	}
}

func fetchRSS(logger *logger.Logger) (string, error) {
	resp, err := http.Get("https://news.mail.ru/rss/")
	if err != nil {
		logger.Log("Ошибка при получении данных RSS: " + err.Error())
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Log("Ошибка при чтении тела ответа RSS: " + err.Error())
		return "", err
	}

	return string(body), nil
}

func saveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item, logger *logger.Logger) error {
	newsText := item.Title + ". " + item.Description

	err := redisClient.SAdd(ctx, "news", newsText).Err()
	if err != nil {
		logger.Log("Ошибка при сохранении новости в Redis: " + err.Error())
		return err
	}

	return nil
}
