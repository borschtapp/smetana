package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	fiberS3 "github.com/gofiber/storage/s3/v2"
	"github.com/joho/godotenv"

	_ "borscht.app/smetana/docs"
	"borscht.app/smetana/handlers"
	"borscht.app/smetana/pkg/configs"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/routes"
	"borscht.app/smetana/pkg/services"
	"borscht.app/smetana/pkg/storage"
	"borscht.app/smetana/pkg/utils"
)

// @title Smetana API
// @version 1.0
// @description The backend API for Borscht app.
// @license.name MIT license
// @license.url https://opensource.org/license/mit/
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Bearer token authorization using JWT. Example: "Bearer {token}"
// @tag.name auth
// @tag.description Authentication and user management.
// @tag.name feeds
// @tag.description Recipe streams and RSS feed subscriptions.
// @tag.name households
// @tag.description Shared household management and member coordination.
// @tag.name mealplan
// @tag.description Scheduling and planning meals.
// @tag.name recipes
// @tag.description Recipe search, creation, and management.
// @tag.name shoppinglist
// @tag.description Household shopping lists and inventory.
// @tag.name user
// @tag.description Current user profile operations.
func main() {
	skipMigrations := flag.Bool("no-migrate", false, "Skip database migrations")
	flag.Parse()
	_ = godotenv.Load()

	if err := os.MkdirAll("./data", 0700); err != nil {
		log.Fatalf("Unable to create data directory: %s", err)
	}

	if _, err := database.Connect(); err != nil {
		log.Fatalf("Database Connection Error %s", err)
	}

	if !*skipMigrations {
		if err := database.Migrate(); err != nil {
			log.Fatalf("Database Migration Error %s", err)
		}
	}

	app := fiber.New(configs.FiberConfig())
	app.Use(cors.New())
	app.Use(recover.New(configs.RecoverConfig()))
	app.Use(helmet.New())
	app.Use(etag.New())

	if utils.GetenvBool("ENABLE_LIMITER", false) {
		app.Use(limiter.New())
	}
	if utils.GetenvBool("ENABLE_COMPRESS", false) {
		app.Use(compress.New())
	}
	if utils.GetenvBool("ENABLE_LOGGER", true) {
		app.Use(logger.New())
	}

	apiGroup := app.Group("/api/v1")

	serverHost := utils.Getenv("SERVER_HOST", "127.0.0.1")
	serverPort := utils.GetenvInt("SERVER_PORT", 3000)

	var fileStorage storage.FileStorage
	baseUrl := os.Getenv("BASE_URL")
	if os.Getenv("S3_BUCKET") != "" {
		if baseUrl == "" {
			baseUrl = fmt.Sprintf("%s/%s", os.Getenv("S3_HOST"), os.Getenv("S3_BUCKET"))
		}
		fileStorage = storage.NewS3Storage(fiberS3.Config{
			Bucket:   os.Getenv("S3_BUCKET"),
			Endpoint: os.Getenv("S3_HOST"),
			Region:   os.Getenv("S3_REGION"),
			Credentials: fiberS3.Credentials{
				AccessKey:       os.Getenv("S3_ACCESS_KEY"),
				SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
			},
		}, baseUrl)
	} else {
		if baseUrl == "" {
			baseUrl = fmt.Sprintf("http://%s:%d/uploads", serverHost, serverPort)
		}
		storageRoot := utils.Getenv("STORAGE_ROOT", "./data/uploads")
		fileStorage = storage.NewLocalStorage(storageRoot, baseUrl)
		app.Use("/uploads", static.New(storageRoot))
	}
	storage.SetDefault(fileStorage)

	imageService := services.NewImageService(fileStorage)

	routes.RegisterRoutes(apiGroup, imageService)

	app.Get("/_health", handlers.HealthCheck)
	app.Get("/*", swaggo.New())

	log.Fatal(app.Listen(fmt.Sprintf("%s:%d", serverHost, serverPort), fiber.ListenConfig{}))
}
