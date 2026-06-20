package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
)

type EquipmentHandler struct {
	service domain.EquipmentService
}

func NewEquipmentHandler(service domain.EquipmentService) *EquipmentHandler {
	return &EquipmentHandler{service: service}
}

// GetEquipment godoc
// @Summary List all equipment.
// @Description List all equipment stored in the database, with optional search and filtering.
// @Tags equipment
// @Accept */*
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param preload query string false "Comma-separated extras to include: total_recipes"
// @Param scope query string false "Restrict results to recipes visible in a given screen: feeds, saved (default: all household-visible recipes)"
// @Param sort query string false "Sort by field: id, name, created, total_recipes (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Equipment]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/equipment [get]
func (h *EquipmentHandler) GetEquipment(c fiber.Ctx) error {
	opts, err := types.GetSearchOptions(c, types.SearchConfig{
		AllowedPreloads: []string{"total_recipes"},
		AllowedSorts:    []string{"total_recipes"},
	})
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	equipment, total, err := h.service.Search(tokenData.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Equipment]{
		Data: equipment,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
}
