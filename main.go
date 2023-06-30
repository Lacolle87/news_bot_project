package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/joho/godotenv"
	"news_bot_project/logger"
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
	logger := logger.SetupLogger()

	fmt.Println("Программа запущена. Новостной бот начинает работу.")

	if logger == nil {
		log.Fatal("Ошибка при настройке логгера")
	}

	err := godotenv.Load()
	if err != nil {
		logger.Fatal("Ошибка при загрузке файла .env")
	}

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")

	redisClient := setupRedisClient()
	defer redisClient.Close()

	err = processNews(logger, redisClient)
	if err != nil {
		logger.Printf("Ошибка при обработке новостей: %v", err)
	}

	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		go func() {
			err := processNews(logger, redisClient)
			if err != nil {
				logger.Printf("Ошибка при обработке новостей: %v", err)
			}
		}()
	}
}

func setupRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     redisHost,
		Password: redisPassword,
		DB:       0,
	})
}

// processNews обрабатывает новости: получает данные RSS, сохраняет их в Redis и отправляет подписчикам.
// Принимает логгер logger и клиент Redis redisClient в качестве аргументов.
// Возвращает ошибку, если произошла ошибка в процессе обработки новостей.
func processNews(logger *log.Logger, redisClient *redis.Client) error {
	ctx := context.Background()

	rssData, err := fetchRSS()
	if err != nil {
		return fmt.Errorf("ошибка при получении данных RSS: %v", err)
	}

	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		return fmt.Errorf("ошибка при разборе данных RSS: %v", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	newsCount := 0

	for _, item := range rss.Channel.Items {
		wg.Add(1)
		go func(item Item) {
			defer wg.Done()

			exists, err := redisClient.SIsMember(ctx, "news", item.Description).Result()
			if err != nil {
				logger.Printf("Ошибка Redis SIsMember: %v", err)
				return
			}

			if !exists {
				mu.Lock()
				err := saveNewsToRedis(ctx, redisClient, item)
				if err != nil {
					logger.Printf("Ошибка при сохранении описания новости в Redis: %v", err)
				} else {
					err = sendNewsToSubscribers(logger, redisClient, item)
					if err != nil {
						logger.Printf("Ошибка при отправке новости подписчикам: %v", err)
					}

					newsCount++
				}
				mu.Unlock()
			}
		}(item)
	}

	wg.Wait()

	if newsCount > 0 {
		logger.Printf("Добавлено новостей: %d", newsCount)
	}

	return nil
}

func fetchRSS() (string, error) {
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

func saveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item) error {
	err := redisClient.SAdd(ctx, "news", item.Description).Err()
	if err != nil {
		return fmt.Errorf("ошибка при сохранении описания новости в Redis: %v", err)
	}

	// Устанавливаем время жизни ключа на 48 часов (в секундах)
	err = redisClient.Expire(ctx, item.Description, 48*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("ошибка при установке времени жизни ключа в Redis: %v", err)
	}

	return nil
}

func sendNewsToSubscribers(logger *log.Logger, redisClient *redis.Client, item Item) error {
	ctx := context.Background()

	subscribers, err := redisClient.SMembers(ctx, "subscribers").Result()
	if err != nil {
		return fmt.Errorf("ошибка при получении подписчиков из Redis: %v", err)
	}

	for _, subscriber := range subscribers {
		err := sendMessageToSubscriber(subscriber, item)
		if err != nil {
			logger.Printf("Ошибка при отправке новости подписчику %s: %v", subscriber, err)
		}
	}

	return nil
}

func sendMessageToSubscriber(subscriber string, item Item) error {
	// Отправляем сообщение с описанием новости подписчику
	// Используйте вашу реализацию Telegram бота для отправки сообщений
	// ...

	return nil
}
