package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
)

// LoggerConfig содержит параметры для настройки логгера.
type LoggerConfig struct {
	LogDir     string `json:"logDir"`     // Директория для хранения логов
	MaxSize    int    `json:"maxSize"`    // Максимальный размер файла лога в мегабайтах
	MaxBackups int    `json:"maxBackups"` // Максимальное количество старых файлов лога, которые нужно сохранить
	MaxAge     int    `json:"maxAge"`     // Максимальное количество дней, в течение которых нужно хранить старые файлы лога
	Compress   bool   `json:"compress"`   // Сжимать старые файлы лога
	Timezone   string `json:"timezone"`   // Часовой пояс
}

// Logger представляет логгер для записи в файл с ротацией логов.
type Logger struct {
	logger *log.Logger
	file   *lumberjack.Logger
}

// NewLogger создает новый экземпляр логгера на основе переданных настроек.
// Возвращает инициализированный логгер или ошибку, если не удалось настроить логгер.
func SetupLogger(config LoggerConfig) (*Logger, error) {
	err := os.MkdirAll(config.LogDir, os.ModePerm)
	if err != nil {
		return nil, err
	}

	logPath := filepath.Join(config.LogDir, getLogFileName(config.Timezone))
	file := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}

	logger := log.New(io.MultiWriter(file, os.Stdout), "", log.LstdFlags) // Вывод в терминал и файл

	return &Logger{logger: logger, file: file}, nil
}

// Log записывает сообщение в лог.
func (l *Logger) Log(message string) {
	l.logger.Println(message)
}

// Close закрывает файловый логгер.
func (l *Logger) Close() error {
	if l.file != nil {
		err := l.file.Close()
		if err != nil {
			return fmt.Errorf("ошибка при закрытии файла лога: %v", err)
		}
	}
	return nil
}

// getLogFileName возвращает имя файла лога с текущей датой и временем в формате "log_DDMMYYYY.txt".
func getLogFileName(timezone string) string {
	currentTime := time.Now().In(getTimezone(timezone))
	return fmt.Sprintf("log_%02d%02d%04d.txt", currentTime.Day(), currentTime.Month(), currentTime.Year())
}

// getTimezone возвращает объект типа *time.Location для указанного часового пояса.
// Если часовой пояс недопустим, будет использован часовой пояс UTC.
func getTimezone(timezone string) *time.Location {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		// Использование часового пояса UTC, если указанный часовой пояс недопустим
		loc = time.UTC
	}
	return loc
}

// LoadLoggerConfig загружает конфигурацию логгера из JSON-файла.
func LoadLoggerConfig(configPath string) (LoggerConfig, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return LoggerConfig{}, err
	}
	defer file.Close()

	var config LoggerConfig
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		return LoggerConfig{}, err
	}

	return config, nil
}
