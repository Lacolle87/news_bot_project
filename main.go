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
	log.Println("Программа запущена. Новостной бот начинает работу.")

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка при загрузке файла .env")
	}

	redisHost = os.Getenv("REDIS_HOST")
	redisPassword = os.Getenv("REDIS_PASSWORD")

	redisClient := setupRedisClient()
	defer redisClient.Close()

	processNews(redisClient)

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		go func() {
			processNews(redisClient)
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
// Принимает клиент Redis redisClient в качестве аргумента.
func processNews(redisClient *redis.Client) {
	ctx := context.Background()

	rssData, err := fetchRSS()
	if err != nil {
		log.Printf("Ошибка при получении данных RSS: %v", err)
		return
	}

	rss := RSS{}
	err = xml.Unmarshal([]byte(rssData), &rss)
	if err != nil {
		log.Printf("Ошибка при разборе данных RSS: %v", err)
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	newsCount := 0

	for _, item := range rss.Channel.Items {
		wg.Add(1)
		go func(item Item) {
			defer wg.Done()

			mu.Lock()
			defer mu.Unlock()

			exists, err := redisClient.SIsMember(ctx, "news", item.Title+item.Description).Result()
			if err != nil {
				log.Printf("Ошибка Redis SIsMember: %v", err)
				return
			}

			if !exists {
				err := saveNewsToRedis(ctx, redisClient, item)
				if err != nil {
					log.Printf("Ошибка при сохранении новости в Redis: %v", err)
					return
				}

				newsCount++
			}
		}(item)
	}

	wg.Wait()

	log.Printf("Добавлено новостей: %d", newsCount)
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
