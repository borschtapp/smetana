package api

import (
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/api/handler"
	"borscht.app/smetana/api/middleware"
	"borscht.app/smetana/pkg/config"
	"borscht.app/smetana/pkg/domain/book"
	"borscht.app/smetana/pkg/domain/user"
)

func RegisterRoutes(router fiber.Router) {
	userRepo := user.NewRepo(config.DB)
	userService := user.NewService(userRepo)

	router.Post("/register", handler.CreateUser(userService))

	auth := router.Group("/auth")
	auth.Post("/login", handler.Login(userService))
	auth.Post("/renew", handler.Login(userService))

	users := router.Group("/user", middleware.Protected())
	users.Get("/", handler.GetUser(userService))
	users.Patch("/", handler.UpdateUser(userService))
	users.Delete("/", handler.DeleteUser(userService))

	bookRepo := book.NewRepo(config.DB)
	bookService := book.NewService(bookRepo)

	books := router.Group("/books", middleware.Protected())
	books.Get("/", handler.GetBooks(bookService))
	books.Get("/:id", handler.GetBook(bookService))
	books.Post("/", handler.CreateBook(bookService))
	books.Put("/", handler.UpdateBook(bookService))
	books.Delete("/:id", handler.DeleteBook(bookService))
}
