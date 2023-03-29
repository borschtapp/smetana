package handler

import (
	"borscht.app/smetana/auth"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/api/form"
	"borscht.app/smetana/api/presenter"
	"borscht.app/smetana/domain/user"
	"borscht.app/smetana/utils"
)

func Login(service user.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var requestBody form.LoginForm
		if err := c.BodyParser(&requestBody); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(presenter.ErrorResponse(err))
		}

		var validate = validator.New()
		if err := validate.Struct(requestBody); err != nil {
			return c.Status(http.StatusBadRequest).JSON(presenter.ValidatorResponse(err))
		}

		foundUser, err := service.FindUserByEmail(requestBody.Email)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		if foundUser == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(presenter.BadResponse("user with the given email is not found"))
		}
		if !utils.ValidatePassword(foundUser.Password, requestBody.Password) {
			return c.Status(fiber.StatusUnauthorized).JSON(presenter.BadResponse("wrong user email address or password"))
		}

		tokens, err := utils.GenerateNewTokens(foundUser.ID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(presenter.ErrorResponse(err))
		}
		return c.JSON(presenter.OkResponse(tokens))
	}
}
