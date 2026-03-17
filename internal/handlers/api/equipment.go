package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type EquipmentHandler struct {
	service domain.EquipmentService
}

func NewEquipmentHandler(service domain.EquipmentService) *EquipmentHandler {
	return &EquipmentHandler{service: service}
}

// Search godoc
// @Summary Search equipment.
// @Description Search for equipment by name or slug.
// @Tags equipment
// @Accept */*
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10) (default: 20)"
// @Success 200 {object} types.ListResponse[domain.Equipment]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/equipment [get]
func (h *EquipmentHandler) Search(c fiber.Ctx) error {
	query := c.Query("q")
	p := types.GetPagination(c)

	equipment, total, err := h.service.Search(query, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Equipment]{
		Data: equipment,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}
