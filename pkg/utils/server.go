package utils

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
)

func Listen(app *fiber.App) error {
	serverHost := os.Getenv("SERVER_HOST")
	serverPort := GetenvInt("SERVER_PORT", 3000)

	return app.Listen(fmt.Sprintf("%s:%d", serverHost, serverPort))
}
