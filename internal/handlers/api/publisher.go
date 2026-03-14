package api

import (
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
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
// @Param preload query string false "Comma-separated extras to include: feeds, recipes:5, recipes.images and total_recipes"
// @Param sort query string false "Sort by field: id, name, created (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param page query int false "Page number"
// @Param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Publisher]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/publishers [get]
func (h *PublisherHandler) GetPublishers(c fiber.Ctx) error {
	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	publishers, total, err := h.publisherService.Search(opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Publisher]{
		Data: publishers,
		Meta: types.Meta{
			Total: int(total),
			Page:  opts.Page,
		},
	})
}
