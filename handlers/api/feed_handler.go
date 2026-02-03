package api

import (
	"errors"

	"borscht.app/smetana/domain"
	sErrors "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/services"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FeedHandler struct {
	feedService *services.FeedService
}

func NewFeedHandler(service *services.FeedService) *FeedHandler {
	return &FeedHandler{feedService: service}
}

type SubscribeRequest struct {
	Url string `json:"url" validate:"required,url"`
}

// Subscribe godoc
// @Summary Subscribe to an RSS feed
// @Description Subscribe to a valid RSS/Atom feed URL.
// @Tags feeds
// @Accept json
// @Produce json
// @Param request body SubscribeRequest true "Subscribe request"
// @Success 201 {object} domain.Feed
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/feeds [post]
func (h *FeedHandler) Subscribe(c fiber.Ctx) error {
	var req SubscribeRequest
	if err := c.Bind().Body(&req); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	claims, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	feed, err := h.feedService.Subscribe(claims.ID, req.Url)
	if err != nil {
		return sErrors.BadRequest(err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(feed)
}

// Unsubscribe godoc
// @Summary Unsubscribe from a feed
// @Description Unsubscribe from a feed by ID.
// @Tags feeds
// @Accept json
// @Produce json
// @Param id path string true "Feed ID"
// @Success 204
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/feeds/{id} [delete]
func (h *FeedHandler) Unsubscribe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid feed id")
	}

	claims, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.feedService.Unsubscribe(claims.ID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("subscription not found")
		}
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListSubscriptions godoc
// @Summary List subscriptions
// @Description Get all feeds the user is subscribed to.
// @Tags feeds
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Feed]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/feeds [get]
func (h *FeedHandler) ListSubscriptions(c fiber.Ctx) error {
	claims, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	p := types.GetPagination(c)
	feeds, total, err := h.feedService.ListSubscriptions(claims.ID, p.Offset(), p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Feed]{
		Data: feeds,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

// GetStream godoc
// @Summary Get recipe stream
// @Description Get a timeline of recipes from subscribed feeds.
// @Tags feeds
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Recipe]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/feeds/stream [get]
func (h *FeedHandler) GetStream(c fiber.Ctx) error {
	claims, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	p := types.GetPagination(c)
	recipes, total, err := h.feedService.GetStream(claims.ID, p.Page, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}
