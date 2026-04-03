package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/borschtapp/krip"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type feedService struct {
	repo             domain.FeedRepository
	publisherService domain.PublisherService
	recipeService    domain.RecipeService
	importService    domain.ImportService
	scraperService   domain.ScraperService
}

func NewFeedService(repo domain.FeedRepository, publisherService domain.PublisherService, recipeService domain.RecipeService, importService domain.ImportService, scraperService domain.ScraperService) domain.FeedService {
	return &feedService{
		repo:             repo,
		publisherService: publisherService,
		recipeService:    recipeService,
		importService:    importService,
		scraperService:   scraperService,
	}
}

func (s *feedService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Feed, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *feedService) Stream(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	recipes, total, err := s.recipeService.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts, FromFeeds: true})
	if err != nil || len(recipes) == 0 {
		return recipes, total, err
	}

	// Copy-on-write override: replace global recipes with household copies where they exist.
	parentIDs := make([]uuid.UUID, len(recipes))
	for i, r := range recipes {
		parentIDs[i] = r.ID
	}

	overrides, err := s.recipeService.ByParentIDsAndHousehold(parentIDs, householdID, opts.PreloadOptions)
	if err != nil {
		log.Warnw("stream override lookup failed", "household_id", householdID, "error", err)
		return recipes, total, nil
	}

	// Build a quick lookup: parentID → household copy
	overrideMap := make(map[uuid.UUID]domain.Recipe, len(overrides))
	for _, o := range overrides {
		if o.ParentID != nil {
			overrideMap[*o.ParentID] = o
		}
	}

	// Swap in-place
	for i, r := range recipes {
		if override, ok := overrideMap[r.ID]; ok {
			recipes[i] = override
		}
	}

	return recipes, total, nil
}

func (s *feedService) Subscribe(ctx context.Context, householdID uuid.UUID, url string) (*domain.Feed, error) {
	feed, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrNotFound) {
		return nil, err
	}

	if errors.Is(err, sentinels.ErrNotFound) {
		feed, err = s.createFeed(ctx, url)
		if err != nil {
			log.Warnw("subscribe scrape feed failed", "url", url, "error", err)
			return nil, sentinels.Unprocessable("can't scrape feed url")
		}

		// Submit background task to fetch recipes
		go func(f *domain.Feed) {
			if found, imported, err := s.FetchFeed(context.WithoutCancel(ctx), f); err != nil {
				log.Errorw("background feed fetch failed", "url", url, "error", err)
			} else if found == 0 {
				log.Warnw("background feed fetch found no recipes", "url", url)

				f.Active = false
				if updateErr := s.repo.Update(f); updateErr != nil {
					log.Warnw("failed to deactivate feed with no recipes", "url", url, "error", updateErr)
				}
			} else {
				log.Infow("background feed fetched successfully", "url", url, "found", found, "imported", imported)
			}
		}(new(*feed))
	}

	if err := s.repo.AddFeed(householdID, feed); err != nil {
		return nil, err
	}

	if len(feed.Recipes) == 0 {
		return nil, sentinels.Unprocessable("feed has no importable recipes")
	}
	return feed, nil
}

func (s *feedService) Unsubscribe(householdID uuid.UUID, feedID uuid.UUID) error {
	return s.repo.DeleteFeed(householdID, feedID)
}

func (s *feedService) createFeed(ctx context.Context, url string) (*domain.Feed, error) {
	scrapeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	feed := &domain.Feed{Url: url}
	recipes, err := s.scraperService.ScrapeFeed(scrapeCtx, feed, krip.FeedOptions{SkipEntriesScrape: true})
	if err != nil {
		return nil, fmt.Errorf("invalid feed url: %w", err)
	}

	if feed.Publisher != nil {
		if err := s.publisherService.FindOrCreate(ctx, feed.Publisher); err != nil {
			log.Warnw("error creating publisher", "publisher", feed.Publisher, "error", err)
		} else {
			feed.PublisherID = feed.Publisher.ID
		}
	}

	if err := s.repo.Create(feed); err != nil {
		return nil, err
	}

	feed.Recipes = recipes
	return feed, nil
}

func (s *feedService) FetchFeed(ctx context.Context, feed *domain.Feed) (int, int, error) {
	imported := 0
	feed.LastSyncAt = time.Now()

	scrapeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	opts := krip.FeedOptions{}
	opts.MinIngredients = 3
	opts.Discovered = feed.Discovered
	recipes, err := s.scraperService.ScrapeFeed(scrapeCtx, feed, opts)

	if err != nil {
		log.Warnw("failed to scrape feed", "url", feed.Url, "error", err)
		feed.ErrorCount++
		feed.LastSyncSuccess = false
		if feed.ErrorCount > 3 {
			feed.Active = false
			log.Errorw("deactivating feed due to repeated errors", "url", feed.Url)
		}
		if updateErr := s.repo.Update(feed); updateErr != nil {
			log.Warnw("failed to persist error state for feed", "url", feed.Url, "error", updateErr)
		}
		return 0, 0, err
	}

	feed.ErrorCount = 0
	feed.LastSyncSuccess = true

	for _, recipe := range recipes {
		url := ""
		if recipe.SourceUrl != nil {
			url = *recipe.SourceUrl
		}
		if url == "" {
			continue
		}
		if _, err := s.recipeService.ByUrl(url, uuid.Nil); err == nil {
			continue
		}

		recipe.FeedID = &feed.ID
		if _, err := s.importService.ImportRecipe(ctx, recipe); err != nil {
			log.Warnw("failed to import recipe", "url", url, "error", err)
		} else {
			imported++
			name := ""
			if recipe.Name != nil {
				name = *recipe.Name
			}
			log.Infow("imported new recipe", "name", name)
		}
	}

	if err := s.repo.Update(feed); err != nil {
		log.Warnw("failed to persist feed metadata", "url", feed.Url, "error", err)
	}

	return len(recipes), imported, nil
}
