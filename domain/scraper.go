package domain

type FeedScrapeOptions struct {
	Quick               bool
	MinIngredients      int
	RequireImage        bool
	RequireInstructions bool
}

// ScraperService fetches and converts external recipe data into domain objects.
type ScraperService interface {
	ScrapeRecipe(url string) (*Recipe, error)
	ScrapeFeed(url string, opts FeedScrapeOptions) ([]*Recipe, error)
}
