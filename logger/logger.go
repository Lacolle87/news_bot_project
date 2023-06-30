package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/lumberjack"
)

// setupLogger настраивает логгер для записи в файл с ротацией логов.
// Возвращает инициализированный логгер или nil в случае ошибки.
func SetupLogger() *log.Logger {
	logDir := "logs"
	err := os.MkdirAll(logDir, os.ModePerm)
	if err != nil {
		return nil
	}

	logPath := filepath.Join(logDir, getLogFileName())
	file := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1,     // Максимальный размер файла лога в мегабайтах
		MaxBackups: 5,     // Максимальное количество старых файлов лога, которые нужно сохранить
		MaxAge:     30,    // Максимальное количество дней, в течение которых нужно хранить старые файлы лога
		Compress:   false, // Сжимать старые файлы лога
	}

	logger := log.New(io.MultiWriter(file, os.Stdout), "", log.LstdFlags) // Вывод в терминал и файл

	return logger
}

// getLogFileName возвращает имя файла лога с текущей датой и временем в формате "log_DDMMYYYY.txt".
func getLogFileName() string {
	currentTime := time.Now().In(time.FixedZone("MSK", 3*60*60)) // Установка часового пояса на Москву (UTC+3)
	return fmt.Sprintf("log_%02d%02d%04d.txt", currentTime.Day(), currentTime.Month(), currentTime.Year())
}
