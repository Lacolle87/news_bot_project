package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
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
	loggerConfig := logger.LoggerConfig{
		LogDir:     "logs",
		MaxSize:    1,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   false,
	}

	logger, err := logger.SetupLogger(loggerConfig)
	if err != nil {
		log.Fatal("Ошибка при инициализации логгера:", err)
	}
	defer logger.Close()

	err = godotenv.Load()
	if err != nil {
		logger.Log(fmt.Sprintf("Ошибка при загрузке файла .env: %v", err))
		logger.Close()
		return
	}

	logger.Log(fmt.Sprintf("Программа запущена. Новостной бот начинает работу."))

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")

	redisClient := setupRedisClient(logger)
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

func setupRedisClient(logger *logger.Logger) *redis.Client {
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

func processNews(redisClient *redis.Client, logger *logger.Logger) {
	ctx := context.Background()

	rssData, err := fetchRSS(logger)
	if err != nil {
		logger.Log(fmt.Sprintf("Ошибка при получении данных RSS: %v", err))
		return
	}

	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		logger.Log(fmt.Sprintf("Ошибка при разборе данных RSS: %v", err))
		return
	}

	var mu sync.Mutex
	newsCount := 0

	for _, item := range rss.Channel.Items {
		exists, err := redisClient.SIsMember(ctx, "news", item.Title+item.Description).Result()
		if err != nil {
			logger.Log(fmt.Sprintf("Ошибка Redis SIsMember: %v", err))
			continue
		}

		if !exists {
			mu.Lock()

			err := saveNewsToRedis(ctx, redisClient, item, logger)
			if err != nil {
				logger.Log(fmt.Sprintf("Ошибка при сохранении новости в Redis: %v", err))
			} else {
				newsCount++
			}

			mu.Unlock()
		}
	}

	logger.Log(fmt.Sprintf("Добавлено новостей: %d", newsCount))
}

func fetchRSS(logger *logger.Logger) (string, error) {
	resp, err := http.Get("https://news.mail.ru/rss/")
	if err != nil {
		return "", fmt.Errorf("ошибка при получении данных RSS: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении тела ответа RSS: %v", err)
	}

	return string(body), nil
}

func saveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item, logger *logger.Logger) error {
	newsText := item.Title + ". " + item.Description

	err := redisClient.SAdd(ctx, "news", newsText).Err()
	if err != nil {
		return fmt.Errorf("ошибка при сохранении новости в Redis: %v", err)
	}

	// Устанавливаем время жизни ключа на 48 часов (в секундах)
	err = redisClient.Expire(ctx, "news", 48*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("ошибка при установке времени жизни ключа в Redis: %v", err)
	}

	return nil
}
