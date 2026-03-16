package api

import (
	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type EquipmentHandler struct {
	db *gorm.DB
}

func NewEquipmentHandler(db *gorm.DB) *EquipmentHandler {
	return &EquipmentHandler{db: db}
}

// Search godoc
// @Summary Search equipment.
// @Description Search for equipment by name or slug.
// @Tags equipment
// @Accept */*
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param page query int false "Page number"
// @Param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page (default: 20)"
// @Success 200 {object} types.ListResponse[domain.Equipment]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/equipment [get]
func (h *EquipmentHandler) Search(c fiber.Ctx) error {
	query := c.Query("q")
	p := types.GetPagination(c)

	q := h.db

	if query != "" {
		q = q.Where("name LIKE ? OR slug LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	var total int64
	if err := q.Model(&domain.Equipment{}).Count(&total).Error; err != nil {
		return err
	}

	var equipment []domain.Equipment
	if err := q.Offset(p.Offset).Limit(p.Limit).Find(&equipment).Error; err != nil {
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
