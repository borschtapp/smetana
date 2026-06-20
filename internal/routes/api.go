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
		log.Warnw("Email service not initialized", "error", err.Error())
	}

	scraperProvider := services.NewKripProvider()
	scraperService := services.NewScraperService(scraperProvider, scraperProvider)
	taxonomyService := services.NewTaxonomyService(taxonomyRepo)
	imageService := services.NewImageService(fileStorage, imageRepo)
	equipmentService := services.NewEquipmentService(equipmentRepo, imageService)
	foodService := services.NewFoodService(foodRepo, imageService)
	unitService := services.NewUnitService(unitRepo)
	publisherService := services.NewPublisherService(publisherRepo, imageService)
	authorService := services.NewAuthorService(authorRepo, imageService)
	recipeService := services.NewRecipeService(recipeRepo, userRepo, imageService, foodService, unitService)
	recipeIngestService := services.NewRecipeIngestService(recipeService, imageService, foodService, unitService, publisherService, authorService, taxonomyService, equipmentService)
	feedService := services.NewFeedService(feedRepo, publisherService, recipeService, recipeIngestService, scraperService)
	importService := services.NewImportService(recipeService, recipeIngestService, feedService, scraperService)
	userService := services.NewUserService(userRepo, householdRepo)
	collectionService := services.NewCollectionService(collectionRepo, recipeService)
	mealPlanService := services.NewMealPlanService(mealPlanRepo)
	householdService := services.NewHouseholdService(householdRepo, userRepo, emailService)
	shoppingListService := services.NewShoppingListService(shoppingListRepo, scraperProvider, foodService, unitService)

	oidcService, err := services.NewOIDCService(userRepo)
	if err != nil {
		log.Warnw("OIDC service not initialized", "error", err.Error())
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
	uploadsGroup := router.Group("/uploads", middlewares.Protected())
	uploadsGroup.Post("/", uploadHandler.Upload)

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
	// Intentionally unprotected — anyone may look up or revoke an invite code.
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

	foodHandler := api.NewFoodHandler(foodService)
	foodGroup := router.Group("/food", middlewares.Protected())
	foodGroup.Get("/", foodHandler.GetFoods)
	foodGroup.Patch("/:id", foodHandler.UpdateFood)
	foodGroup.Post("/:id/merge", foodHandler.MergeFood)
	foodGroup.Get("/:id/price", foodHandler.GetPrice)
	foodGroup.Post("/:id/price", foodHandler.RecordPrice)
	foodGroup.Delete("/:id/price/:priceId", foodHandler.DeletePrice)

	importHandler := api.NewImportHandler(importService)
	importGroup := router.Group("/import", middlewares.Protected())
	importGroup.Post("/", importHandler.DetectAndImport)

	recipeHandler := api.NewRecipeHandler(recipeService)
	recipesGroup := router.Group("/recipes", middlewares.Protected())
	recipesGroup.Get("/", recipeHandler.GetRecipes)
	recipesGroup.Post("/", recipeHandler.CreateRecipe)
	recipesGroup.Post("/import", importHandler.Import)
	recipesGroup.Get("/:id", recipeHandler.GetRecipe)
	recipesGroup.Patch("/:id", recipeHandler.UpdateRecipe)
	recipesGroup.Delete("/:id", recipeHandler.DeleteRecipe)
	recipesGroup.Post("/:id/favorite", recipeHandler.SaveRecipe)
	recipesGroup.Delete("/:id/favorite", recipeHandler.UnsaveRecipe)
	recipesGroup.Get("/:id/price", recipeHandler.GetRecipePrice)

	recipesGroup.Post("/:id/ingredients", recipeHandler.CreateIngredient)
	recipesGroup.Patch("/:id/ingredients/:ingredientId", recipeHandler.UpdateIngredient)
	recipesGroup.Delete("/:id/ingredients/:ingredientId", recipeHandler.DeleteIngredient)

	recipesGroup.Post("/:id/equipment/:equipmentId", recipeHandler.AddEquipment)
	recipesGroup.Delete("/:id/equipment/:equipmentId", recipeHandler.RemoveEquipment)

	recipesGroup.Post("/:id/instructions", recipeHandler.CreateInstruction)
	recipesGroup.Patch("/:id/instructions/:instructionId", recipeHandler.UpdateInstruction)
	recipesGroup.Delete("/:id/instructions/:instructionId", recipeHandler.DeleteInstruction)

	authorHandler := api.NewAuthorHandler(authorService)
	authorsGroup := router.Group("/authors", middlewares.Protected())
	authorsGroup.Get("/", authorHandler.GetAuthors)

	publisherHandler := api.NewPublisherHandler(publisherService)
	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", publisherHandler.GetPublishers)

	taxonomyHandler := api.NewTaxonomyHandler(taxonomyService)
	taxonomiesGroup := router.Group("/taxonomies", middlewares.Protected())
	taxonomiesGroup.Get("/", taxonomyHandler.GetTaxonomies)

	unitHandler := api.NewUnitHandler(unitService)
	unitGroup := router.Group("/units", middlewares.Protected())
	unitGroup.Get("/", unitHandler.GetUnits)
	unitGroup.Patch("/:id", unitHandler.UpdateUnit)
	unitGroup.Post("/:id/merge", unitHandler.MergeUnit)

	equipmentHandler := api.NewEquipmentHandler(equipmentService)
	equipmentGroup := router.Group("/equipment", middlewares.Protected())
	equipmentGroup.Get("/", equipmentHandler.GetEquipment)

	feedHandler := api.NewFeedHandler(feedService)
	feedsGroup := router.Group("/feeds", middlewares.Protected())
	feedsGroup.Post("/", feedHandler.Subscribe)
	feedsGroup.Delete("/:id", feedHandler.Unsubscribe)
	feedsGroup.Post("/:id/sync", feedHandler.Sync)
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
			log.Errorw("scheduler shutdown error", "error", err.Error())
		}
	}()

	return nil
}
