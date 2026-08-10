package configs

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

// FiberConfig func for configuration Fiber app.
// See: https://docs.gofiber.io/api/fiber#config
func FiberConfig() fiber.Config {
	return fiber.Config{
		ReadTimeout:  time.Second * time.Duration(utils.GetenvInt("SERVER_READ_TIMEOUT", 15)),
		WriteTimeout: time.Second * time.Duration(utils.GetenvInt("SERVER_WRITE_TIMEOUT", 30)),
		IdleTimeout:  time.Second * time.Duration(utils.GetenvInt("SERVER_IDLE_TIMEOUT", 120)),
		BodyLimit:    8 * 1024 * 1024, // 8 MB

		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			var se *sentinels.Error
			var fe *fiber.Error

			switch {
			case errors.As(err, &se):
				// already a sentinels.Error, use as-is
			case errors.As(err, &fe):
				se = &sentinels.Error{Status: fe.Code, Message: fe.Message}
			default:
				log.Errorw("unexpected error", "error", err.Error(), "method", ctx.Method(), "path", ctx.Path())
				se = &sentinels.Error{Status: fiber.StatusInternalServerError, Message: "An internal error occurred"}
			}

			return ctx.Status(se.Status).JSON(se)
		},
	}
}
