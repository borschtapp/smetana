package services

import "github.com/google/uuid"

// canonicalize rewrites, in place, each non-nil ID that lookup reports as an alias.
func canonicalize(lookup func([]uuid.UUID) (map[uuid.UUID]uuid.UUID, error), ptrs []*uuid.UUID) error {
	ids := make([]uuid.UUID, 0, len(ptrs))
	for _, p := range ptrs {
		if p != nil {
			ids = append(ids, *p)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	canonical, err := lookup(ids)
	if err != nil {
		return err
	}
	for _, p := range ptrs {
		if p == nil {
			continue
		}
		if c, ok := canonical[*p]; ok {
			*p = c
		}
	}
	return nil
}
