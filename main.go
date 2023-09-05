package main

import (
	"log"
	"news_bot_project/pkg/bot"
	"news_bot_project/pkg/loader"
	"news_bot_project/pkg/news"
	"news_bot_project/pkg/redis"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {

	// Загрузка логгера
	err := loader.LoadLoggerFromConfig()
	if err != nil {
		log.Println(err)
	}

	botToken, err := loader.Loader()

	loader.BotLogger.Log("Программа запущена. Новостной бот начинает работу.")

	redisClient := redispkg.SetupRedisClient()
	defer func(redisClient *redis.Client) {
		err := redisClient.Close()
		if err != nil {
			loader.BotLogger.Log("Ошибка закрытия соединения Redis" + err.Error())
		}
	}(redisClient)

	news.ProcessNews(redisClient)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			news.ProcessNews(redisClient)
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
