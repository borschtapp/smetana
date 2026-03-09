package services

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type FeedService struct {
	repo             domain.FeedRepository
	publisherRepo    domain.PublisherRepository
	recipeRepo       domain.RecipeRepository
	recipeService    domain.RecipeService
	scraperService   domain.ScraperService
	fetchConcurrency int
}

func NewFeedService(repo domain.FeedRepository, pubRepo domain.PublisherRepository, recipeRepo domain.RecipeRepository, recipeService domain.RecipeService, scraperService domain.ScraperService) domain.FeedService {
	return &FeedService{
		repo:             repo,
		publisherRepo:    pubRepo,
		recipeRepo:       recipeRepo,
		recipeService:    recipeService,
		scraperService:   scraperService,
		fetchConcurrency: utils.GetenvInt("FETCH_CONCURRENCY", 5),
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
		log.Warn("stream override lookup failed", err)
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

func (s *FeedService) Subscribe(householdID uuid.UUID, url string) (*domain.Feed, error) {
	feed, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, sentinels.ErrRecordNotFound) {
		feed, err = s.createFeed(url)
		if err != nil {
			return nil, err
		}
	}

	if err := s.repo.AddFeed(householdID, feed); err != nil {
		return nil, err
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
		log.Warn("error creating publisher", "publisher", pub, "error", err)
	} else {
		feed.PublisherID = pub.ID
	}

	if err := s.repo.Create(feed); err != nil {
		return nil, err
	}

	return feed, nil
}

func (s *FeedService) FetchUpdates() error {
	var feeds, err = s.repo.ListActive()
	if err != nil {
		return err
	}

	log.Info("checking feeds for updates", "count", len(feeds))

	var wg sync.WaitGroup
	sem := make(chan struct{}, s.fetchConcurrency)

	for i := range feeds {
		wg.Add(1)
		go func(feed *domain.Feed) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			s.processFeed(feed)
		}(&feeds[i])
	}

	wg.Wait()
	return nil
}

func (s *FeedService) processFeed(feed *domain.Feed) {
	recipes, err := s.scraperService.ScrapeFeed(feed.Url, domain.FeedScrapeOptions{
		MinIngredients:      3,
		RequireImage:        true,
		RequireInstructions: true,
	})

	if err != nil {
		log.Warn("failed to scrape feed", "url", feed.Url, "error", err)
		feed.ErrorCount++
		if feed.ErrorCount > 10 {
			feed.Active = false
			log.Error("deactivating feed due to repeated errors", "url", feed.Url)
		}
		if err := s.repo.Update(feed); err != nil {
			log.Warn("failed to persist error state for feed", "url", feed.Url, "error", err)
		}
		return
	}

	feed.ErrorCount = 0
	feed.Retrieved = time.Now()
	if err := s.repo.Update(feed); err != nil {
		log.Warn("failed to persist feed metadata", "url", feed.Url, "error", err)
	}

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
		if _, err := s.recipeService.ImportRecipe(recipe); err != nil {
			log.Warn("failed to import recipe", "url", url, "error", err)
		} else {
			name := ""
			if recipe.Name != nil {
				name = *recipe.Name
			}
			log.Info("imported new recipe", "name", name)
		}
	}
}
