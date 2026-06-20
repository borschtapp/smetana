package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
)

type PublisherHandler struct {
	publisherService domain.PublisherService
}

func NewPublisherHandler(publisherService domain.PublisherService) *PublisherHandler {
	return &PublisherHandler{publisherService: publisherService}
}

// GetPublishers godoc
// @Summary List all publishers.
// @Description List publishers stored in the database.
// @Tags publishers
// @Accept */*
// @Produce json
// @Param q query string false "Text search"
// @Param preload query string false "Comma-separated extras to include: feeds, images, last3_recipes and total_recipes"
// @Param scope query string false "Restrict results to recipes visible in a given screen: feeds, saved (default: all household-visible recipes)"
// @Param sort query string false "Sort by field: id, name, created, total_recipes (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Publisher]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/publishers [get]
func (h *PublisherHandler) GetPublishers(c fiber.Ctx) error {
	opts, err := types.GetSearchOptions(c, types.SearchConfig{
		AllowedPreloads: []string{"feeds", "images", "last3_recipes", "total_recipes"},
		AllowedSorts:    []string{"total_recipes"},
	})
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	publishers, total, err := h.publisherService.Search(tokenData.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Publisher]{
		Data: publishers,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
}
