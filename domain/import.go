package domain

import (
	"context"

	"github.com/google/uuid"
)

type ImportService interface {
	ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*Recipe, error)
	ImportRecipe(ctx context.Context, recipe *Recipe) (*Recipe, error)
}
