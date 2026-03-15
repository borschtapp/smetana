package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
)

type HouseholdHandler struct {
	householdService domain.HouseholdService
}

func NewHouseholdHandler(householdService domain.HouseholdService) *HouseholdHandler {
	return &HouseholdHandler{householdService: householdService}
}

// GetHousehold godoc
// @Summary Returns the details of a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Success 200 {object} domain.Household
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id} [get]
func (h *HouseholdHandler) GetHousehold(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	household, err := h.householdService.ByID(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	return c.JSON(household)
}

type UpdateHouseholdForm struct {
	Name string `validate:"required,min=2" json:"name"`
}

// UpdateHousehold godoc
// @Summary Update household by ID.
// @Description Rename a specific household.
// @Tags households
// @Accept json
// @Produce json
// @Param id path string true "Household ID"
// @Param household body UpdateHouseholdForm true "Household data"
// @Success 200 {object} domain.Household
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id} [patch]
func (h *HouseholdHandler) UpdateHousehold(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var form UpdateHouseholdForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(form); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	household, err := h.householdService.ByID(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	household.Name = form.Name
	if err := h.householdService.Update(household, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.JSON(household)
}

// GetHouseholdMembers godoc
// @Summary List household members by household ID.
// @Description Returns a list of users in a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Param page query int false "Page number"
// @Param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.User]
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members [get]
func (h *HouseholdHandler) GetHouseholdMembers(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	p := types.GetPagination(c)
	members, total, err := h.householdService.Members(id, tokenData.HouseholdID, p.Offset, p.Limit)
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

// AddHouseholdMember godoc
// @Summary Add a member to the household by household ID.
// @Description Assigns a user to a specific household by email.
// @Tags households
// @Accept json
// @Produce json
// @Param id path string true "Household ID"
// @Param member body AddMemberForm true "Member data"
// @Success 202
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members [post]
func (h *HouseholdHandler) AddHouseholdMember(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var form AddMemberForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(form); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.householdService.AddMember(id, tokenData.HouseholdID, form.Email); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusAccepted)
}

// RemoveHouseholdMember godoc
// @Summary Remove a member from the household by IDs.
// @Description Disassociates a user from a specific household.
// @Tags households
// @Accept */*
// @Produce json
// @Param id path string true "Household ID"
// @Param userId path string true "User ID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/members/{userId} [delete]
func (h *HouseholdHandler) RemoveHouseholdMember(c fiber.Ctx) error {
	id, targetUserID, err := types.UuidParams(c, "id", "userId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.householdService.RemoveMember(id, tokenData.HouseholdID, targetUserID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
