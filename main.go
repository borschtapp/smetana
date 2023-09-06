package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"borscht.app/smetana/pkg/configs"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/routes"
	"borscht.app/smetana/pkg/utils"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Failed to load dotenv $s", err)
	}

	if err := database.Connect(); err != nil {
		log.Fatal("Database Connection Error $s", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatal("Database Migration Error $s", err)
	}

	app := fiber.New(configs.FiberConfig())
	app.Use(cors.New())
	app.Use(recover.New())
	app.Use(logger.New())

	app.Get("/", func(ctx *fiber.Ctx) error {
		return ctx.Send([]byte("Welcome to the Smetana API!"))
	})

	apiGroup := app.Group("/api")
	routes.RegisterRoutes(apiGroup)

	log.Fatal(utils.Listen(app))
}
