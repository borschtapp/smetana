package configs

import (
	"io"
	"os"
	"strings"

	"borscht.app/smetana/internal/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/logger"
)

func parseLogLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "debug":
		return log.LevelDebug
	case "info":
		return log.LevelInfo
	case "warn":
		return log.LevelWarn
	case "error":
		return log.LevelError
	default:
		return log.LevelTrace
	}
}

func LoggerConfig() fiber.Handler {
	logLevel := parseLogLevel(utils.Getenv("LOG_LEVEL", "info"))
	logTarget := strings.ToLower(utils.Getenv("LOG_TARGET", "console"))
	logFilePath := utils.Getenv("LOG_FILE_PATH", "./smetana.log")
	enableRequestsLog := utils.GetenvBool("LOG_REQUESTS", false)

	log.SetLevel(logLevel)

	if logTarget == "file" || logTarget == "both" {
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal("Failed to open log file:", err)
		}
		defer func(file *os.File) {
			_ = file.Close()
		}(file)

		if logTarget == "both" {
			iw := io.MultiWriter(os.Stdout, file)
			log.SetOutput(iw)
		} else {
			log.SetOutput(file)
		}
	}

	if enableRequestsLog {
		return logger.New()
	}

	return func(c fiber.Ctx) error {
		return c.Next()
	}
}
