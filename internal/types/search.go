package types

import (
	"encoding/json"
	"slices"
	"strings"

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
	Scope string

	Pagination
	PreloadOptions
}

var baseSortFields = []string{"id", "name", "updated", "created"}

// SearchConfig defines validation rules applied inside GetSearchOptions.
type SearchConfig struct {
	AllowedSorts    []string
	AllowedPreloads []string
}

// GetSearchOptions extracts and validates search parameters from the request context.
func GetSearchOptions(c fiber.Ctx, cfg SearchConfig) (SearchOptions, error) {
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
	order := strings.ToUpper(c.Query("order", "DESC"))
	scope := c.Query("scope")

	if order != "ASC" && order != "DESC" {
		return SearchOptions{}, sentinels.BadRequest("invalid order parameter, must be 'ASC' or 'DESC'")
	}

	validSorts := slices.Concat(baseSortFields, cfg.AllowedSorts)
	if !utils.ContainsFold(sort, validSorts...) {
		return SearchOptions{}, sentinels.BadRequest("invalid sort parameter, must be one of: " + strings.Join(validSorts, ", "))
	}

	if scope != "" && !utils.ContainsFold(scope, "feeds", "saved") {
		return SearchOptions{}, sentinels.BadRequest("invalid scope parameter, must be 'feeds' or 'saved'")
	}

	preloadOpts := GetPreloadOptions(c)
	if err := preloadOpts.Validate(cfg.AllowedPreloads...); err != nil {
		return SearchOptions{}, err
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
		Scope:          scope,
		Pagination:     GetPagination(c),
		PreloadOptions: preloadOpts,
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
