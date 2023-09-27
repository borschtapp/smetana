package api

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/utils"
)

func GetUser(c *fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}
	return c.JSON(user)
}

type RegisterForm struct {
	Name     string `validate:"min=2"`
	Email    string `validate:"required,email,min=6"`
	Password string `validate:"required,min=6"`
}

func CreateUser(c *fiber.Ctx) error {
	var requestBody RegisterForm
	if err := c.BodyParser(&requestBody); err != nil {
		return err
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	var exists bool
	if err := database.DB.Model(&domain.User{}).Select("COUNT(*) > 0").Where("email = ?", requestBody.Email).Find(&exists).Error; err != nil || exists {
		return errors.BadRequestField("Email", "already exists")
	}

	user := new(domain.User)
	user.Name = requestBody.Name
	user.Email = requestBody.Email
	if hash, err := utils.HashPassword(requestBody.Password); err != nil {
		return err
	} else {
		user.Password = hash
	}

	if err := database.DB.Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return errors.BadRequestField("Email", "already exists")
		}
		return err
	}

	if tokens, err := generateTokens(*user); err == nil {
		return c.JSON(tokens)
	} else {
		return err
	}
}

type UpdateUserForm struct {
	Name  *string `validate:"omitempty,min=2"`
	Email *string `validate:"omitempty,email,min=6"`
}

func UpdateUser(c *fiber.Ctx) error {
	var requestBody UpdateUserForm
	if err := c.BodyParser(&requestBody); err != nil {
		return err
	}

	var validate = validator.New()
	if err := validate.Struct(requestBody); err != nil {
		return errors.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	if requestBody.Name != nil {
		user.Name = *requestBody.Name
	}
	if requestBody.Email != nil {
		user.Email = *requestBody.Email
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return err
	}
	return c.JSON(user)
}

func DeleteUser(c *fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&domain.User{}, tokenData.ID).Error; err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
