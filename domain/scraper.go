package domain

import "context"

type FeedScrapeOptions struct {
	Quick               bool
	MinIngredients      int
	RequireImage        bool
	RequireInstructions bool
}

// ScraperService fetches and converts external recipe data into domain objects.
type ScraperService interface {
	ScrapeRecipe(ctx context.Context, url string) (*Recipe, error)
	ScrapeFeed(ctx context.Context, url string, opts FeedScrapeOptions) ([]*Recipe, error)
}
