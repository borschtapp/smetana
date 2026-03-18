package main

import (
	"context"
	"flag"
	"fmt"
	"os/signal"
	"syscall"

	_ "borscht.app/smetana/docs"
	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/database"
	"borscht.app/smetana/internal/handlers"
	"borscht.app/smetana/internal/routes"
	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
	"github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/joho/godotenv"
)

var version = "dev"

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
// @tag.name meal-plan
// @tag.description Scheduling and planning meals.
// @tag.name recipes
// @tag.description Recipe search, creation, and management.
// @tag.name shopping-lists
// @tag.description Household shopping lists and inventory.
// @tag.name user
// @tag.description Current user profile operations.
func main() {
	skipMigrations := flag.Bool("no-migrate", false, "Skip database migrations")
	flag.Parse()
	_ = godotenv.Load()

	log.Infow("starting smetana", "version", version)

	db, err := database.Connect()
	if err != nil {
		log.Fatalw("database connection error", "error", err)
	}

	if !*skipMigrations {
		if err := database.Migrate(db); err != nil {
			log.Fatalw("database migration error", "error", err)
		}
	}

	app := fiber.New(configs.FiberConfig())
	app.Use(configs.LoggerConfig())
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

	apiGroup := app.Group("/api/v1")

	serverHost := utils.Getenv("SERVER_HOST", "")
	serverPort := utils.GetenvInt("SERVER_PORT", 3000)

	storageCfg := configs.NewStorage(serverHost, serverPort)
	storage.SetDefault(storageCfg.Storage)
	if storageCfg.StorageRoot != "" {
		app.Use("/uploads", static.New(storageCfg.StorageRoot))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := routes.RegisterApiRoutes(ctx, apiGroup, storageCfg.Storage, db); err != nil {
		log.Fatalw("failed to register api routes", "error", err)
	}

	app.Get("/_health", handlers.HealthCheck)
	app.Get("/*", swaggo.New())

	if err := app.Listen(fmt.Sprintf("%s:%d", serverHost, serverPort), fiber.ListenConfig{
		GracefulContext: ctx,
	}); err != nil {
		log.Fatalw("server stopped with error", "error", err)
	}
}
