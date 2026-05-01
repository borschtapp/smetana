package domain

import (
	"context"

	"github.com/borschtapp/krip"
)

type PageType uint8

const (
	PageTypeRecipe PageType = 1
	PageTypeFeed   PageType = 2
)

type ScrapeResult struct {
	Type   PageType `json:"type"`
	Recipe *Recipe  `json:"recipe,omitempty"`
	Feed   *Feed    `json:"feed,omitempty"`
}

// ScraperService fetches and converts external recipe data into domain objects.
type ScraperService interface {
	ScrapeUrl(ctx context.Context, url string, requestedType string) (*ScrapeResult, error)
	ScrapeRecipe(ctx context.Context, url string) (*Recipe, error)
	ScrapeFeed(ctx context.Context, feed *Feed, opts krip.FeedOptions) ([]*Recipe, error)
}
