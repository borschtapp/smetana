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
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Publisher]
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/publishers [get]
func (h *PublisherHandler) GetPublishers(c fiber.Ctx) error {
	p := types.GetPagination(c)
	publishers, total, err := h.publisherService.List(p.Offset(), p.Limit)
	if err != nil {
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
