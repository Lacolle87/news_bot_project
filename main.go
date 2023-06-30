package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
)

// Замените эти значения на свои
var (
	redisHost     string
	redisPassword string
)

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
	PubDate     string `xml:"pubDate"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка при загрузке файла .env")
	}

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")

	redisClient := setupRedisClient()
	defer redisClient.Close()

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)
	logger.Println("Программа запущена. Новостной бот начинает работу.")

	err = processNews(redisClient, logger)
	if err != nil {
		logger.Printf("Ошибка при обработке новостей: %v", err)
	}

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		err := processNews(redisClient, logger)
		if err != nil {
			logger.Printf("Ошибка при обработке новостей: %v", err)
		}
	}
}

// setupRedisClient создает и возвращает клиент Redis.
func setupRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       0,
	})
}

// processNews обрабатывает новости: получает данные RSS, сохраняет их в Redis и отправляет их.
func processNews(redisClient *redis.Client, logger *log.Logger) error {
	ctx := context.Background()

	rssData, err := fetchRSS()
	if err != nil {
		return fmt.Errorf("ошибка при получении данных из RSS: %v", err)
	}

	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		return fmt.Errorf("ошибка при разборе данных из RSS: %v", err)
	}

	newsCount := 0

	for _, item := range rss.Channel.Items {
		exists, err := redisClient.SIsMember(ctx, "sent_news", item.Link).Result()
		if err != nil {
			logger.Printf("Ошибка Redis SIsMember: %v", err)
		}

		if !exists {
			err := saveNewsToRedis(ctx, redisClient, item)
			if err != nil {
				logger.Printf("Ошибка при сохранении новости в Redis: %v", err)
			}

			// Обработка и отправка новости
			// ...

			newsCount++
		}
	}

	logger.Printf("Добавлено новостей: %d", newsCount)

	return nil
}

// fetchRSS получает данные из RSS-ленты.
func fetchRSS() (string, error) {
	resp, err := http.Get("https://news.mail.ru/rss/")
	if err != nil {
		return "", fmt.Errorf("ошибка при получении данных из RSS: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении тела ответа RSS: %v", err)
	}

	return string(body), nil
}

// saveNewsToRedis сохраняет ссылку на новость в Redis.
func saveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item) error {
	err := redisClient.SAdd(ctx, "sent_news", item.Link).Err()
	if err != nil {
		return fmt.Errorf("ошибка при сохранении ссылки на новость в Redis: %v", err)
	}

	return nil
}
