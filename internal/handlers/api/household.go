package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
)

type HouseholdHandler struct {
	householdService domain.HouseholdService
	authService      domain.AuthService
}

func NewHouseholdHandler(householdService domain.HouseholdService, authService domain.AuthService) *HouseholdHandler {
	return &HouseholdHandler{householdService: householdService, authService: authService}
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
	if err := bindBody(c, &form); err != nil {
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
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
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
			Pagination: p,
			Total:      int(total),
		},
	})
}

type CreateInviteForm struct {
	Email string `validate:"omitempty,email" json:"email"`
}

// CreateHouseholdInvite godoc
// @Summary Create an invite code for the household.
// @Description Generates a single-use 8-character invite code valid for 7 days. If email is provided, a join link is sent to that address.
// @Tags households
// @Accept json
// @Produce json
// @Param id path string true "Household ID"
// @Param body body CreateInviteForm false "Optional email to send the invite to"
// @Success 201 {object} domain.UserToken
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/invites [post]
func (h *HouseholdHandler) CreateHouseholdInvite(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var form CreateInviteForm
	if err := bindBody(c, &form); err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	invite, err := h.householdService.CreateInvite(id, tokenData.ID, tokenData.HouseholdID, form.Email)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(invite)
}

// ListHouseholdInvites godoc
// @Summary List active invite codes for the household.
// @Tags households
// @Produce json
// @Param id path string true "Household ID"
// @Success 200 {array} domain.UserToken
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/{id}/invites [get]
func (h *HouseholdHandler) ListHouseholdInvites(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	invites, err := h.householdService.ListInvites(id, tokenData.ID, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	return c.JSON(invites)
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

	if _, err := h.householdService.RemoveMember(id, tokenData.ID, tokenData.HouseholdID, targetUserID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// JoinHousehold godoc
// @Summary Join a household using an invite code.
// @Description Moves the authenticated user into the household identified by the invite code. Returns updated tokens reflecting the new household_id.
// @Tags households
// @Produce json
// @Param code path string true "Invite code"
// @Success 200 {object} AuthResponse
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/invites/{code}/join [post]
func (h *HouseholdHandler) JoinHousehold(c fiber.Ctx) error {
	code := c.Params("code")

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	user, err := h.householdService.JoinByInvite(tokenData.ID, code)
	if err != nil {
		return err
	}

	accessToken, err := h.authService.IssueAccessToken(*user)
	if err != nil {
		return err
	}

	return c.JSON(AuthResponse{User: *user, AuthTokens: domain.AuthTokens{Access: accessToken}})
}

// LeaveHousehold godoc
// @Summary Leave the current household.
// @Description Removes the authenticated user from their household and assigns them a new personal household. Returns updated tokens reflecting the new household_id.
// @Tags households
// @Produce json
// @Success 200 {object} AuthResponse
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/households/leave [post]
func (h *HouseholdHandler) LeaveHousehold(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	user, err := h.householdService.RemoveMember(tokenData.HouseholdID, tokenData.ID, tokenData.HouseholdID, tokenData.ID)
	if err != nil {
		return err
	}

	accessToken, err := h.authService.IssueAccessToken(*user)
	if err != nil {
		return err
	}

	return c.JSON(AuthResponse{User: *user, AuthTokens: domain.AuthTokens{Access: accessToken}})
}

// GetInviteInfo godoc
// @Summary Get public info about an invite code.
// @Description Returns the household name and inviter name for a given invite code without requiring authentication.
// @Tags households
// @Produce json
// @Param code path string true "Invite code"
// @Success 200 {object} domain.InviteInfo
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/households/invites/{code}/info [get]
func (h *HouseholdHandler) GetInviteInfo(c fiber.Ctx) error {
	code := c.Params("code")

	info, err := h.householdService.InviteInfo(code)
	if err != nil {
		return err
	}

	return c.JSON(info)
}

// RevokeHouseholdInvite godoc
// @Summary Revoke an invite code.
// @Tags households
// @Param id path string true "Household ID"
// @Param code path string true "Invite code"
// @Success 204
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/households/invites/{code} [delete]
func (h *HouseholdHandler) RevokeHouseholdInvite(c fiber.Ctx) error {
	code := c.Params("code")

	if err := h.householdService.RevokeInvite(code); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
