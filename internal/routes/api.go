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
	authorRepo := repositories.NewAuthorRepository(db)
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
	equipmentRepo := repositories.NewEquipmentRepository(db)

	// Services with business logic (need repos injected)
	emailService, err := services.NewEmailService()
	if err != nil {
		log.Warnw("Email service not initialized", "error", err)
	}

	scraperService := services.NewScraperService()
	taxonomyService := services.NewTaxonomyService(taxonomyRepo)
	imageService := services.NewImageService(fileStorage, imageRepo)
	equipmentService := services.NewEquipmentService(equipmentRepo, imageService)
	foodService := services.NewFoodService(foodRepo, imageService)
	unitService := services.NewUnitService(unitRepo)
	publisherService := services.NewPublisherService(publisherRepo, imageService)
	authorService := services.NewAuthorService(authorRepo, imageService)
	recipeService := services.NewRecipeService(recipeRepo, userRepo, imageService, publisherService, authorService, foodService, unitService, taxonomyService, equipmentService, scraperService)
	feedService := services.NewFeedService(feedRepo, publisherService, recipeRepo, recipeService, scraperService)
	userService := services.NewUserService(userRepo)
	collectionService := services.NewCollectionService(collectionRepo, recipeRepo)
	mealPlanService := services.NewMealPlanService(mealPlanRepo)
	householdService := services.NewHouseholdService(householdRepo, userRepo, emailService)
	shoppingListService := services.NewShoppingListService(shoppingListRepo, foodService, unitService)

	oidcService, err := services.NewOIDCService(userRepo)
	if err != nil {
		log.Warnw("OIDC service not initialized", "error", err)
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

	householdHandler := api.NewHouseholdHandler(householdService, authService)
	householdsGroup := router.Group("/households", middlewares.Protected())
	householdsGroup.Get("/:id", householdHandler.GetHousehold)
	householdsGroup.Patch("/:id", householdHandler.UpdateHousehold)
	householdsGroup.Get("/:id/members", householdHandler.GetHouseholdMembers)
	householdsGroup.Delete("/:id/members/:userId", householdHandler.RemoveHouseholdMember)
	householdsGroup.Post("/:id/invites", householdHandler.CreateHouseholdInvite)
	householdsGroup.Get("/:id/invites", householdHandler.ListHouseholdInvites)
	householdsGroup.Post("/leave", householdHandler.LeaveHousehold)
	householdsGroup.Post("/invites/:code/join", householdHandler.JoinHousehold)
	router.Get("/households/invites/:code/info", householdHandler.GetInviteInfo, limiter.New(limiter.Config{Max: 3}))
	router.Delete("/households/invites/:code", householdHandler.RevokeHouseholdInvite)

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

	mealPlanHandler := api.NewMealPlanHandler(mealPlanService)
	mealPlanGroup := router.Group("/mealplan", middlewares.Protected())
	mealPlanGroup.Get("/", mealPlanHandler.GetMealPlan)
	mealPlanGroup.Post("/", mealPlanHandler.CreateMealPlan)
	mealPlanGroup.Patch("/:id", mealPlanHandler.UpdateMealPlan)
	mealPlanGroup.Delete("/:id", mealPlanHandler.DeleteMealPlan)

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

	recipesGroup.Post("/:id/equipment/:equipmentId", recipeHandler.AddEquipment)
	recipesGroup.Delete("/:id/equipment/:equipmentId", recipeHandler.RemoveEquipment)

	recipesGroup.Post("/:id/instructions", recipeHandler.CreateInstruction)
	recipesGroup.Patch("/:id/instructions/:instructionId", recipeHandler.UpdateInstruction)
	recipesGroup.Delete("/:id/instructions/:instructionId", recipeHandler.DeleteInstruction)

	publisherHandler := api.NewPublisherHandler(publisherService)
	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", publisherHandler.GetPublishers)

	taxonomyHandler := api.NewTaxonomyHandler(taxonomyService)
	router.Get("/taxonomies", taxonomyHandler.GetTaxonomies, middlewares.Protected())

	equipmentHandler := api.NewEquipmentHandler(equipmentService)
	router.Get("/equipment", equipmentHandler.Search, middlewares.Protected())

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
