package configs

import (
	"fmt"
	"runtime/debug"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

func RecoverConfig() recover.Config {
	return recover.Config{
		EnableStackTrace: false,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			stackTrace := debug.Stack()
			log.Errorw("panic caught", "panic", fmt.Sprintf("%+v", e), "stack", string(stackTrace))
		},
	}
}
