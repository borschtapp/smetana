package configs

import (
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func RecoverConfig() recover.Config {
	return recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			stackTrace := debug.Stack()
			log.Errorf("Panic caught: %v\n%s\n", e, stackTrace)
		},
	}
}
