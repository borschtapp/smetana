package types

// ListResponse wraps a list of items with metadata.
type ListResponse[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

// Meta contains pagination metadata.
type Meta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
}
