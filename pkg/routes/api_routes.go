package routes

import (
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/handlers/api"
	"borscht.app/smetana/handlers/api/oauth"
	"borscht.app/smetana/pkg/middlewares"
)

func RegisterRoutes(router fiber.Router) {
	router.Post("/login", api.Login)

	tokenGroup := router.Group("/token")
	tokenGroup.Post("/refresh", api.Refresh)

	oauthGroup := router.Group("/oauth")
	oauthGroup.Post("/login", api.OAuthLogin)
	oauthGroup.Get("/google", oauth.GoogleRequest)
	oauthGroup.Get("/google/callback", oauth.AuthCallbackGoogle)

	usersGroup := router.Group("/users")
	usersGroup.Get("/", api.GetUser, middlewares.Protected())
	usersGroup.Post("/", api.CreateUser)
	usersGroup.Patch("/", api.UpdateUser, middlewares.Protected())
	usersGroup.Delete("/", api.DeleteUser, middlewares.Protected())

	recipesGroup := router.Group("/recipes", middlewares.Protected())
	recipesGroup.Get("/", api.GetRecipes)
	recipesGroup.Get("/explore", api.ExploreRecipes)
	recipesGroup.Get("/scrape", api.Scrape)
	recipesGroup.Get("/:id", api.GetRecipe)
	recipesGroup.Post("/", api.CreateRecipe)
	recipesGroup.Put("/", api.UpdateRecipe)
	recipesGroup.Delete("/:id", api.DeleteRecipe)
	recipesGroup.Post("/:id/save", api.SaveRecipe)
	recipesGroup.Delete("/:id/save", api.UnsaveRecipe)

	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", api.GetPublishers)
}
