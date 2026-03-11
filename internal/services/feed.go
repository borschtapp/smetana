package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type FeedService struct {
	repo           domain.FeedRepository
	publisherRepo  domain.PublisherRepository
	recipeRepo     domain.RecipeRepository
	recipeService  domain.RecipeService
	scraperService domain.ScraperService
}

func NewFeedService(repo domain.FeedRepository, pubRepo domain.PublisherRepository, recipeRepo domain.RecipeRepository, recipeService domain.RecipeService, scraperService domain.ScraperService) domain.FeedService {
	return &FeedService{
		repo:           repo,
		publisherRepo:  pubRepo,
		recipeRepo:     recipeRepo,
		recipeService:  recipeService,
		scraperService: scraperService,
	}
}

func (s *FeedService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Feed, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *FeedService) Stream(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	recipes, total, err := s.recipeRepo.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts, FromFeeds: true})
	if err != nil || len(recipes) == 0 {
		return recipes, total, err
	}

	// Copy-on-write override: replace global recipes with household copies where they exist.
	parentIDs := make([]uuid.UUID, len(recipes))
	for i, r := range recipes {
		parentIDs[i] = r.ID
	}

	overrides, err := s.recipeRepo.ByParentIDsAndHousehold(parentIDs, householdID)
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

func (s *FeedService) Subscribe(ctx context.Context, householdID uuid.UUID, url string) (*domain.Feed, error) {
	feed, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, sentinels.ErrRecordNotFound) {
		feed, err = s.createFeed(url)
		if err != nil {
			log.Warnw("subscribe scrape feed failed", "url", url, "error", err)
			return nil, sentinels.Unprocessable("can't scrape feed url")
		}

		// Submit background task to fetch recipes
		go func() {
			if found, imported, err := s.FetchFeed(context.WithoutCancel(ctx), feed); err != nil {
				log.Errorw("background feed fetch failed", "url", url, "error", err)
			} else if found == 0 {
				log.Warnw("background feed fetch found no recipes", "url", url)

				feed.Active = false
				if updateErr := s.repo.Update(feed); updateErr != nil {
					log.Warnw("failed to deactivate feed with no recipes", "url", url, "error", updateErr)
				}
			} else {
				log.Infow("background feed fetched successfully", "url", url, "found", found, "imported", imported)
			}
		}()
	}

	if err := s.repo.AddFeed(householdID, feed); err != nil {
		return nil, err
	}

	if len(feed.Recipes) == 0 {
		return nil, sentinels.Unprocessable("feed has no importable recipes")
	}
	return feed, nil
}

func (s *FeedService) Unsubscribe(householdID uuid.UUID, feedID uuid.UUID) error {
	return s.repo.DeleteFeed(householdID, feedID)
}

func (s *FeedService) createFeed(url string) (*domain.Feed, error) {
	recipes, err := s.scraperService.ScrapeFeed(url, domain.FeedScrapeOptions{Quick: true})
	if err != nil {
		return nil, fmt.Errorf("invalid feed url: %w", err)
	}

	feed := &domain.Feed{Url: url}
	var pub *domain.Publisher
	if len(recipes) > 0 && recipes[0].Publisher != nil {
		pub = recipes[0].Publisher
	} else {
		pub = &domain.Publisher{Url: url, Name: url}
	}

	if err := s.publisherRepo.FindOrCreate(pub); err != nil {
		log.Warnw("error creating publisher", "publisher", pub, "error", err)
	} else {
		feed.PublisherID = pub.ID
	}

	if err := s.repo.Create(feed); err != nil {
		return nil, err
	}

	feed.Recipes = recipes
	return feed, nil
}

func (s *FeedService) FetchFeed(ctx context.Context, feed *domain.Feed) (int, int, error) {
	imported := 0
	feed.LastSyncAt = time.Now()

	recipes, err := s.scraperService.ScrapeFeed(feed.Url, domain.FeedScrapeOptions{
		MinIngredients:      3,
		RequireImage:        true,
		RequireInstructions: true,
	})

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
		if recipe.IsBasedOn != nil {
			url = *recipe.IsBasedOn
		}
		if url == "" {
			continue
		}
		if _, err := s.recipeRepo.ByUrl(url); err == nil {
			continue
		}

		recipe.FeedID = &feed.ID
		if _, err := s.recipeService.ImportRecipe(ctx, recipe); err != nil {
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
