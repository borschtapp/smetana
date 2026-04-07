package api

import (
	"borscht.app/smetana/domain"
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
	if err := bindBody(c, &req); err != nil {
		return err
	}

	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	feed, err := h.feedService.Subscribe(c.Context(), claims.HouseholdID, req.Url)
	if err != nil {
		return err
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
// @Param preload query string false "Comma-separated extras to include: publisher, last3_recipes and total_recipes"
// @Param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
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
	if err := opts.Validate("publisher", "last3_recipes", "total_recipes"); err != nil {
		return err
	}

	feeds, total, err := h.feedService.Search(claims.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Feed]{
		Data: feeds,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
}

// Sync godoc
// @Summary Sync a feed
// @Description Trigger an immediate synchronization of the feed, importing any new recipes. This call is synchronous and blocks until the sync completes. If the connection drops, the sync continues server-side.
// @Tags feeds
// @Accept json
// @Produce json
// @Param id path string true "Feed ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/feeds/{id}/sync [post]
func (h *FeedHandler) Sync(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.feedService.Sync(c.Context(), claims.HouseholdID, id); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListStream godoc
// @Summary List a timeline of recipes from subscribed feeds.
// @Tags feeds
// @Accept json
// @Produce json
// @Param q query string false "Text search"
// @Param taxonomies query string false "Comma-separated taxonomy IDs to filter by (using OR logic)"
// @Param publishers query string false "Comma-separated publisher IDs to filter by"
// @Param authors query string false "Comma-separated author IDs to filter by"
// @Param equipment query string false "Comma-separated equipment IDs to filter by"
// @Param cook_time_max query int false "Max cook time in seconds (e.g. 1800 = 30 min)"
// @Param total_time_max query int false "Max total time in seconds (e.g. 3600 = 1 hour)"
// @Param preload query string false "Comma-separated extras to include: publisher, author, feed, images, ingredients, equipment, instructions, nutrition, taxonomies, collections and saved"
// @Param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
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
	if err := opts.Validate("publisher", "author", "feed", "images", "ingredients", "equipment", "instructions", "nutrition", "taxonomies", "collections", "saved"); err != nil {
		return err
	}

	recipes, total, err := h.feedService.Stream(claims.ID, claims.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
}
