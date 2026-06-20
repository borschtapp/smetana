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

// GetUnits godoc
// @Summary List all the units.
// @Description List all the units stored in the database, with optional search and filtering.
// @Tags units
// @Accept */*
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param imperial query bool false "Filter by system: true=imperial, false=metric"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Unit]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/units [get]
func (h *UnitHandler) GetUnits(c fiber.Ctx) error {
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

type updateUnitRequest struct {
	Name *string `json:"name" validate:"omitempty,min=1,max=255"`
}

// UpdateUnit godoc
// @Summary Update a unit
// @Description Update the name of a unit.
// @Tags units
// @Accept json
// @Produce json
// @Param id path string true "Unit UUID"
// @Param unit body updateUnitRequest true "Fields to update"
// @Success 200 {object} domain.Unit
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/units/{id} [patch]
// @Security ApiKeyAuth
func (h *UnitHandler) UpdateUnit(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req updateUnitRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	unit, err := h.service.ByID(id)
	if err != nil {
		return err
	}

	if req.Name != nil {
		unit.Name = *req.Name
	}

	if err := h.service.Update(unit); err != nil {
		return err
	}
	return c.JSON(unit)
}

// MergeUnit godoc
// @Summary Merge two units
// @Description Reassigns all ingredients, shopping items, prices, food defaults, and derived units from {id} to merge_into, then marks {id} as an alias.
// @Tags units
// @Accept json
// @Produce json
// @Param id path string true "Unit UUID to merge away (becomes alias)"
// @Param body body mergeRequest true "Target unit to keep"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/units/{id}/merge [post]
// @Security ApiKeyAuth
func (h *UnitHandler) MergeUnit(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req mergeRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	if err := h.service.Merge(req.MergeInto, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
