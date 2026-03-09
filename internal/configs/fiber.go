package configs

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/internal/sentinels"
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
			var se *sentinels.Error

			if e, ok := err.(*sentinels.Error); ok {
				se = e
			} else if e, ok := err.(*fiber.Error); ok {
				se = &sentinels.Error{Status: e.Code, Message: e.Message}
			} else {
				log.Error("unexpected error:", err)
				se = &sentinels.Error{Status: fiber.StatusInternalServerError, Message: "An internal error occurred"}
			}

			return ctx.Status(se.Status).JSON(se)
		},
	}
}
