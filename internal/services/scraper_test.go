package services

import (
	"context"
	"testing"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"borscht.app/smetana/domain"
)

type mockScraperProvider struct {
	mock.Mock
}

func (m *mockScraperProvider) UrlInput(url string, opts krip.ScrapeOptions) (*krip.DataInput, error) {
	args := m.Called(url, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*krip.DataInput), args.Error(1)
}

func (m *mockScraperProvider) Scrape(data *krip.DataInput, target *krip.Recipe, opts krip.ScrapeOptions) error {
	args := m.Called(data, target, opts)
	if recipe := args.Get(0); recipe != nil {
		*target = *(recipe.(*krip.Recipe))
	}
	return args.Error(1)
}

func (m *mockScraperProvider) ScrapeFeed(data *krip.DataInput, target *krip.Feed, opts krip.FeedOptions) error {
	args := m.Called(data, target, opts)
	if feed := args.Get(0); feed != nil {
		*target = *(feed.(*krip.Feed))
	}
	return args.Error(1)
}

func (m *mockScraperProvider) ScrapeUrl(url string, opts krip.ScrapeOptions) (*krip.Recipe, error) {
	args := m.Called(url, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*krip.Recipe), args.Error(1)
}

func (m *mockScraperProvider) ScrapeFeedUrl(url string, opts krip.FeedOptions) (*krip.Feed, error) {
	args := m.Called(url, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*krip.Feed), args.Error(1)
}

func (m *mockScraperProvider) ParseIngredient(text string, opts kapusta.IngredientOptions) (kapusta.Ingredient, error) {
	args := m.Called(text, opts)
	return args.Get(0).(kapusta.Ingredient), args.Error(1)
}

func newTestScraperService(t *testing.T) (*scraperService, *mockScraperProvider) {
	t.Helper()
	p := &mockScraperProvider{}
	p.On("ParseIngredient", mock.Anything, mock.Anything).Return(kapusta.Ingredient{}, nil).Maybe()
	svc := NewScraperService(p, p).(*scraperService)
	return svc, p
}

func TestScraperService_ScrapeUrl_Recipe(t *testing.T) {
	service, mockProvider := newTestScraperService(t)

	url := "https://example.com/recipe"
	data := &krip.DataInput{}
	recipe := &krip.Recipe{
		Url:          url,
		Name:         "Strong Recipe",
		Ingredients:  []*krip.PropertyValue{{Name: "Ingredient 1"}},
		Instructions: []*krip.HowToSection{{HowToStep: krip.HowToStep{Text: "Step 1"}}},
	}

	mockProvider.On("UrlInput", url, mock.Anything).Return(data, nil)
	mockProvider.On("Scrape", data, mock.Anything, mock.Anything).Return(recipe, nil)
	mockProvider.On("ScrapeFeed", data, mock.Anything, mock.Anything).Return(&krip.Feed{}, nil)

	res, err := service.ScrapeUrl(context.Background(), url, domain.ImportTypeAuto)

	assert.NoError(t, err)
	assert.Equal(t, domain.PageTypeRecipe, res.Type)
	assert.Equal(t, "Strong Recipe", *res.Recipe.Name)
	mockProvider.AssertExpectations(t)
}

func TestScraperService_ScrapeUrl_Feed(t *testing.T) {
	service, mockProvider := newTestScraperService(t)

	url := "https://example.com/feed"
	data := &krip.DataInput{}
	feed := &krip.Feed{
		Url:     url,
		Name:    "Test Feed",
		Entries: []*krip.Recipe{{Name: "Entry 1"}},
	}

	mockProvider.On("UrlInput", url, mock.Anything).Return(data, nil)
	mockProvider.On("Scrape", data, mock.Anything, mock.Anything).Return(&krip.Recipe{}, nil)
	mockProvider.On("ScrapeFeed", data, mock.Anything, mock.Anything).Return(feed, nil)

	res, err := service.ScrapeUrl(context.Background(), url, domain.ImportTypeAuto)

	assert.NoError(t, err)
	assert.Equal(t, domain.PageTypeFeed, res.Type)
	assert.Equal(t, "Test Feed", res.Feed.Name)
	mockProvider.AssertExpectations(t)
}
