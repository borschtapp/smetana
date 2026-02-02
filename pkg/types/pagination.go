package types

import (
	"github.com/gofiber/fiber/v2"
)

// Pagination represents common pagination parameters.
type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

// GetPagination extracts pagination parameters from the request context.
// Default values are page=1, limit=10. Maximum limit is 100.
func GetPagination(c *fiber.Ctx) Pagination {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 10)

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return Pagination{
		Page:  page,
		Limit: limit,
	}
}

// Offset returns the starting record index for the current page and limit.
func (p Pagination) Offset() int {
	return (p.Page - 1) * p.Limit
}
