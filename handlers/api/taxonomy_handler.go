package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/types"
)

// GetTaxonomies godoc
// @Summary List all taxonomies.
// @Description Returns a list of all taxonomies. Supports filtering by type.
// @Tags taxonomies
// @Accept */*
// @Produce json
// @Param type query string false "Filter by taxonomy type"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Taxonomy]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/taxonomies [get]
func GetTaxonomies(c fiber.Ctx) error {
	taxonomyType := c.Query("type")

	p := types.GetPagination(c)
	query := database.DB.Model(&domain.Taxonomy{})

	if taxonomyType != "" {
		query = query.Where("type = ?", taxonomyType)
	}

	var total int64
	query.Count(&total)

	var taxonomies []domain.Taxonomy
	if err := query.Offset(p.Offset()).Limit(p.Limit).Find(&taxonomies).Error; err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Taxonomy]{
		Data: taxonomies,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}
