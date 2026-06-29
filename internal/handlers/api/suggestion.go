package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type SuggestionHandler struct {
	service domain.SuggestionService
}

func NewSuggestionHandler(service domain.SuggestionService) *SuggestionHandler {
	return &SuggestionHandler{service: service}
}

// ListSuggestions godoc
// @Summary List suggestions
// @Description List catalog maintenance suggestions with optional filters. Defaults to pending status.
// @Tags suggestions
// @Produce json
// @Param type query string false "Suggestion type filter"
// @Param status query string false "Status filter (default: pending)"
// @Param subject_type query string false "Subject entity type (food|unit|taxonomy|ingredient)"
// @Param subject_id query string false "Subject entity UUID"
// @Param scope query string false "Visibility scope: global|household|all (default: all)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.SuggestionView]
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/suggestions [get]
// @Security ApiKeyAuth
func (h *SuggestionHandler) ListSuggestions(c fiber.Ctx) error {
	tokenData := tokens.MustClaims(c)
	p := types.GetPagination(c)

	statusParam := c.Query("status", string(domain.StatusPending))
	scope := c.Query("scope", "all")
	if !utils.ContainsFold(scope, "global", "household", "all") {
		return sentinels.BadRequest("invalid scope parameter, must be 'global', 'household', or 'all'")
	}

	filter := domain.SuggestionFilter{
		Type:        domain.SuggestionType(c.Query("type")),
		Status:      domain.SuggestionStatus(statusParam),
		SubjectType: c.Query("subject_type"),
		HouseholdID: &tokenData.HouseholdID,
		Scope:       scope,
	}

	if subjectIDStr := c.Query("subject_id"); subjectIDStr != "" {
		id, err := uuid.Parse(subjectIDStr)
		if err == nil {
			filter.SubjectID = &id
		}
	}

	views, total, err := h.service.List(filter, p)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.SuggestionView]{
		Data: views,
		Meta: types.Meta{Pagination: p, Total: int(total)},
	})
}

// GetSuggestion godoc
// @Summary Get a single suggestion with hydrated view
// @Tags suggestions
// @Produce json
// @Param id path string true "Suggestion UUID"
// @Success 200 {object} domain.SuggestionView
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/suggestions/{id} [get]
// @Security ApiKeyAuth
func (h *SuggestionHandler) GetSuggestion(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}
	view, err := h.service.Get(id)
	if err != nil {
		return err
	}
	return c.JSON(view)
}

// GetStats godoc
// @Summary Suggestion counts grouped by type and status
// @Tags suggestions
// @Produce json
// @Success 200 {array} domain.SuggestionStat
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/suggestions/stats [get]
// @Security ApiKeyAuth
func (h *SuggestionHandler) GetStats(c fiber.Ctx) error {
	tokenData := tokens.MustClaims(c)
	stats, err := h.service.Stats(&tokenData.HouseholdID)
	if err != nil {
		return err
	}
	return c.JSON(stats)
}

// AcceptSuggestion godoc
// @Summary Accept and apply a suggestion
// @Tags suggestions
// @Param id path string true "Suggestion UUID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/suggestions/{id}/accept [post]
// @Security ApiKeyAuth
func (h *SuggestionHandler) AcceptSuggestion(c fiber.Ctx) error {
	tokenData := tokens.MustClaims(c)
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Accept(c.Context(), id, tokenData.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RejectSuggestion godoc
// @Summary Reject a suggestion
// @Description Decline a pending suggestion. The suggestion is kept for dedup purposes.
// @Tags suggestions
// @Param id path string true "Suggestion UUID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/suggestions/{id}/reject [post]
// @Security ApiKeyAuth
func (h *SuggestionHandler) RejectSuggestion(c fiber.Ctx) error {
	tokenData := tokens.MustClaims(c)
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Reject(c.Context(), id, tokenData.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ScanNow godoc
// @Summary Trigger an immediate reconcile sweep
// @Description Runs all global detectors (or a single global type if ?type= is provided) synchronously. Household-scoped detectors are skipped.
// @Tags suggestions
// @Accept json
// @Param type query string false "Limit scan to a single detector type"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/suggestions/scan [post]
// @Security ApiKeyAuth
func (h *SuggestionHandler) ScanNow(c fiber.Ctx) error {
	var filterTypes []domain.SuggestionType
	if t := c.Query("type"); t != "" {
		filterTypes = append(filterTypes, domain.SuggestionType(t))
	}
	if err := h.service.Reconcile(c.Context(), nil, filterTypes...); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
