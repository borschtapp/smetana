package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/types"
)

func newTestFeedService(
	feedRepo *stubFeedRepo,
	pubRepo *stubPublisherRepo,
	recipeRepo *stubRecipeRepo,
	recipeSvc *stubRecipeService,
	scraper *stubScraperService,
) domain.FeedService {
	return services.NewFeedService(feedRepo, pubRepo, recipeRepo, recipeSvc, scraper)
}

func TestFeedService_Stream_NoRecipes_ReturnsEmpty(t *testing.T) {
	recipeRepo := &stubRecipeRepo{
		searchFn: func(_, _ uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			return nil, 0, nil
		},
	}

	svc := newTestFeedService(&stubFeedRepo{}, &stubPublisherRepo{}, recipeRepo, &stubRecipeService{}, &stubScraperService{})
	recipes, total, err := svc.Stream(uuid.New(), uuid.New(), types.SearchOptions{})

	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
	assert.Empty(t, recipes)
}

func TestFeedService_Stream_SwapsGlobalWithHouseholdOverrides(t *testing.T) {
	// Given two global recipes and one household override for the first,
	// Stream must replace the first recipe in the result with its override.
	globalID1 := uuid.New()
	globalID2 := uuid.New()
	hid := uuid.New()

	overrideID := uuid.New()
	global1 := domain.Recipe{ID: globalID1}
	global2 := domain.Recipe{ID: globalID2}
	override := domain.Recipe{ID: overrideID, ParentID: &globalID1, HouseholdID: &hid}

	recipeRepo := &stubRecipeRepo{
		searchFn: func(_, _ uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			return []domain.Recipe{global1, global2}, 2, nil
		},
		byParentIDsAndHouseholdFn: func(ids []uuid.UUID, h uuid.UUID) ([]domain.Recipe, error) {
			assert.Equal(t, hid, h)
			assert.Len(t, ids, 2)
			return []domain.Recipe{override}, nil
		},
	}

	svc := newTestFeedService(&stubFeedRepo{}, &stubPublisherRepo{}, recipeRepo, &stubRecipeService{}, &stubScraperService{})
	got, total, err := svc.Stream(uuid.New(), hid, types.SearchOptions{})

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	require.Len(t, got, 2)
	assert.Equal(t, overrideID, got[0].ID, "global recipe must be replaced by household override")
	assert.Equal(t, globalID2, got[1].ID, "recipe without override must be returned as-is")
}

func TestFeedService_Stream_OverrideLookupFailure_ReturnsOriginalResults(t *testing.T) {
	// If ByParentIDsAndHousehold fails, Stream must return the original results
	// non-fatally (warning logged, no error returned to caller).
	globalID := uuid.New()
	global := domain.Recipe{ID: globalID}

	recipeRepo := &stubRecipeRepo{
		searchFn: func(_, _ uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			return []domain.Recipe{global}, 1, nil
		},
		byParentIDsAndHouseholdFn: func(_ []uuid.UUID, _ uuid.UUID) ([]domain.Recipe, error) {
			return nil, errors.New("db temporarily unavailable")
		},
	}

	svc := newTestFeedService(&stubFeedRepo{}, &stubPublisherRepo{}, recipeRepo, &stubRecipeService{}, &stubScraperService{})
	got, total, err := svc.Stream(uuid.New(), uuid.New(), types.SearchOptions{})

	require.NoError(t, err, "override lookup failure must not propagate as an error")
	assert.EqualValues(t, 1, total)
	require.Len(t, got, 1)
	assert.Equal(t, globalID, got[0].ID, "original recipe must be returned when override lookup fails")
}

func TestFeedService_FetchFeed_ScrapeError_IncrementsErrorCount(t *testing.T) {
	feed := domain.Feed{ID: uuid.New(), Active: true, ErrorCount: 2, Url: "https://bad.feed"}
	var updatedFeed *domain.Feed

	feedRepo := &stubFeedRepo{
		updateFn: func(f *domain.Feed) error {
			updatedFeed = f
			return nil
		},
	}
	scraper := &stubScraperService{
		scrapeFeedFn: func(_ context.Context, _ string, _ domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
			return nil, errors.New("feed unreachable")
		},
	}

	svc := newTestFeedService(feedRepo, &stubPublisherRepo{}, &stubRecipeRepo{}, &stubRecipeService{}, scraper)
	_, _, err := svc.FetchFeed(context.Background(), &feed)

	require.Error(t, err)
	require.NotNil(t, updatedFeed)
	assert.Equal(t, 3, updatedFeed.ErrorCount, "error count must increment on scrape failure")
	assert.True(t, updatedFeed.Active, "feed must remain active below threshold")
}

func TestFeedService_FetchFeed_ExceedErrorThreshold_DeactivatesFeed(t *testing.T) {
	// After 3 consecutive errors the feed must be deactivated.
	feed := domain.Feed{ID: uuid.New(), Active: true, ErrorCount: 3, Url: "https://dead.feed"}
	var updatedFeed *domain.Feed

	feedRepo := &stubFeedRepo{
		updateFn: func(f *domain.Feed) error { updatedFeed = f; return nil },
	}
	scraper := &stubScraperService{
		scrapeFeedFn: func(_ context.Context, _ string, _ domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
			return nil, errors.New("still broken")
		},
	}

	svc := newTestFeedService(feedRepo, &stubPublisherRepo{}, &stubRecipeRepo{}, &stubRecipeService{}, scraper)
	_, _, err := svc.FetchFeed(context.Background(), &feed)

	require.Error(t, err)
	require.NotNil(t, updatedFeed)
	assert.False(t, updatedFeed.Active, "feed must be deactivated after exceeding the error threshold")
	assert.Equal(t, 4, updatedFeed.ErrorCount)
}

func TestFeedService_FetchFeed_SkipsAlreadyImportedRecipes(t *testing.T) {
	feedID := uuid.New()
	feed := domain.Feed{ID: feedID, Active: true, Url: "https://good.feed"}

	scraped := &domain.Recipe{IsBasedOn: new("https://recipe.example.com/borsch")}
	feedRepo := &stubFeedRepo{
		updateFn: func(_ *domain.Feed) error { return nil },
	}
	scraper := &stubScraperService{
		scrapeFeedFn: func(_ context.Context, _ string, _ domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
			return []*domain.Recipe{scraped}, nil
		},
	}
	recipeRepo := &stubRecipeRepo{
		// ByUrl returning no error means the recipe already exists
		byUrlFn: func(_ string) (*domain.Recipe, error) {
			return &domain.Recipe{ID: uuid.New()}, nil
		},
	}
	importCalled := false
	recipeSvc := &stubRecipeService{
		importRecipeFn: func(_ context.Context, _ *domain.Recipe) (*domain.Recipe, error) {
			importCalled = true
			return nil, nil
		},
	}

	svc := newTestFeedService(feedRepo, &stubPublisherRepo{}, recipeRepo, recipeSvc, scraper)
	_, _, err := svc.FetchFeed(context.Background(), &feed)

	require.NoError(t, err)
	assert.False(t, importCalled, "ImportRecipe must not be called for already-imported recipes")
}

func TestFeedService_FetchFeed_NewRecipe_ImportsAndAssignsFeedID(t *testing.T) {
	feedID := uuid.New()
	feed := domain.Feed{ID: feedID, Active: true, Url: "https://good.feed"}

	scraped := &domain.Recipe{IsBasedOn: new("https://recipe.example.com/new")}
	feedRepo := &stubFeedRepo{
		updateFn: func(_ *domain.Feed) error { return nil },
	}
	scraper := &stubScraperService{
		scrapeFeedFn: func(_ context.Context, _ string, _ domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
			return []*domain.Recipe{scraped}, nil
		},
	}
	recipeRepo := &stubRecipeRepo{
		// ByUrl returning ErrNotFound means recipe is new
		byUrlFn: func(_ string) (*domain.Recipe, error) {
			return nil, sentinels.ErrNotFound
		},
	}
	var importedRecipe *domain.Recipe
	recipeSvc := &stubRecipeService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			importedRecipe = r
			r.ID, _ = uuid.NewV7()
			return r, nil
		},
	}

	svc := newTestFeedService(feedRepo, &stubPublisherRepo{}, recipeRepo, recipeSvc, scraper)
	_, _, err := svc.FetchFeed(context.Background(), &feed)

	require.NoError(t, err)
	require.NotNil(t, importedRecipe)
	assert.Equal(t, feedID, *importedRecipe.FeedID, "imported recipe must be linked to the source feed")
}

func TestFeedService_FetchFeed_RecipeWithNoURL_IsSkipped(t *testing.T) {
	// Recipes without IsBasedOn (no URL) cannot be deduplicated and must be
	// skipped to avoid importing the same recipe repeatedly.
	feed := domain.Feed{ID: uuid.New(), Active: true, Url: "https://good.feed"}
	noURLRecipe := &domain.Recipe{IsBasedOn: nil}

	feedRepo := &stubFeedRepo{
		updateFn: func(_ *domain.Feed) error { return nil },
	}
	scraper := &stubScraperService{
		scrapeFeedFn: func(_ context.Context, _ string, _ domain.FeedScrapeOptions) ([]*domain.Recipe, error) {
			return []*domain.Recipe{noURLRecipe}, nil
		},
	}
	importCalled := false
	recipeSvc := &stubRecipeService{
		importRecipeFn: func(_ context.Context, _ *domain.Recipe) (*domain.Recipe, error) {
			importCalled = true
			return nil, nil
		},
	}

	svc := newTestFeedService(feedRepo, &stubPublisherRepo{}, &stubRecipeRepo{}, recipeSvc, scraper)
	_, _, err := svc.FetchFeed(context.Background(), &feed)

	require.NoError(t, err)
	assert.False(t, importCalled, "recipe without URL must be skipped")
}
