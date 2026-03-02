package routes

import (
	"borscht.app/smetana/domain"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"gorm.io/gorm"

	"borscht.app/smetana/handlers/api"
	"borscht.app/smetana/pkg/middlewares"
	"borscht.app/smetana/pkg/repositories"
	"borscht.app/smetana/pkg/services"
)

func RegisterApiRoutes(router fiber.Router, imageService domain.ImageService, db *gorm.DB) {
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
	recipeService := services.NewRecipeService(recipeRepo, imageService, publisherService, foodRepo, unitRepo, userRepo)
	feedService := services.NewFeedService(feedRepo, recipeService)

	oidcService, err := services.NewOIDCService()
	if err != nil {
		log.Warnf("OIDC service not initialized: %v", err)
	}

	userService := services.NewUserService(userRepo)
	tokenService := services.NewTokenService(userRepo)

	authHandler := api.NewAuthHandler(oidcService, userService, tokenService)
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

	householdService := services.NewHouseholdService(householdRepo)
	householdHandler := api.NewHouseholdHandler(householdService, userService)
	householdsGroup := router.Group("/households", middlewares.Protected())
	householdsGroup.Get("/:id", householdHandler.GetHousehold)
	householdsGroup.Patch("/:id", householdHandler.UpdateHousehold)
	householdsGroup.Get("/:id/members", householdHandler.GetHouseholdMembers)
	householdsGroup.Post("/:id/members", householdHandler.AddHouseholdMember)
	householdsGroup.Delete("/:id/members/:userId", householdHandler.RemoveHouseholdMember)

	collectionService := services.NewCollectionService(collectionRepo)
	collectionHandler := api.NewCollectionHandler(collectionService, userService)
	collectionsGroup := router.Group("/collections", middlewares.Protected())
	collectionsGroup.Get("/", collectionHandler.GetCollections)
	collectionsGroup.Post("/", collectionHandler.CreateCollection)
	collectionsGroup.Get("/:id", collectionHandler.GetCollection)
	collectionsGroup.Patch("/:id", collectionHandler.UpdateCollection)
	collectionsGroup.Delete("/:id", collectionHandler.DeleteCollection)
	collectionsGroup.Post("/:id/recipes/:recipeId", collectionHandler.AddRecipeToCollection)
	collectionsGroup.Delete("/:id/recipes/:recipeId", collectionHandler.RemoveRecipeFromCollection)

	mealPlanService := services.NewMealPlanService(mealPlanRepo)
	mealPlanHandler := api.NewMealPlanHandler(mealPlanService, userService)
	mealPlanGroup := router.Group("/mealplan", middlewares.Protected())
	mealPlanGroup.Get("/", mealPlanHandler.GetMealPlan)
	mealPlanGroup.Post("/", mealPlanHandler.CreateMealPlan)
	mealPlanGroup.Patch("/:id", mealPlanHandler.UpdateMealPlan)
	mealPlanGroup.Delete("/:id", mealPlanHandler.DeleteMealPlan)

	shoppingListService := services.NewShoppingListService(shoppingListRepo)
	shoppingListHandler := api.NewShoppingListHandler(shoppingListService, userService)
	shoppingListGroup := router.Group("/shoppinglist", middlewares.Protected())
	shoppingListGroup.Get("/", shoppingListHandler.GetShoppingList)
	shoppingListGroup.Post("/", shoppingListHandler.CreateShoppingListItem)
	shoppingListGroup.Patch("/:id", shoppingListHandler.UpdateShoppingListItem)
	shoppingListGroup.Delete("/:id", shoppingListHandler.DeleteShoppingListItem)

	importHandler := api.NewImportHandler(recipeService)
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
}
