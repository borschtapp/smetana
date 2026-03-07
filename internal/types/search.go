package types

import (
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// SearchOptions represents the parameters for searching entities like recipes.
// It includes filters, pagination, and preload options.
type SearchOptions struct {
	SearchQuery string
	Taxonomies  []uuid.UUID
	Preload     []string

	Sort  string
	Order string

	Pagination
}

// GetSearchOptions extracts search parameters from the request context.
func GetSearchOptions(c fiber.Ctx) (SearchOptions, error) {
	searchQuery := c.Query("q")
	taxonomies := utils.CsvSplitUUID(c.Query("taxonomies"))
	preload := utils.CsvSplit(c.Query("preload"))

	sort := c.Query("sort", "id") // ids are UUIDv7 which are sortable by creation time
	order := c.Query("order", "DESC")

	if !utils.ContainsFold(sort, "id", "name", "updated", "created") {
		return SearchOptions{}, sentinels.BadRequest("invalid sort parameter, must be 'id', 'name', 'updated', or 'created'")
	}

	if !utils.ContainsFold(order, "ASC", "DESC") {
		return SearchOptions{}, sentinels.BadRequest("invalid order parameter, must be 'asc' or 'desc'")
	}

	return SearchOptions{
		SearchQuery: searchQuery,
		Taxonomies:  taxonomies,
		Preload:     preload,
		Sort:        sort,
		Order:       order,
		Pagination:  GetPagination(c),
	}, nil
}
