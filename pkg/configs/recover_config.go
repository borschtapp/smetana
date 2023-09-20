package configs

import (
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func RecoverConfig() recover.Config {
	return recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e interface{}) {
			stackTrace := debug.Stack()
			log.Errorf("Panic caught: %v\n%s\n", e, stackTrace)
		},
	}
}
