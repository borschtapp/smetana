package domain

import (
	"context"

	"github.com/borschtapp/krip"
)

// ScraperService fetches and converts external recipe data into domain objects.
type ScraperService interface {
	ScrapeRecipe(ctx context.Context, url string) (*Recipe, error)
	ScrapeFeed(ctx context.Context, feed *Feed, opts krip.FeedOptions) ([]*Recipe, error)
}
