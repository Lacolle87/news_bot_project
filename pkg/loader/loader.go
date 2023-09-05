package loader

import (
	"fmt"
	"github.com/joho/godotenv"
	"news_bot_project/pkg/logger"
	"news_bot_project/pkg/redis"
	"os"
)

const configFilePath = "config/logger_config.json"

var BotLogger *logger.Logger

// LoadLoggerFromConfig загружает логгер из конфигурационного файла.
func LoadLoggerFromConfig() error {
	botLogger, err := logger.InitializeLoggerFromConfig(configFilePath)
	if err != nil {
		return fmt.Errorf("ошибка при инициализации логгера: %v", err)
	}
	BotLogger = botLogger
	return nil
}

func Loader() (string, error) {
	err := godotenv.Load()
	if err != nil {
		BotLogger.Log("Ошибка при загрузке файла .env: " + err.Error())
		return "", err
	}

	redispkg.RedisHost = os.Getenv("REDIS_HOST")
	redispkg.RedisPassword = os.Getenv("REDIS_PASSWORD")
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	return botToken, err
}
