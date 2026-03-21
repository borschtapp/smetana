package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type UnitHandler struct {
	service domain.UnitService
}

func NewUnitHandler(service domain.UnitService) *UnitHandler {
	return &UnitHandler{service: service}
}

// Search godoc
// @Summary Search units.
// @Description Search for units by name or slug, optionally filtered by measurement system.
// @Tags units
// @Accept */*
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param imperial query bool false "Filter by system: true=imperial, false=metric"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 20)"
// @Success 200 {object} types.ListResponse[domain.Unit]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/units [get]
func (h *UnitHandler) Search(c fiber.Ctx) error {
	query := c.Query("q")
	p := types.GetPagination(c)

	var imperial *bool
	if raw := c.Query("imperial"); raw != "" {
		imperial = new(raw == "true")
	}

	units, total, err := h.service.Search(query, imperial, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Unit]{
		Data: units,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}
