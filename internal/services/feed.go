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
	"borscht.app/smetana/internal/utils"
)

type feedService struct {
	repo             domain.FeedRepository
	publisherService domain.PublisherService
	recipeService    domain.RecipeService
	recipeIngest     domain.RecipeIngestService
	scraperService   domain.ScraperService
	syncLimit        chan struct{}
}

func NewFeedService(repo domain.FeedRepository, publisherService domain.PublisherService, recipeService domain.RecipeService, recipeIngest domain.RecipeIngestService, scraperService domain.ScraperService) domain.FeedService {
	return &feedService{
		repo:             repo,
		publisherService: publisherService,
		recipeService:    recipeService,
		recipeIngest:     recipeIngest,
		scraperService:   scraperService,
		syncLimit:        make(chan struct{}, utils.GetenvInt("FEED_SYNC_CONCURRENCY", 2)),
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
		log.Warnw("stream override lookup failed", "household_id", householdID, "error", err.Error())
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

func (s *feedService) Subscribe(ctx context.Context, householdID uuid.UUID, url string, scraped *domain.Feed) (*domain.Feed, error) {
	feed, err := s.findOrCreate(ctx, url, scraped)
	if err != nil {
		return nil, err
	}

	// Already subscribed — return existing subscription idempotently.
	if existing, err := s.repo.ByIDForHousehold(feed.ID, householdID); err == nil {
		return existing, nil
	}

	// Validation: ensure we found something (recipes or at least a feed name).
	// New feeds might have 0 recipes during initial shallow scrape but will sync in background.
	if len(feed.Recipes) == 0 && feed.Name == "" {
		return nil, sentinels.Unprocessable("feed has no importable recipes")
	}

	if err := s.repo.AddFeed(householdID, feed); err != nil {
		return nil, err
	}

	return feed, nil
}

func (s *feedService) Unsubscribe(householdID uuid.UUID, feedID uuid.UUID) error {
	return s.repo.DeleteFeed(householdID, feedID)
}

func (s *feedService) findOrCreate(ctx context.Context, url string, scraped *domain.Feed) (*domain.Feed, error) {
	url = utils.NormalizeURL(url)
	feed, err := s.repo.ByUrl(url)
	if err == nil {
		return feed, nil
	}
	if !errors.Is(err, sentinels.ErrNotFound) {
		return nil, err
	}

	if scraped != nil {
		feed = scraped
		feed.Url = url
	} else {
		scrapeCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		feed = &domain.Feed{Url: url}
		recipes, err := s.scraperService.ScrapeFeed(scrapeCtx, feed, krip.FeedOptions{SkipEntriesScrape: true})
		if err != nil {
			return nil, fmt.Errorf("invalid feed url: %w", err)
		}
		feed.Recipes = recipes
	}

	if feed.Publisher == nil {
		feed.Publisher = &domain.Publisher{Name: feed.Name, Url: new(utils.BaseURL(feed.Url))}
	}

	if err := s.publisherService.FindOrCreate(ctx, feed.Publisher); err != nil {
		log.Warnw("error creating publisher", "publisher", feed.Publisher, "error", err.Error())
	} else {
		feed.PublisherID = feed.Publisher.ID
	}

	if err := s.repo.Create(feed); err != nil {
		return nil, err
	}

	// Submit background task to fetch recipes
	go func(f domain.Feed) {
		s.syncLimit <- struct{}{}
		defer func() { <-s.syncLimit }()

		if found, imported, err := s.FetchFeed(context.WithoutCancel(ctx), &f); err != nil {
			log.Errorw("background feed fetch failed", "url", url, "error", err.Error())
		} else if found == 0 {
			log.Warnw("background feed fetch found no recipes", "url", url)

			f.Active = false
			if updateErr := s.repo.Update(&f); updateErr != nil {
				log.Warnw("failed to deactivate feed with no recipes", "url", url, "error", updateErr.Error())
			}
		} else {
			log.Infow("background feed fetched successfully", "url", url, "found", found, "imported", imported)
		}
	}(*feed)

	return feed, nil
}

func (s *feedService) Sync(ctx context.Context, householdID uuid.UUID, feedID uuid.UUID) (int, int, error) {
	feed, err := s.repo.ByIDForHousehold(feedID, householdID)
	if err != nil {
		return 0, 0, err
	}

	return s.FetchFeed(context.WithoutCancel(ctx), feed)
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
		log.Warnw("failed to scrape feed", "feed", feed.ID, "error", err.Error())
		feed.ErrorCount++
		feed.LastSyncSuccess = false
		if feed.ErrorCount > 3 {
			feed.Active = false
			log.Errorw("deactivating feed due to repeated errors", "feed", feed.ID)
		}
		if updateErr := s.repo.Update(feed); updateErr != nil {
			log.Warnw("failed to persist error state for feed", "feed", feed.ID, "error", updateErr.Error())
		}
		return 0, 0, err
	}

	feed.ErrorCount = 0
	feed.LastSyncSuccess = true

	for _, recipe := range recipes {
		url := ""
		if recipe.SourceUrl != nil {
			url = utils.NormalizeURL(*recipe.SourceUrl)
		}
		if url == "" {
			continue
		}
		existing, lookupErr := s.recipeService.ByUrl(url, uuid.Nil)
		if lookupErr == nil {
			// Public recipe exists — backfill FeedID if it was imported without one
			if existing.FeedID == nil {
				if updateErr := s.recipeService.SetFeedID(existing.ID, feed.ID); updateErr != nil {
					log.Warnw("failed to link existing recipe to feed", "url", url, "error", updateErr.Error())
				}
			}
			continue
		}

		recipe.FeedID = &feed.ID
		if _, err := s.recipeIngest.ImportRecipe(ctx, recipe); err != nil {
			log.Warnw("failed to import recipe", "url", url, "error", err.Error())
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
		log.Warnw("failed to persist feed metadata", "feed", feed.ID, "error", err.Error())
	}

	return len(recipes), imported, nil
}
