package types

import (
	"github.com/gofiber/fiber/v3"
)

// Pagination represents common pagination parameters.
type Pagination struct {
	Page   int `json:"page,omitempty"`
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// GetPagination extracts pagination parameters from the request context.
// Default values are offset=0, limit=10. Maximum limit is 100.
func GetPagination(c fiber.Ctx) Pagination {
	page := fiber.Query(c, "page", 0)
	offset := fiber.Query(c, "offset", 0)
	limit := fiber.Query(c, "limit", 10)

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	if page > 1 && offset == 0 {
		offset = (page - 1) * limit
	}

	return Pagination{
		Page:   page,
		Offset: offset,
		Limit:  limit,
	}
}
