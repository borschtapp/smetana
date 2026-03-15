package routes

import (
	"context"
	"fmt"

	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/handlers/api"
	"borscht.app/smetana/internal/jobs"
	"borscht.app/smetana/internal/middlewares"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/scheduler"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/utils"
)

func RegisterApiRoutes(appCtx context.Context, router fiber.Router, fileStorage storage.FileStorage, db *gorm.DB) error {
	// Repositories
	imageRepo := repositories.NewImageRepository(db)
	userRepo := repositories.NewUserRepository(db)
	publisherRepo := repositories.NewPublisherRepository(db)
	recipeRepo := repositories.NewRecipeRepository(db)
	foodRepo := repositories.NewFoodRepository(db)
	unitRepo := repositories.NewUnitRepository(db)
	feedRepo := repositories.NewFeedRepository(db)
	householdRepo := repositories.NewHouseholdRepository(db)
	schedulerRepo := repositories.NewSchedulerRepository(db)
	collectionRepo := repositories.NewCollectionRepository(db)
	mealPlanRepo := repositories.NewMealPlanRepository(db)
	shoppingListRepo := repositories.NewShoppingListRepository(db)
	taxonomyRepo := repositories.NewTaxonomyRepository(db)

	// Services with business logic (need repos injected)
	imageService := services.NewImageService(fileStorage, imageRepo)
	publisherService := services.NewPublisherService(publisherRepo, imageService)
	scraperService := services.NewScraperService()
	recipeService := services.NewRecipeService(recipeRepo, userRepo, imageService, publisherService, foodRepo, unitRepo, taxonomyRepo, scraperService)
	feedService := services.NewFeedService(feedRepo, publisherRepo, recipeRepo, recipeService, scraperService)
	userService := services.NewUserService(userRepo)
	oidcService, err := services.NewOIDCService(userRepo)
	if err != nil {
		log.Warnw("OIDC service not initialized", "error", err)
	}

	emailService, err := services.NewEmailService()
	if err != nil {
		log.Warnw("Email service not initialized", "error", err)
	}

	authService := services.NewAuthService(userRepo, emailService)
	authHandler := api.NewAuthHandler(authService, oidcService)
	authGroup := router.Group("/auth", limiter.New()) // enforce always-on limiter
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/refresh", authHandler.Refresh)
	authGroup.Post("/logout", authHandler.Logout)
	authGroup.Post("/forgot-password", authHandler.ForgotPassword)
	authGroup.Post("/reset-password", authHandler.ResetPassword)

	oidcGroup := authGroup.Group("/oidc")
	oidcGroup.Get("/login", authHandler.OIDCLogin)
	oidcGroup.Get("/callback", authHandler.OIDCCallback)

	uploadHandler := api.NewUploadHandler(imageService)
	router.Post("/uploads", uploadHandler.Upload, middlewares.Protected())

	userHandler := api.NewUserHandler(userService)
	usersGroup := router.Group("/users", middlewares.Protected())
	usersGroup.Get("/:id", userHandler.GetUser)
	usersGroup.Patch("/:id", userHandler.UpdateUser)
	usersGroup.Delete("/:id", userHandler.DeleteUser)

	householdService := services.NewHouseholdService(householdRepo, userRepo)
	householdHandler := api.NewHouseholdHandler(householdService)
	householdsGroup := router.Group("/households", middlewares.Protected())
	householdsGroup.Get("/:id", householdHandler.GetHousehold)
	householdsGroup.Patch("/:id", householdHandler.UpdateHousehold)
	householdsGroup.Get("/:id/members", householdHandler.GetHouseholdMembers)
	householdsGroup.Delete("/:id/members/:userId", householdHandler.RemoveHouseholdMember)
	householdsGroup.Post("/:id/invites", householdHandler.CreateHouseholdInvite)
	householdsGroup.Get("/:id/invites", householdHandler.ListHouseholdInvites)
	householdsGroup.Delete("/:id/invites/:code", householdHandler.RevokeHouseholdInvite)
	householdsGroup.Post("/invites/join", householdHandler.JoinHousehold)

	collectionService := services.NewCollectionService(collectionRepo, recipeRepo)
	collectionHandler := api.NewCollectionHandler(collectionService)
	collectionsGroup := router.Group("/collections", middlewares.Protected())
	collectionsGroup.Get("/", collectionHandler.GetCollections)
	collectionsGroup.Post("/", collectionHandler.CreateCollection)
	collectionsGroup.Get("/:id", collectionHandler.GetCollection)
	collectionsGroup.Patch("/:id", collectionHandler.UpdateCollection)
	collectionsGroup.Delete("/:id", collectionHandler.DeleteCollection)
	collectionsGroup.Get("/:id/recipes", collectionHandler.ListRecipes)
	collectionsGroup.Post("/:id/recipes/:recipeId", collectionHandler.AddRecipeToCollection)
	collectionsGroup.Delete("/:id/recipes/:recipeId", collectionHandler.RemoveRecipeFromCollection)

	mealPlanService := services.NewMealPlanService(mealPlanRepo)
	mealPlanHandler := api.NewMealPlanHandler(mealPlanService)
	mealPlanGroup := router.Group("/mealplan", middlewares.Protected())
	mealPlanGroup.Get("/", mealPlanHandler.GetMealPlan)
	mealPlanGroup.Post("/", mealPlanHandler.CreateMealPlan)
	mealPlanGroup.Patch("/:id", mealPlanHandler.UpdateMealPlan)
	mealPlanGroup.Delete("/:id", mealPlanHandler.DeleteMealPlan)

	shoppingListService := services.NewShoppingListService(shoppingListRepo, foodRepo, unitRepo)
	shoppingListHandler := api.NewShoppingListHandler(shoppingListService)
	shoppingListGroup := router.Group("/shoppinglists", middlewares.Protected())
	shoppingListGroup.Get("/", shoppingListHandler.GetShoppingLists)
	shoppingListGroup.Post("/", shoppingListHandler.CreateShoppingList)
	shoppingListGroup.Delete("/:id", shoppingListHandler.DeleteShoppingList)
	shoppingListGroup.Get("/:id/items", shoppingListHandler.GetShoppingListItems)
	shoppingListGroup.Post("/:id/items", shoppingListHandler.AddShoppingItem)
	shoppingListGroup.Patch("/:id/items/:itemId", shoppingListHandler.UpdateShoppingItem)
	shoppingListGroup.Delete("/:id/items/:itemId", shoppingListHandler.DeleteShoppingItem)

	importHandler := api.NewImportHandler(recipeService)
	recipeHandler := api.NewRecipeHandler(recipeService)
	recipesGroup := router.Group("/recipes", middlewares.Protected())
	recipesGroup.Get("/", recipeHandler.Search)
	recipesGroup.Post("/", recipeHandler.CreateRecipe)
	recipesGroup.Post("/import", importHandler.Import)
	recipesGroup.Get("/:id", recipeHandler.GetRecipe)
	recipesGroup.Patch("/:id", recipeHandler.UpdateRecipe)
	recipesGroup.Delete("/:id", recipeHandler.DeleteRecipe)
	recipesGroup.Post("/:id/favorite", recipeHandler.SaveRecipe)
	recipesGroup.Delete("/:id/favorite", recipeHandler.UnsaveRecipe)

	recipesGroup.Post("/:id/ingredients", recipeHandler.CreateIngredient)
	recipesGroup.Patch("/:id/ingredients/:ingredientId", recipeHandler.UpdateIngredient)
	recipesGroup.Delete("/:id/ingredients/:ingredientId", recipeHandler.DeleteIngredient)

	recipesGroup.Post("/:id/instructions", recipeHandler.CreateInstruction)
	recipesGroup.Patch("/:id/instructions/:instructionId", recipeHandler.UpdateInstruction)
	recipesGroup.Delete("/:id/instructions/:instructionId", recipeHandler.DeleteInstruction)

	publisherHandler := api.NewPublisherHandler(publisherService)
	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", publisherHandler.GetPublishers)

	taxonomyHandler := api.NewTaxonomyHandler(services.NewTaxonomyService(taxonomyRepo))
	router.Get("/taxonomies", taxonomyHandler.GetTaxonomies, middlewares.Protected())

	feedHandler := api.NewFeedHandler(feedService)
	feedsGroup := router.Group("/feeds", middlewares.Protected())
	feedsGroup.Post("/", feedHandler.Subscribe)
	feedsGroup.Delete("/:id", feedHandler.Unsubscribe)
	feedsGroup.Get("/", feedHandler.ListSubscriptions)
	feedsGroup.Get("/stream", feedHandler.ListStream)

	// Scheduler for background jobs
	sched, err := scheduler.New()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	fetchInterval := utils.GetenvDuration("FETCH_INTERVAL", 24*time.Hour)
	if err := sched.Register(jobs.NewFeedFetchJob(feedService, feedRepo, schedulerRepo), fetchInterval); err != nil {
		return fmt.Errorf("failed to register feed fetch job: %w", err)
	}

	sched.Start()
	go func() {
		<-appCtx.Done()
		if err := sched.Shutdown(); err != nil {
			log.Errorw("scheduler shutdown error", "error", err)
		}
	}()

	return nil
}
