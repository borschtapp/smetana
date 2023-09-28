package api

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/handlers/api/oauth"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/errors"
)

type OAuthLoginForm struct {
	Provider string `validate:"required"`
	Token    string `validate:"required"`
}

func OAuthLogin(c *fiber.Ctx) error {
	var requestBody OAuthLoginForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var retrievedUser domain.User
	if requestBody.Provider == "google" {
		if profile, err := oauth.GoogleGetProfile(requestBody.Token); err == nil {
			retrievedUser = domain.User{
				Email:         profile.Email,
				Name:          profile.Name,
				EmailVerified: true,
				Image:         profile.Picture,
			}
		} else {
			return errors.BadRequest("unable to get profile from google")
		}
	} else {
		return errors.BadRequest("unknown provider")
	}

	var user domain.User
	if err := database.DB.Where("email = ?", retrievedUser.Email).Find(&user).Error; err != nil {
		return err
	}

	if user.ID == 0 {
		if err := database.DB.Create(&retrievedUser).Error; err != nil {
			return err
		}
		user = retrievedUser
	}

	if tokens, err := generateTokens(user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}
