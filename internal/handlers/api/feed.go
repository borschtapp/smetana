package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
)

type FeedHandler struct {
	feedService domain.FeedService
}

func NewFeedHandler(service domain.FeedService) *FeedHandler {
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/feeds [post]
func (h *FeedHandler) Subscribe(c fiber.Ctx) error {
	var req SubscribeRequest
	if err := c.Bind().Body(&req); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(req); err != nil {
		return sentinels.BadRequestVal(err)
	}

	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	feed, err := h.feedService.Subscribe(c.Context(), claims.HouseholdID, req.Url)
	if err != nil {
		return sentinels.BadRequest(err.Error())
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/feeds/{id} [delete]
func (h *FeedHandler) Unsubscribe(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.feedService.Unsubscribe(claims.HouseholdID, id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListSubscriptions godoc
// @Summary List subscriptions
// @Description ByIDWithRecipes all feeds the user is subscribed to.
// @Tags feeds
// @Accept json
// @Produce json
// @Param q query string false "Text search"
// @Param preload query string false "Comma-separated extras to include: publisher, recipes:5, recipes.images and total_recipes"
// @param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @param order query string false "Sort order: asc or desc (default: desc)"
// @Param page query int false "Page number"
// @param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Feed]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/feeds [get]
func (h *FeedHandler) ListSubscriptions(c fiber.Ctx) error {
	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	feeds, total, err := h.feedService.Search(claims.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Feed]{
		Data: feeds,
		Meta: types.Meta{
			Total: int(total),
			Page:  opts.Page,
		},
	})
}

// ListStream godoc
// @Summary List a timeline of recipes from subscribed feeds.
// @Tags feeds
// @Accept json
// @Produce json
// @Param q query string false "Text search"
// @Param preload query string false "Comma-separated extras to include: publisher, feed, images, ingredients, instructions, taxonomies, collections and saved"
// @param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @param order query string false "Sort order: asc or desc (default: desc)"
// @Param page query int false "Page number"
// @param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Recipe]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/feeds/stream [get]
func (h *FeedHandler) ListStream(c fiber.Ctx) error {
	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	recipes, total, err := h.feedService.Stream(claims.ID, claims.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Total: int(total),
			Page:  opts.Page,
		},
	})
}
