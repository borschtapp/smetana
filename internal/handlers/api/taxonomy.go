package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type TaxonomyHandler struct {
	taxonomyService domain.TaxonomyService
}

func NewTaxonomyHandler(taxonomyService domain.TaxonomyService) *TaxonomyHandler {
	return &TaxonomyHandler{taxonomyService: taxonomyService}
}

// GetTaxonomies godoc
// @Summary List all taxonomies.
// @Description Returns a list of all taxonomies. Supports filtering by type.
// @Tags taxonomies
// @Accept */*
// @Produce json
// @Param type query string false "Filter by taxonomy type"
// @Param page query int false "Page number"
// @Param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Taxonomy]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/taxonomies [get]
func (h *TaxonomyHandler) GetTaxonomies(c fiber.Ctx) error {
	taxonomyType := c.Query("type")

	p := types.GetPagination(c)
	taxonomies, total, err := h.taxonomyService.List(taxonomyType, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Taxonomy]{
		Data: taxonomies,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}
