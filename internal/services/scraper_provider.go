package services

import (
	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/scraper"
)

// ScraperProvider wraps krip calls, so they can be easily mocked
type ScraperProvider interface {
	UrlInput(url string, opts krip.ScrapeOptions) (*krip.DataInput, error)
	Scrape(data *krip.DataInput, target *krip.Recipe, opts krip.ScrapeOptions) error
	ScrapeFeed(data *krip.DataInput, target *krip.Feed, opts krip.FeedOptions) error
	ScrapeUrl(url string, opts krip.ScrapeOptions) (*krip.Recipe, error)
	ScrapeFeedUrl(url string, opts krip.FeedOptions) (*krip.Feed, error)
	ParseIngredient(text string, opts kapusta.IngredientOptions) (kapusta.Ingredient, error)
}

type IngredientParser interface {
	ParseIngredient(text string, opts kapusta.IngredientOptions) (kapusta.Ingredient, error)
}

type KripProvider struct{}

func NewKripProvider() *KripProvider {
	return &KripProvider{}
}

func (p *KripProvider) UrlInput(url string, opts krip.ScrapeOptions) (*krip.DataInput, error) {
	return scraper.UrlInput(url, opts)
}

func (p *KripProvider) Scrape(data *krip.DataInput, target *krip.Recipe, opts krip.ScrapeOptions) error {
	return scraper.Scrape(data, target, opts)
}

func (p *KripProvider) ScrapeFeed(data *krip.DataInput, target *krip.Feed, opts krip.FeedOptions) error {
	return scraper.ScrapeFeed(data, target, opts)
}

func (p *KripProvider) ScrapeUrl(url string, opts krip.ScrapeOptions) (*krip.Recipe, error) {
	return krip.ScrapeUrl(url, opts)
}

func (p *KripProvider) ScrapeFeedUrl(url string, opts krip.FeedOptions) (*krip.Feed, error) {
	return krip.ScrapeFeedUrl(url, opts)
}

func (p *KripProvider) ParseIngredient(text string, opts kapusta.IngredientOptions) (kapusta.Ingredient, error) {
	return kapusta.ParseIngredient(text, opts)
}
