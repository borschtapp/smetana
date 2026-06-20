package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
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
// @Param preload query string false "Comma-separated extras to include: total_recipes"
// @Param scope query string false "Restrict results to recipes visible in a given screen: feeds, saved (default: all household-visible recipes)"
// @Param sort query string false "Sort by field: id, name, created, total_recipes (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Taxonomy]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/taxonomies [get]
func (h *TaxonomyHandler) GetTaxonomies(c fiber.Ctx) error {
	taxonomyType := c.Query("type")
	opts, err := types.GetSearchOptions(c, types.SearchConfig{
		AllowedPreloads: []string{"total_recipes"},
		AllowedSorts:    []string{"total_recipes"},
	})
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	taxonomies, total, err := h.taxonomyService.Search(taxonomyType, tokenData.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Taxonomy]{
		Data: taxonomies,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
}
