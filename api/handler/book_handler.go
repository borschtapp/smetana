package handler

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/api/presenter"
	"borscht.app/smetana/domain/book"
	"borscht.app/smetana/model"
)

// GetBooks is handler/controller which lists all Books from the BookShop
func GetBooks(service book.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		fetched, err := service.FetchBooks()
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(fetched))
	}
}

func GetBook(service book.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		fetched, err := service.FindBook(uint(id))
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(fetched))
	}
}

// CreateBook is handler/controller which creates Books in the BookShop
func CreateBook(service book.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var requestBody model.Book

		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		if requestBody.Author == "" || requestBody.Title == "" {
			return c.Status(http.StatusBadRequest).
				JSON(presenter.BadResponse("Please specify title and author"))
		}

		result, err := service.CreateBook(&requestBody)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(result))
	}
}

// UpdateBook is handler/controller which updates data of Books in the BookShop
func UpdateBook(service book.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var requestBody model.Book
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		result, err := service.UpdateBook(&requestBody)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(result))
	}
}

// DeleteBook is handler/controller which removes Books from the BookShop
func DeleteBook(service book.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, err := c.ParamsInt("id")
		if err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		if err := service.DeleteBook(uint(id)); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(nil))
	}
}
