package api

import (
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/utils"
)

type LoginForm struct {
	Email    string `validate:"required,email,min=6"`
	Password string `validate:"required,min=6"`
}

func Login(c *fiber.Ctx) error {
	var requestBody LoginForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var user domain.User
	if err := database.DB.Where(&domain.User{Email: requestBody.Email}).Find(&user).Error; err != nil {
		return err
	}

	if !utils.ValidatePassword(user.Password, requestBody.Password) {
		return errors.BadRequest("wrong user email address or password")
	}

	if tokens, err := generateTokens(user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}

type AuthReturn struct {
	utils.Tokens
	user *domain.User
}

func generateTokens(user domain.User) (*AuthReturn, error) {
	tokens, err := utils.GenerateNewTokens(user.ID)
	if err != nil {
		return nil, err
	}

	// Set expires in for refresh key from .env file.
	expiresIn := time.Minute * time.Duration(utils.GetenvInt("JWT_REFRESH_EXPIRE_MINUTES", 10080))

	token := new(domain.UserToken)
	token.User = user
	token.Type = "refresh"
	token.Token = tokens.Refresh
	token.Expires = time.Now().Add(expiresIn)

	if err := database.DB.Create(&token).Error; err != nil {
		return nil, err
	}

	return &AuthReturn{user: &user, Tokens: *tokens}, nil
}

type RenewForm struct {
	RefreshToken string `validate:"required" json:"refresh_token"`
}

func Refresh(c *fiber.Ctx) error {
	var requestBody RenewForm
	if err := c.BodyParser(&requestBody); err != nil {
		return errors.BadRequest(err.Error())
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var userToken domain.UserToken
	// Retrieve the token from database and check if it's valid.
	if err := database.DB.Where(&domain.UserToken{Token: requestBody.RefreshToken, Type: "refresh"}).First(&userToken).Error; err != nil {
		return err
	}

	// Checking, if now time greater than Refresh token expiration time.
	if time.Now().Before(userToken.Expires) {
		user := userToken.User

		// remove old refresh token
		if err := database.DB.Unscoped().Where(&userToken).Delete(&userToken).Error; err != nil {
			return err
		}

		if tokens, err := generateTokens(user); err == nil {
			return c.JSON(tokens)
		} else {
			return err
		}
	} else {
		return errors.Unauthorized("unauthorized, your session was ended earlier")
	}
}
