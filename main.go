package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
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
	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/database"
	"borscht.app/smetana/internal/handlers"
	"borscht.app/smetana/internal/routes"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
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
		log.Fatal("unable to create data directory", err)
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal("database connection error", err)
	}

	if !*skipMigrations {
		if err := database.Migrate(db); err != nil {
			log.Fatal("database migration error", err)
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

	serverHost := utils.Getenv("SERVER_HOST", "")
	serverPort := utils.GetenvInt("SERVER_PORT", 3000)

	var fileStorage storage.FileStorage
	uploadsBaseUrl := os.Getenv("BASE_URL")
	if os.Getenv("S3_BUCKET") != "" {
		if uploadsBaseUrl == "" {
			uploadsBaseUrl = fmt.Sprintf("%s/%s", os.Getenv("S3_HOST"), os.Getenv("S3_BUCKET"))
		}
		fileStorage = storage.NewS3Storage(fiberS3.Config{
			Bucket:   os.Getenv("S3_BUCKET"),
			Endpoint: os.Getenv("S3_HOST"),
			Region:   os.Getenv("S3_REGION"),
			Credentials: fiberS3.Credentials{
				AccessKey:       os.Getenv("S3_ACCESS_KEY"),
				SecretAccessKey: os.Getenv("S3_SECRET_KEY"),
			},
		}, uploadsBaseUrl)
	} else {
		if uploadsBaseUrl == "" {
			uploadsHost := serverHost
			if serverHost == "" {
				uploadsHost = "localhost"
			}
			uploadsBaseUrl = fmt.Sprintf("http://%s:%d/uploads", uploadsHost, serverPort)
		}
		storageRoot := utils.Getenv("STORAGE_ROOT", "./data/uploads")
		fileStorage = storage.NewLocalStorage(storageRoot, uploadsBaseUrl)
		app.Use("/uploads", static.New(storageRoot))
	}
	storage.SetDefault(fileStorage)

	imageService := services.NewImageService(fileStorage)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	feedService := routes.RegisterApiRoutes(apiGroup, imageService, db)

	fetchInterval := utils.GetenvDuration("FETCH_INTERVAL", 24*time.Hour)
	go func() {
		ticker := time.NewTicker(fetchInterval)
		defer ticker.Stop()

		if err := feedService.FetchUpdates(ctx); err != nil && ctx.Err() == nil {
			log.Warn("feed update failed", err)
		}

		for {
			select {
			case <-ticker.C:
				if err := feedService.FetchUpdates(ctx); err != nil && ctx.Err() == nil {
					log.Warn("feed update failed", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	app.Get("/_health", handlers.HealthCheck)
	app.Get("/*", swaggo.New())

	if err := app.Listen(fmt.Sprintf("%s:%d", serverHost, serverPort), fiber.ListenConfig{
		GracefulContext: ctx,
	}); err != nil {
		log.Fatal("server stopped with error", err)
	}
}
