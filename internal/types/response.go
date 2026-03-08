package types

import "encoding/json"

// Meta contains pagination metadata.
type Meta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
}

// ListResponse wraps a list of items with metadata.
type ListResponse[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}

// MarshalJSON ensures Data is serialized as [] instead of null when empty.
func (r ListResponse[T]) MarshalJSON() ([]byte, error) {
	data := r.Data
	if data == nil {
		data = []T{}
	}

	return json.Marshal(struct {
		Data []T  `json:"data"`
		Meta Meta `json:"meta"`
	}{Data: data, Meta: r.Meta})
}
