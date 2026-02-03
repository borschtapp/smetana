package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/types"
)

// GetPublishers godoc
// @Summary List all publishers.
// @Description List publishers stored in the database.
// @Tags publishers
// @Accept */*
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Publisher]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/publishers [get]
func GetPublishers(c fiber.Ctx) error {
	p := types.GetPagination(c)

	var total int64
	database.DB.Model(&domain.Publisher{}).Count(&total)

	var publishers []domain.Publisher
	if err := database.DB.Model(&domain.Publisher{}).
		Offset(p.Offset()).
		Limit(p.Limit).
		Find(&publishers).Error; err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Publisher]{
		Data: publishers,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}
