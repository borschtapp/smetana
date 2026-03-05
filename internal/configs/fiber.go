package configs

import (
	"time"

	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

// FiberConfig func for configuration Fiber app.
// See: https://docs.gofiber.io/api/fiber#config
func FiberConfig() fiber.Config {
	readTimeoutSecondsCount := utils.GetenvInt("SERVER_READ_TIMEOUT", 60)

	// Return Fiber configuration.
	return fiber.Config{
		ReadTimeout: time.Second * time.Duration(readTimeoutSecondsCount),

		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			var se *domain.Error

			if e, ok := err.(*domain.Error); ok {
				se = e
			} else if e, ok := err.(*fiber.Error); ok {
				se = &domain.Error{Status: e.Code, Message: e.Message}
			} else {
				se = &domain.Error{Status: fiber.StatusInternalServerError, Message: err.Error()}
			}

			return ctx.Status(se.Status).JSON(se)
		},
	}
}
