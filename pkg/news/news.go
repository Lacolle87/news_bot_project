package news

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/go-redis/redis/v8"
	"io"
	"net/http"
	"news_bot_project/pkg/loader"
	"strings"
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
}

// ProcessNews processNews обрабатывает новостные данные из RSS-ленты и сохраняет их в Redis.
func ProcessNews(redisClient *redis.Client) {
	ctx := context.Background()

	// Получаем данные RSS
	rssData, err := FetchRSS()
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
			err := SaveNewsToRedis(ctx, redisClient, item)
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

// FetchRSS получает данные из RSS-ленты
func FetchRSS() (string, error) {
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

// SaveNewsToRedis сохраняет новость в хранилище Redis
func SaveNewsToRedis(ctx context.Context, redisClient *redis.Client, item Item) error {
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

	//botLogger.Log("Новость добавлена в Redis: " + newsText)

	return nil
}
