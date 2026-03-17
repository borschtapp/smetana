package types

import (
	"github.com/gofiber/fiber/v3"
)

// Pagination represents common pagination parameters.
type Pagination struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

// GetPagination extracts pagination parameters from the request context.
// Default values are offset=0, limit=10. Maximum limit is 100.
func GetPagination(c fiber.Ctx) Pagination {
	offset := fiber.Query(c, "offset", 0)
	limit := fiber.Query(c, "limit", 10)

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return Pagination{
		Offset: offset,
		Limit:  limit,
	}
}
