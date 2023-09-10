package routes

import (
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/handlers/api"
	"borscht.app/smetana/handlers/api/oauth"
	"borscht.app/smetana/pkg/middlewares"
)

func RegisterRoutes(router fiber.Router) {
	router.Get("/_health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"message": "Hello, i'm ok!",
		})
	})

	router.Post("/login", api.Login)
	router.Post("/token/refresh", api.Refresh)

	oauthGroup := router.Group("/oauth")
	oauthGroup.Get("/google", oauth.GoogleRequest)
	oauthGroup.Get("/google/callback", oauth.AuthCallbackGoogle)

	usersGroup := router.Group("/users", middlewares.Protected())
	usersGroup.Get("/", api.GetUser)
	usersGroup.Post("/", api.CreateUser)
	usersGroup.Patch("/", api.UpdateUser)
	usersGroup.Delete("/", api.DeleteUser)

	recipesGroup := router.Group("/recipes", middlewares.Protected())
	recipesGroup.Get("/", api.GetRecipes)
	recipesGroup.Get("/scrape", api.Scrape)
	recipesGroup.Get("/:id", api.GetRecipe)
	recipesGroup.Post("/", api.CreateRecipe)
	recipesGroup.Put("/", api.UpdateRecipe)
	recipesGroup.Delete("/:id", api.DeleteRecipe)

	publishersGroup := router.Group("/publishers", middlewares.Protected())
	publishersGroup.Get("/", api.GetPublishers)
}
