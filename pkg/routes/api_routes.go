package routes

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	"borscht.app/smetana/handlers/api"
	"borscht.app/smetana/pkg/middlewares"
	"borscht.app/smetana/pkg/services"
)

func RegisterRoutes(router fiber.Router, imageService *services.ImageService) {
	publisherService := services.NewPublisherService(imageService)
	foodService := services.NewFoodService()
	unitService := services.NewUnitService()
	recipeService := services.NewRecipeService(imageService, publisherService, foodService, unitService)
	feedService := services.NewFeedService(recipeService)

	oidcService, err := services.NewOIDCService()
	if err != nil {
		log.Warnf("OIDC service not initialized: %v", err)
	}

	authHandler := api.NewAuthHandler(oidcService)
	authGroup := router.Group("/auth", limiter.New()) // enforce always-on limiter
	authGroup.Post("/login", authHandler.Login)
	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/refresh", authHandler.Refresh)

	oidcGroup := authGroup.Group("/oidc")
	oidcGroup.Get("/login", authHandler.OIDCLogin)
	oidcGroup.Get("/callback", authHandler.OIDCCallback)

	uploadHandler := api.NewUploadHandler(imageService)
	router.Post("/api/uploads", uploadHandler.Upload, middlewares.Protected())

	usersGroup := router.Group("/users", middlewares.Protected())
	usersGroup.Get("/:id", api.GetUser)
	usersGroup.Patch("/:id", api.UpdateUser)
	usersGroup.Delete("/:id", api.DeleteUser)

	householdsGroup := router.Group("/households", middlewares.Protected())
	householdsGroup.Get("/:id", api.GetHousehold)
	householdsGroup.Patch("/:id", api.UpdateHousehold)
	householdsGroup.Get("/:id/members", api.GetHouseholdMembers)
	householdsGroup.Post("/:id/members", api.AddHouseholdMember)
	householdsGroup.Delete("/:id/members/:userId", api.RemoveHouseholdMember)

	collectionsGroup := router.Group("/collections", middlewares.Protected())
	collectionsGroup.Get("/", api.GetCollections)
	collectionsGroup.Post("/", api.CreateCollection)
	collectionsGroup.Get("/:id", api.GetCollection)
	collectionsGroup.Patch("/:id", api.UpdateCollection)
	collectionsGroup.Delete("/:id", api.DeleteCollection)

	mealPlanGroup := router.Group("/mealplan", middlewares.Protected())
	mealPlanGroup.Get("/", api.GetMealPlan)
	mealPlanGroup.Post("/", api.CreateMealPlan)
	mealPlanGroup.Patch("/:id", api.UpdateMealPlan)
	mealPlanGroup.Delete("/:id", api.DeleteMealPlan)

	shoppingListGroup := router.Group("/shoppinglist", middlewares.Protected())
	shoppingListGroup.Get("/", api.GetShoppingList)
	shoppingListGroup.Post("/", api.CreateShoppingListItem)
	shoppingListGroup.Patch("/:id", api.UpdateShoppingListItem)
	shoppingListGroup.Delete("/:id", api.DeleteShoppingListItem)

	scrapeHandler := api.NewScrapeHandler(recipeService)
	recipeHandler := api.NewRecipeHandler(recipeService)
	recipesGroup := router.Group("/recipes", middlewares.Protected())
	recipesGroup.Get("/", recipeHandler.GetRecipes)
	recipesGroup.Get("/:id", recipeHandler.GetRecipe)
	recipesGroup.Post("/", recipeHandler.CreateRecipe)
	recipesGroup.Patch("/:id", recipeHandler.UpdateRecipe)
	recipesGroup.Delete("/:id", recipeHandler.DeleteRecipe)
	recipesGroup.Post("/:id/favorite", recipeHandler.SaveRecipe)
	recipesGroup.Delete("/:id/favorite", recipeHandler.UnsaveRecipe)
	recipesGroup.Post("/import", scrapeHandler.Scrape)

	recipesGroup.Post("/:id/ingredients", recipeHandler.CreateIngredient)
	recipesGroup.Patch("/:id/ingredients/:ingredientId", recipeHandler.UpdateIngredient)
	recipesGroup.Delete("/:id/ingredients/:ingredientId", recipeHandler.DeleteIngredient)

	recipesGroup.Post("/:id/instructions", recipeHandler.CreateInstruction)
	recipesGroup.Patch("/:id/instructions/:instructionId", recipeHandler.UpdateInstruction)
	recipesGroup.Delete("/:id/instructions/:instructionId", recipeHandler.DeleteInstruction)

	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", api.GetPublishers)

	router.Get("/taxonomies", api.GetTaxonomies, middlewares.Protected())

	feedHandler := api.NewFeedHandler(feedService)
	feedsGroup := router.Group("/feeds", middlewares.Protected())
	feedsGroup.Post("/", feedHandler.Subscribe)
	feedsGroup.Delete("/:id", feedHandler.Unsubscribe)
	feedsGroup.Get("/", feedHandler.ListSubscriptions)
	feedsGroup.Get("/stream", feedHandler.GetStream)
}
