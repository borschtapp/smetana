package domain

import (
	"context"

	"github.com/google/uuid"
)

type ImportResult struct {
	Created bool    `json:"created"`
	Recipe  *Recipe `json:"recipe,omitempty"`
	Feed    *Feed   `json:"feed,omitempty"`
}

type ImportService interface {
	// ImportFromURL scrapes the URL and imports it as a recipe. Returns an error if the URL points to a feed.
	ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*Recipe, error)
	// DetectAndImport scrapes the URL and auto-detects whether it is a recipe or a feed, importing accordingly.
	DetectAndImport(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*ImportResult, error)
}

type RecipeIngestService interface {
	ImportRecipe(ctx context.Context, recipe *Recipe) (*Recipe, error)
}
