package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/vrischmann/envconfig"

	"borscht.app/smetana/api"
	"borscht.app/smetana/pkg/config"

	_ "github.com/joho/godotenv/autoload" // load .env file automatically
)

func main() {
	if err := envconfig.Init(&config.Env); err != nil {
		log.Fatal("Failed to load configuration $s", err)
	}

	if err := config.ConnectSqlLite(); err != nil {
		log.Fatal("Database Connection Error $s", err)
	}

	if err := config.MigrateDB(); err != nil {
		log.Fatal("Database Migration Error $s", err)
	}

	app := fiber.New()
	app.Use(cors.New())
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.Send([]byte("Welcome to the Smetana API!"))
	})

	apiGroup := app.Group("/api")
	api.RegisterRoutes(apiGroup)

	log.Fatal(app.Listen(":3000"))
}
