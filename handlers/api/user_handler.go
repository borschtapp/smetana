package api

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/utils"
)

// GetUser godoc
// @Summary Get user by ID.
// @Description Get details of a specific user.
// @Tags users
// @Accept */*
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} domain.User
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [get]
func GetUser(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errors.BadRequest("invalid user id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if id != tokenData.ID {
		return errors.Forbidden("you can only access your own profile")
	}

	var user domain.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return err
	}
	return c.JSON(user)
}

type UpdateUserForm struct {
	Name  *string `validate:"omitempty,min=2" json:"name" example:"John Doe"`
	Email *string `validate:"omitempty,email,min=6" json:"email" format:"email" example:"john@example.com"`
}

// UpdateUser godoc
// @Summary Update user by ID.
// @Description Update name or email of a specific user.
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body UpdateUserForm true "User update data"
// @Success 200 {object} domain.User
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [patch]
func UpdateUser(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errors.BadRequest("invalid user id")
	}

	var requestBody UpdateUserForm
	if err := c.Bind().Body(&requestBody); err != nil {
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

	if id != tokenData.ID {
		return errors.Forbidden("you can only update your own profile")
	}

	var user domain.User
	if err := database.DB.First(&user, id).Error; err != nil {
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

// DeleteUser godoc
// @Summary Delete user by ID.
// @Description Delete the account of a specific user.
// @Tags users
// @Accept */*
// @Produce json
// @Param id path string true "User ID"
// @Success 204
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [delete]
func DeleteUser(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return errors.BadRequest("invalid user id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if id != tokenData.ID {
		return errors.Forbidden("you can only delete your own profile")
	}

	if err := database.DB.Delete(&domain.User{}, id).Error; err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
