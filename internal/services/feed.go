package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

type FeedService struct {
	repo          domain.FeedRepository
	publisherRepo domain.PublisherRepository
	recipeRepo    domain.RecipeRepository
	recipeService domain.RecipeService
}

func NewFeedService(repo domain.FeedRepository, pubRepo domain.PublisherRepository, recipeRepo domain.RecipeRepository, service domain.RecipeService) domain.FeedService {
	return &FeedService{repo: repo, publisherRepo: pubRepo, recipeRepo: recipeRepo, recipeService: service}
}

func (s *FeedService) List(householdID uuid.UUID, offset, limit int) ([]domain.Feed, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *FeedService) Stream(householdID uuid.UUID, offset, limit int) ([]domain.Recipe, int64, error) {
	recipes, total, err := s.repo.Stream(householdID, offset, limit)
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
		log.Warnf("stream override lookup failed: %v", err)
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
	if err != nil && !errors.Is(err, domain.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, domain.ErrRecordNotFound) {
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
	scrapedFeed, err := krip.ScrapeFeedUrl(url, model.FeedOptions{Quick: true})
	if err != nil {
		return nil, fmt.Errorf("invalid feed url: %w", err)
	}

	feed := &domain.Feed{Url: url}
	if len(scrapedFeed.Entries) > 0 && scrapedFeed.Entries[0].Publisher != nil {
		pub := domain.FromKripPublisher(scrapedFeed.Entries[0].Publisher)
		if err := s.publisherRepo.FindOrCreate(pub); err != nil {
			log.Warnf("error creating publisher %v: %s", pub, err.Error())
		} else {
			feed.PublisherID = pub.ID
		}
	} else {
		pub := &domain.Publisher{Url: url, Name: url}
		if err := s.publisherRepo.FindOrCreate(pub); err != nil {
			log.Warnf("error creating publisher %v: %s", pub, err.Error())
		} else {
			feed.PublisherID = pub.ID
		}
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

	log.Infof("Checking %d feeds for updates...", len(feeds))
	for _, feed := range feeds {
		s.processFeed(&feed)
	}
	return nil
}

func (s *FeedService) processFeed(feed *domain.Feed) {
	scrapedFeed, err := krip.ScrapeFeedUrl(feed.Url, model.FeedOptions{
		MinIngredients:      3,
		RequireImage:        true,
		RequireInstructions: true,
	})

	if err != nil {
		log.Warnf("Failed to scrape feed %s: %v", feed.Url, err)
		feed.ErrorCount++
		if feed.ErrorCount > 10 {
			feed.Active = false
		}
		if err := s.repo.Update(feed); err != nil {
			log.Warnf("failed to persist error state for feed %s: %v", feed.Url, err)
		}
		return
	}

	feed.ErrorCount = 0
	feed.Retrieved = time.Now()
	if err := s.repo.Update(feed); err != nil {
		log.Warnf("failed to persist feed metadata for %s: %v", feed.Url, err)
	}

	for _, kripRecipe := range scrapedFeed.Entries {
		if kripRecipe.Url == "" {
			continue
		}

		if _, err := s.recipeRepo.ByUrl(kripRecipe.Url); err == nil {
			continue
		}

		if _, err := s.recipeService.ImportFromKripRecipe(kripRecipe, &feed.ID); err != nil {
			log.Warnf("Failed to import recipe from %s: %v", kripRecipe.Url, err)
		} else {
			log.Infof("Imported new recipe: %s", kripRecipe.Name)
		}
	}
}
