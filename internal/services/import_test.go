package services_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/types"
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
		scrapeUrlFn: func(_ context.Context, _ string, _ string) (*domain.ScrapeResult, error) {
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

	res1, err := svc.DetectAndImport(context.Background(), testURL, domain.ImportTypeAuto, false, uid1, hid1)
	require.NoError(t, err)
	require.NotNil(t, res1.Recipe)
	assert.Equal(t, importedID, res1.Recipe.ID)

	res2, err := svc.DetectAndImport(context.Background(), testURL, domain.ImportTypeAuto, false, uid2, hid2)
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
		scrapeUrlFn: func(_ context.Context, _ string, _ string) (*domain.ScrapeResult, error) {
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
	result, err := svc.DetectAndImport(context.Background(), testURL, domain.ImportTypeAuto, false, uid, hid)

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
		scrapeUrlFn: func(_ context.Context, _ string, _ string) (*domain.ScrapeResult, error) {
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
	res, err := svc.DetectAndImport(context.Background(), testURL, domain.ImportTypeAuto, true, uid, hid)

	require.NoError(t, err)
	require.NotNil(t, res.Recipe)
	assert.Equal(t, 1, scrapeCallCount, "URL must be scraped even if it exists when forceUpdate is true")
	assert.Equal(t, 1, importCallCount, "recipe must be imported even if it exists when forceUpdate is true")
}

func TestImportService_RefreshRecipe_Success(t *testing.T) {
	rid := uuid.New()
	uid := uuid.New()
	hid := uuid.New()
	sourceURL := "https://example.com/recipe/borsch"

	existing := &domain.Recipe{
		ID:        rid,
		SourceUrl: &sourceURL,
	}

	canonicalURL := "https://example.com/recipes/borsch-classic"
	scrapedRecipe := &domain.Recipe{
		SourceUrl:    &canonicalURL,
		Name:         new("Freshly Scraped"),
		Ingredients:  []*domain.RecipeIngredient{{}},
		Instructions: []*domain.RecipeInstruction{{Text: "Simmer"}},
	}

	preloadedRecipe := &domain.Recipe{ID: rid}

	scrapeCalled := false
	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ context.Context, u string) (*domain.Recipe, error) {
			scrapeCalled = true
			assert.Equal(t, sourceURL, u)
			return scrapedRecipe, nil
		},
	}

	ingestCalled := false
	recipeIngest := &stubRecipeIngestService{
		importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
			ingestCalled = true
			assert.Equal(t, rid, r.ID, "scraped recipe must carry the known recipe ID directly into ingest")
			assert.Equal(t, sourceURL, *r.SourceUrl, "refresh must keep the stored source URL, not the scraped canonical one")
			return r, nil
		},
	}

	recipeSvc := &stubRecipeService{
		byIDFn: func(id, h uuid.UUID) (*domain.Recipe, error) {
			assert.Equal(t, rid, id)
			assert.Equal(t, hid, h)
			return existing, nil
		},
		byIDPreloadFn: func(id, u, h uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			assert.Equal(t, rid, id)
			return preloadedRecipe, nil
		},
	}

	svc := newTestImportService(importServiceDeps{
		recipeService: recipeSvc,
		recipeIngest:  recipeIngest,
		scraper:       scraper,
	})

	refreshed, err := svc.RefreshRecipe(context.Background(), rid, uid, hid)
	require.NoError(t, err)
	require.NotNil(t, refreshed)
	assert.True(t, scrapeCalled)
	assert.True(t, ingestCalled)
	assert.Same(t, preloadedRecipe, refreshed, "refresh must return the reloaded recipe")
}

func TestImportService_RefreshRecipe_Rejects(t *testing.T) {
	hid := uuid.New()
	sourceURL := "https://example.com/recipe/borsch"

	tests := []struct {
		name       string
		byID       *domain.Recipe
		byIDErr    error
		wantStatus int
	}{
		{name: "no source url", byID: &domain.Recipe{}, wantStatus: fiber.StatusBadRequest},
		{name: "customized recipe", byID: &domain.Recipe{HouseholdID: &hid, SourceUrl: &sourceURL}, wantStatus: fiber.StatusBadRequest},
		{name: "not found", byIDErr: sentinels.ErrNotFound, wantStatus: fiber.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recipeSvc := &stubRecipeService{
				byIDFn: func(id, h uuid.UUID) (*domain.Recipe, error) {
					return tt.byID, tt.byIDErr
				},
			}
			svc := newTestImportService(importServiceDeps{recipeService: recipeSvc})

			refreshed, err := svc.RefreshRecipe(context.Background(), uuid.New(), uuid.New(), hid)
			assert.Nil(t, refreshed)
			var se *sentinels.Error
			require.ErrorAs(t, err, &se)
			assert.Equal(t, tt.wantStatus, se.Status)
		})
	}
}

func TestImportService_RefreshRecipe_CollapsesConcurrentCalls(t *testing.T) {
	// synctest only advances the clock once all goroutines block, so the second caller joins
	// the refresh before the scrape returns.
	synctest.Test(t, func(t *testing.T) {
		rid := uuid.New()
		uid := uuid.New()
		hid := uuid.New()
		sourceURL := "https://example.com/recipe/concurrent"

		existing := &domain.Recipe{
			ID:        rid,
			SourceUrl: &sourceURL,
		}

		scrapedRecipe := &domain.Recipe{
			SourceUrl:    &sourceURL,
			Name:         new("Concurrent Recipe"),
			Ingredients:  []*domain.RecipeIngredient{{}},
			Instructions: []*domain.RecipeInstruction{{Text: "Stir"}},
		}

		var scrapeCallCount atomic.Int32
		scraper := &stubScraperService{
			scrapeRecipeFn: func(_ context.Context, u string) (*domain.Recipe, error) {
				scrapeCallCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				return scrapedRecipe, nil
			},
		}

		var ingestCallCount atomic.Int32
		recipeIngest := &stubRecipeIngestService{
			importRecipeFn: func(_ context.Context, r *domain.Recipe) (*domain.Recipe, error) {
				ingestCallCount.Add(1)
				return r, nil
			},
		}

		recipeSvc := &stubRecipeService{
			byIDFn: func(id, h uuid.UUID) (*domain.Recipe, error) {
				return existing, nil
			},
			byIDPreloadFn: func(id, u, h uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
				return scrapedRecipe, nil
			},
		}

		svc := newTestImportService(importServiceDeps{
			recipeService: recipeSvc,
			recipeIngest:  recipeIngest,
			scraper:       scraper,
		})

		var wg sync.WaitGroup
		var errs [2]error
		var results [2]*domain.Recipe
		for i := range results {
			wg.Go(func() {
				results[i], errs[i] = svc.RefreshRecipe(context.Background(), rid, uid, hid)
			})
		}
		wg.Wait()

		for i := range results {
			require.NoError(t, errs[i])
			assert.NotNil(t, results[i])
		}
		assert.EqualValues(t, 1, scrapeCallCount.Load(), "concurrent refresh calls for the same recipe must only scrape once")
		assert.EqualValues(t, 1, ingestCallCount.Load(), "concurrent refresh calls for the same recipe must only ingest once")
	})
}
