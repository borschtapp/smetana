package api

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	sErrors "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
)

// @Summary Get household by ID.
// @Description Returns the details of a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Success 200 {object} domain.Household
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/households/{id} [get]
func GetHousehold(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have access to this household")
	}

	var household domain.Household
	if err := database.DB.First(&household, id).Error; err != nil {
		return err
	}

	return c.JSON(household)
}

type UpdateHouseholdForm struct {
	Name string `validate:"required,min=2" json:"name"`
}

// @Summary Update household by ID.
// @Description Rename a specific household.
// @Tags households
// @Accept json
// @Produce json
// @Param id path string true "Household ID"
// @Param household body UpdateHouseholdForm true "Household data"
// @Success 200 {object} domain.Household
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/households/{id} [patch]
func UpdateHousehold(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	var form UpdateHouseholdForm
	if err := c.Bind().Body(&form); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	validate := validator.New()
	if err := validate.Struct(form); err != nil {
		return sErrors.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to update this household")
	}

	var household domain.Household
	if err := database.DB.First(&household, id).Error; err != nil {
		return err
	}

	household.Name = form.Name
	if err := database.DB.Save(&household).Error; err != nil {
		return err
	}

	return c.JSON(household)
}

// @Summary List household members by household ID.
// @Description Returns a list of users in a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.User]
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/households/{id}/members [get]
func GetHouseholdMembers(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have access to this household")
	}

	p := types.GetPagination(c)
	query := database.DB.Where("household_id = ?", id)

	var total int64
	query.Model(&domain.User{}).Count(&total)

	var members []domain.User
	if err := query.Offset(p.Offset()).Limit(p.Limit).Find(&members).Error; err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.User]{
		Data: members,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

type AddMemberForm struct {
	Email string `validate:"required,email" json:"email" format:"email" example:"newmember@example.com"`
}

// @Summary Add a member to the household by household ID.
// @Description Assigns a user to a specific household by email.
// @Tags households
// @Accept json
// @Produce json
// @Param id path string true "Household ID"
// @Param member body AddMemberForm true "Member data"
// @Success 201 {object} domain.User
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/households/{id}/members [post]
func AddHouseholdMember(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	var form AddMemberForm
	if err := c.Bind().Body(&form); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var currentUser domain.User
	if err := database.DB.First(&currentUser, tokenData.ID).Error; err != nil {
		return err
	}

	if currentUser.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to manage this household")
	}

	var targetUser domain.User
	if err := database.DB.Where("email = ?", form.Email).First(&targetUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("user with this email not found")
		}
		return err
	}

	targetUser.HouseholdID = id
	if err := database.DB.Save(&targetUser).Error; err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(targetUser)
}

// @Summary Remove a member from the household by IDs.
// @Description Disassociates a user from a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Param userId path string true "User ID"
// @Success 204
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/households/{id}/members/{userId} [delete]
func RemoveHouseholdMember(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	targetUserId, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return sErrors.BadRequest("invalid user id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var currentUser domain.User
	if err := database.DB.First(&currentUser, tokenData.ID).Error; err != nil {
		return err
	}

	if currentUser.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to manage this household")
	}

	var targetUser domain.User
	if err := database.DB.First(&targetUser, targetUserId).Error; err != nil {
		return err
	}

	if targetUser.HouseholdID != id {
		return sErrors.Forbidden("user is not in this household")
	}

	// Create a new personal household for the removed user
	newHousehold := &domain.Household{
		Name: targetUser.Name + "'s Household",
	}
	if err := database.DB.Create(newHousehold).Error; err != nil {
		return err
	}

	targetUser.HouseholdID = newHousehold.ID
	if err := database.DB.Save(&targetUser).Error; err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
