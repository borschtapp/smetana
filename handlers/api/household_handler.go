package api

import (
	"errors"

	"borscht.app/smetana/domain"
	sErrors "borscht.app/smetana/pkg/sentinels"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type HouseholdHandler struct {
	householdService domain.HouseholdService
	userService      domain.UserService
}

func NewHouseholdHandler(householdService domain.HouseholdService, userService domain.UserService) *HouseholdHandler {
	return &HouseholdHandler{householdService: householdService, userService: userService}
}

// GetHousehold godoc
// @Summary Returns the details of a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Success 200 {object} domain.Household
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id} [get]
func (h *HouseholdHandler) GetHousehold(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	user, err := h.userService.ById(tokenData.ID)
	if err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have access to this household")
	}

	household, err := h.householdService.ById(id)
	if err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id} [patch]
func (h *HouseholdHandler) UpdateHousehold(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	var form UpdateHouseholdForm
	if err := c.Bind().Body(&form); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	if err := validate.Struct(form); err != nil {
		return sErrors.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	user, err := h.userService.ById(tokenData.ID)
	if err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to update this household")
	}

	household, err := h.householdService.ById(id)
	if err != nil {
		return err
	}

	household.Name = form.Name
	if err := h.householdService.Update(household); err != nil {
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
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members [get]
func (h *HouseholdHandler) GetHouseholdMembers(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	user, err := h.userService.ById(tokenData.ID)
	if err != nil {
		return err
	}

	if user.HouseholdID != id {
		return sErrors.Forbidden("you do not have access to this household")
	}

	p := types.GetPagination(c)
	members, total, err := h.householdService.Members(id, p.Offset(), p.Limit)
	if err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members [post]
func (h *HouseholdHandler) AddHouseholdMember(c fiber.Ctx) error {
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

	currentUser, err := h.userService.ById(tokenData.ID)
	if err != nil {
		return err
	}

	if currentUser.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to manage this household")
	}

	targetUser, err := h.userService.ByEmail(form.Email)
	if err != nil {
		if errors.Is(err, domain.ErrRecordNotFound) {
			return sErrors.NotFound("user with this email not found")
		}
		return err
	}

	targetUser.HouseholdID = id
	if err := h.userService.Update(targetUser); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members/{userId} [delete]
func (h *HouseholdHandler) RemoveHouseholdMember(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid household id")
	}

	targetUserID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return sErrors.BadRequest("invalid user id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	currentUser, err := h.userService.ById(tokenData.ID)
	if err != nil {
		return err
	}

	if currentUser.HouseholdID != id {
		return sErrors.Forbidden("you do not have permission to manage this household")
	}

	targetUser, err := h.userService.ById(targetUserID)
	if err != nil {
		return err
	}

	if targetUser.HouseholdID != id {
		return sErrors.Forbidden("user is not in this household")
	}

	newHousehold := &domain.Household{Name: targetUser.Name + "'s Household"}
	if err = h.householdService.Create(newHousehold); err != nil {
		return err
	}

	targetUser.HouseholdID = newHousehold.ID
	if err := h.userService.Update(targetUser); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
