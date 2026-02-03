package configs

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	smetana "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/utils"
)

// FiberConfig func for configuration Fiber app.
// See: https://docs.gofiber.io/api/fiber#config
func FiberConfig() fiber.Config {
	readTimeoutSecondsCount := utils.GetenvInt("SERVER_READ_TIMEOUT", 60)

	// Return Fiber configuration.
	return fiber.Config{
		ReadTimeout: time.Second * time.Duration(readTimeoutSecondsCount),

		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			var se *smetana.Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				se = smetana.NotFound("The requested entity does not exist")
			} else if e, ok := err.(*smetana.Error); ok {
				se = e
			} else if e, ok := err.(*fiber.Error); ok {
				if e.Code == fiber.StatusNotFound {
					se = smetana.NotFound(err.Error())
				} else {
					se = &smetana.Error{Status: e.Code, Code: "internal-server", Message: e.Message}
				}
			} else {
				se = &smetana.Error{Status: fiber.StatusInternalServerError, Code: "internal-server", Message: err.Error()}
			}

			return ctx.Status(se.Status).JSON(se)
		},
	}
}
