package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/handlers/api"
	"borscht.app/smetana/internal/middlewares"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/services"
)

func RegisterApiRoutes(router fiber.Router, imageService domain.ImageService, db *gorm.DB) domain.FeedService {
	// Repositories
	userRepo := repositories.NewUserRepository(db)
	publisherRepo := repositories.NewPublisherRepository(db)
	recipeRepo := repositories.NewRecipeRepository(db)
	foodRepo := repositories.NewFoodRepository(db)
	unitRepo := repositories.NewUnitRepository(db)
	feedRepo := repositories.NewFeedRepository(db)
	householdRepo := repositories.NewHouseholdRepository(db)
	collectionRepo := repositories.NewCollectionRepository(db)
	mealPlanRepo := repositories.NewMealPlanRepository(db)
	shoppingListRepo := repositories.NewShoppingListRepository(db)
	taxonomyRepo := repositories.NewTaxonomyRepository(db)

	// Services with business logic (need repos injected)
	publisherService := services.NewPublisherService(publisherRepo, imageService)
	scraperService := services.NewScraperService()
	recipeService := services.NewRecipeService(recipeRepo, userRepo, imageService, publisherService, foodRepo, unitRepo, scraperService)
	feedService := services.NewFeedService(feedRepo, publisherRepo, recipeRepo, recipeService, scraperService)

	userService := services.NewUserService(userRepo)
	authService := services.NewAuthService(userRepo)
	oidcService, err := services.NewOIDCService(userRepo)
	if err != nil {
		log.Warn("OIDC service not initialized", err)
	}

	authHandler := api.NewAuthHandler(authService, oidcService)
	authGroup := router.Group("/auth", limiter.New()) // enforce always-on limiter
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/refresh", authHandler.Refresh)

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
	householdsGroup.Post("/:id/members", householdHandler.AddHouseholdMember)
	householdsGroup.Delete("/:id/members/:userId", householdHandler.RemoveHouseholdMember)

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

	shoppingListService := services.NewShoppingListService(shoppingListRepo)
	shoppingListHandler := api.NewShoppingListHandler(shoppingListService)
	shoppingListGroup := router.Group("/shoppinglist", middlewares.Protected())
	shoppingListGroup.Get("/", shoppingListHandler.GetShoppingList)
	shoppingListGroup.Post("/", shoppingListHandler.CreateShoppingListItem)
	shoppingListGroup.Patch("/:id", shoppingListHandler.UpdateShoppingListItem)
	shoppingListGroup.Delete("/:id", shoppingListHandler.DeleteShoppingListItem)

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

	return feedService
}
