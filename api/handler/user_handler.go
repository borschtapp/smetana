package handler

import (
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/api/form"
	"borscht.app/smetana/api/presenter"
	"borscht.app/smetana/domain/user"
	"borscht.app/smetana/model"
	"borscht.app/smetana/utils"
)

func CreateUser(service user.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var requestBody form.RegisterForm
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		var validate = validator.New()
		if err := validate.Struct(requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ValidatorResponse(err))
		}

		userModel := &model.User{}
		userModel.Name = requestBody.Name
		userModel.Email = requestBody.Email
		userModel.Password = requestBody.Password

		result, err := service.CreateUser(userModel)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				return c.Status(http.StatusBadRequest).JSON(presenter.BadResponse("user with that email already exists"))
			}
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}

		return c.JSON(presenter.OkResponse(result))
	}
}

func GetUser(service user.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenData, err := utils.ExtractTokenMetadata(c)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}

		fetched, err := service.FindUser(tokenData.ID)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(fetched))
	}
}

func UpdateUser(service user.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var requestBody form.UpdateUserForm
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		var validate = validator.New()
		if err := validate.Struct(requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ValidatorResponse(err))
		}

		tokenData, err := utils.ExtractTokenMetadata(c)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}

		fetched, err := service.FindUser(tokenData.ID)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}

		fetched.Name = requestBody.Name
		fetched.Email = requestBody.Email

		result, err := service.UpdateUser(fetched)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(result))
	}
}

func DeleteUser(service user.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenData, err := utils.ExtractTokenMetadata(c)
		if err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}

		if err := service.DeleteUser(tokenData.ID); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(nil))
	}
}
