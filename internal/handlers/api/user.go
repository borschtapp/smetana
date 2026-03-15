package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
)

type UserHandler struct {
	userService domain.UserService
}

func NewUserHandler(userService domain.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetUser godoc
// @Summary Return details of a specific user.
// @Tags users
// @Accept */*
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} domain.User
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	user, err := h.userService.ByID(id, tokenData.ID)
	if err != nil {
		return err
	}
	return c.JSON(user)
}

type UpdateUserForm struct {
	Name            *string `validate:"omitempty,min=2" json:"name" example:"John Doe"`
	Email           *string `validate:"omitempty,email,min=6" json:"email" format:"email" example:"john@example.com"`
	NewPassword     *string `validate:"omitempty,min=8" json:"new_password" example:"newpassword123"`
	CurrentPassword *string `validate:"required_if=Email !nil,required_if=NewPassword !nil" json:"current_password" example:"password123"`
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [patch]
func (h *UserHandler) UpdateUser(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var body UpdateUserForm
	if err := c.Bind().Body(&body); err != nil {
		return err
	}
	if err := validate.Struct(body); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	user, err := h.userService.Update(id, tokenData.ID, body.Name, body.Email, body.CurrentPassword, body.NewPassword)
	if err != nil {
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
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.userService.Delete(id, tokenData.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
