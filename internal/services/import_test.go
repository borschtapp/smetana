package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
)

type importServiceDeps struct {
	recipeService *stubRecipeService
	recipeIngest  *stubRecipeIngestService
	feedService   *stubFeedService
	scraper       *stubScraperService
}

func newTestImportService(deps importServiceDeps) domain.ImportService {
	if deps.recipeService == nil {
		deps.recipeService = &stubRecipeService{}
	}
	if deps.recipeIngest == nil {
		deps.recipeIngest = &stubRecipeIngestService{}
	}
	if deps.feedService == nil {
		deps.feedService = &stubFeedService{}
	}
	if deps.scraper == nil {
		deps.scraper = &stubScraperService{}
	}
	return services.NewImportService(deps.recipeService, deps.recipeIngest, deps.feedService, deps.scraper)
}

func TestImportService_ImportFromURL_ExistingRecipe_SavesForUser(t *testing.T) {
	// When a recipe with the URL already exists, the service must save it for
	// the user and return it — no scraping should occur.
	rid := uuid.New()
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/recipe/borsch"

	existing := &domain.Recipe{ID: rid, SourceUrl: &testURL}

	userSaveCalled := false
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) { return existing, nil },
		userSaveFn: func(receivedRid, receivedUid, receivedHid uuid.UUID) error {
			userSaveCalled = true
			assert.Equal(t, rid, receivedRid)
			assert.Equal(t, uid, receivedUid)
			assert.Equal(t, hid, receivedHid)
			return nil
		},
	}
	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc})
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, rid, got.ID)
	assert.True(t, userSaveCalled, "existing recipe must be saved for the user")
}

func TestImportService_ImportFromURL_NewRecipe_ScrapesAndImports(t *testing.T) {
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/new"
	scrapedName := "Fresh Recipe"
	importedID := uuid.New()

	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ context.Context, _ string) (*domain.Recipe, error) {
			return &domain.Recipe{Name: &scrapedName, SourceUrl: &testURL}, nil
		},
	}
	recipeIngest := &stubRecipeIngestService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			r.ID = importedID
			return r, nil
		},
	}
	recipeSvc := &stubRecipeService{
		byUrlFn:    func(_ string, _ uuid.UUID) (*domain.Recipe, error) { return nil, sentinels.ErrNotFound },
		userSaveFn: func(_, _, _ uuid.UUID) error { return nil },
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, recipeIngest: recipeIngest, scraper: scraper})
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, importedID, got.ID)
}

func TestImportService_ImportFromURL_SameURL_DifferentHouseholds_OneCopyStored(t *testing.T) {
	// Importing the same URL from two different households must scrape and persist
	// the recipe exactly once; the second caller gets the cached anonymous copy.
	testURL := "https://example.com/recipe/borsch"
	uid1, hid1 := uuid.New(), uuid.New()
	uid2, hid2 := uuid.New(), uuid.New()
	importedID := uuid.New()

	var stored *domain.Recipe // acts as the in-memory recipe store

	scrapeCallCount := 0
	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ context.Context, _ string) (*domain.Recipe, error) {
			scrapeCallCount++
			return &domain.Recipe{SourceUrl: &testURL}, nil
		},
	}

	importCallCount := 0
	recipeIngest := &stubRecipeIngestService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			importCallCount++
			r.ID = importedID
			stored = r
			return r, nil
		},
	}

	userSaveCallCount := 0
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) {
			if stored != nil {
				return stored, nil // anonymous recipe — visible to every household
			}
			return nil, sentinels.ErrNotFound
		},
		userSaveFn: func(_, _, _ uuid.UUID) error {
			userSaveCallCount++
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, recipeIngest: recipeIngest, scraper: scraper})

	got1, err := svc.ImportFromURL(context.Background(), testURL, false, uid1, hid1)
	require.NoError(t, err)
	require.NotNil(t, got1)
	assert.Equal(t, importedID, got1.ID)

	got2, err := svc.ImportFromURL(context.Background(), testURL, false, uid2, hid2)
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, importedID, got2.ID)

	assert.Equal(t, 1, scrapeCallCount, "URL must be scraped only once")
	assert.Equal(t, 1, importCallCount, "recipe must be imported only once")
	assert.Equal(t, 2, userSaveCallCount, "UserSave must be called once per user")
}

func TestImportService_DetectAndImport_SameURL_DifferentHouseholds_OneCopyStored(t *testing.T) {
	// Same deduplication guarantee as ImportFromURL, but via DetectAndImport.
	testURL := "https://example.com/recipe/borsch"
	uid1, hid1 := uuid.New(), uuid.New()
	uid2, hid2 := uuid.New(), uuid.New()
	importedID := uuid.New()

	var stored *domain.Recipe

	scrapeCallCount := 0
	scraper := &stubScraperService{
		scrapeUrlFn: func(_ context.Context, _ string) (*domain.ScrapeResult, error) {
			scrapeCallCount++
			return &domain.ScrapeResult{
				Type:   domain.PageTypeRecipe,
				Recipe: &domain.Recipe{SourceUrl: &testURL},
			}, nil
		},
	}

	importCallCount := 0
	recipeIngest := &stubRecipeIngestService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			importCallCount++
			r.ID = importedID
			stored = r
			return r, nil
		},
	}

	userSaveCallCount := 0
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) {
			if stored != nil {
				return stored, nil
			}
			return nil, sentinels.ErrNotFound
		},
		userSaveFn: func(_, _, _ uuid.UUID) error {
			userSaveCallCount++
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, recipeIngest: recipeIngest, scraper: scraper})

	res1, err := svc.DetectAndImport(context.Background(), testURL, false, uid1, hid1)
	require.NoError(t, err)
	require.NotNil(t, res1.Recipe)
	assert.Equal(t, importedID, res1.Recipe.ID)

	res2, err := svc.DetectAndImport(context.Background(), testURL, false, uid2, hid2)
	require.NoError(t, err)
	require.NotNil(t, res2.Recipe)
	assert.Equal(t, importedID, res2.Recipe.ID)

	assert.Equal(t, 1, scrapeCallCount, "URL must be scraped only once")
	assert.Equal(t, 1, importCallCount, "recipe must be imported only once")
	assert.Equal(t, 2, userSaveCallCount, "UserSave must be called once per user")
}

func TestImportService_DetectAndImport_FeedURL_SubscribesAndReturnsFeed(t *testing.T) {
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/feed.rss"
	feedID := uuid.New()

	scraper := &stubScraperService{
		scrapeUrlFn: func(_ context.Context, _ string) (*domain.ScrapeResult, error) {
			return &domain.ScrapeResult{
				Type: domain.PageTypeFeed,
				Feed: &domain.Feed{ID: feedID, Url: testURL},
			}, nil
		},
	}
	subscribeCalled := false
	feedSvc := &stubFeedService{
		subscribeFn: func(_ context.Context, receivedHID uuid.UUID, receivedURL string, scraped *domain.Feed) (*domain.Feed, error) {
			subscribeCalled = true
			assert.Equal(t, hid, receivedHID)
			assert.Equal(t, testURL, receivedURL)
			assert.NotNil(t, scraped, "pre-scraped feed must be forwarded to avoid double-scrape")
			return &domain.Feed{ID: feedID}, nil
		},
	}
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) { return nil, sentinels.ErrNotFound },
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, feedService: feedSvc, scraper: scraper})
	result, err := svc.DetectAndImport(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Created)
	assert.NotNil(t, result.Feed)
	assert.Nil(t, result.Recipe)
	assert.True(t, subscribeCalled)
}

func TestImportService_DetectAndImport_ForceUpdate(t *testing.T) {
	// When forceUpdate is true, the service must scrape the URL even if it already exists in the database.
	testURL := "https://example.com/recipe/existing"
	uid, hid := uuid.New(), uuid.New()
	existingID := uuid.New()
	newImportedID := existingID // same ID since we implemented update-in-place

	existing := &domain.Recipe{ID: existingID, SourceUrl: &testURL}

	scrapeCallCount := 0
	scraper := &stubScraperService{
		scrapeUrlFn: func(_ context.Context, _ string) (*domain.ScrapeResult, error) {
			scrapeCallCount++
			return &domain.ScrapeResult{
				Type:   domain.PageTypeRecipe,
				Recipe: &domain.Recipe{SourceUrl: &testURL},
			}, nil
		},
	}

	importCallCount := 0
	recipeIngest := &stubRecipeIngestService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			importCallCount++
			r.ID = newImportedID
			return r, nil
		},
	}

	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) {
			return existing, nil
		},
		userSaveFn: func(_, _, _ uuid.UUID) error { return nil },
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, recipeIngest: recipeIngest, scraper: scraper})

	// forceUpdate = true
	res, err := svc.DetectAndImport(context.Background(), testURL, true, uid, hid)

	require.NoError(t, err)
	require.NotNil(t, res.Recipe)
	assert.Equal(t, 1, scrapeCallCount, "URL must be scraped even if it exists when forceUpdate is true")
	assert.Equal(t, 1, importCallCount, "recipe must be imported even if it exists when forceUpdate is true")
}
