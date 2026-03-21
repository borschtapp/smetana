package types

import (
	"encoding/json"

	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// SearchOptions represents the parameters for searching entities like recipes.
// It includes filters, pagination, and preload options.
type SearchOptions struct {
	SearchQuery  string
	Taxonomies   []uuid.UUID
	Publishers   []uuid.UUID
	Authors      []uuid.UUID
	Equipment    []uuid.UUID
	CookTimeMax  *Duration
	TotalTimeMax *Duration

	Sort  string
	Order string

	Pagination
	PreloadOptions
}

// GetSearchOptions extracts search parameters from the request context.
func GetSearchOptions(c fiber.Ctx) (SearchOptions, error) {
	searchQuery := c.Query("q")
	taxonomies := utils.CsvSplitUUID(c.Query("taxonomies"))
	publishers := utils.CsvSplitUUID(c.Query("publishers"))
	authors := utils.CsvSplitUUID(c.Query("authors"))
	equipment := utils.CsvSplitUUID(c.Query("equipment"))

	cookTimeMax, err := parseDuration(c, "cook_time_max")
	if err != nil {
		return SearchOptions{}, err
	}
	totalTimeMax, err := parseDuration(c, "total_time_max")
	if err != nil {
		return SearchOptions{}, err
	}

	sort := c.Query("sort", "id") // ids are UUIDv7 which are sortable by creation time
	order := c.Query("order", "DESC")

	if !utils.ContainsFold(sort, "id", "name", "updated", "created") {
		return SearchOptions{}, sentinels.BadRequest("invalid sort parameter, must be 'id', 'name', 'updated', or 'created'")
	}

	if !utils.ContainsFold(order, "ASC", "DESC") {
		return SearchOptions{}, sentinels.BadRequest("invalid order parameter, must be 'asc' or 'desc'")
	}

	return SearchOptions{
		SearchQuery:  searchQuery,
		Taxonomies:   taxonomies,
		Publishers:   publishers,
		Authors:      authors,
		Equipment:    equipment,
		CookTimeMax:  cookTimeMax,
		TotalTimeMax: totalTimeMax,

		Sort:           sort,
		Order:          order,
		Pagination:     GetPagination(c),
		PreloadOptions: GetPreloadOptions(c),
	}, nil
}

func parseDuration(c fiber.Ctx, param string) (*Duration, error) {
	raw := c.Query(param)
	if raw == "" {
		return nil, nil
	}
	var d Duration
	if err := json.Unmarshal([]byte(raw), &d); err != nil || d <= 0 {
		return nil, sentinels.BadRequest(param + " must be a positive integer (seconds)")
	}
	return &d, nil
}
