package loader

import (
	"fmt"
	"news_bot_project/logger"
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
