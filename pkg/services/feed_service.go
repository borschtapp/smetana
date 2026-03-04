package services

import (
	"errors"
	"fmt"
	"time"

	"borscht.app/smetana/domain"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type FeedService struct {
	repo          domain.FeedRepository
	recipeRepo    domain.RecipeRepository
	recipeService domain.RecipeService
}

func NewFeedService(repo domain.FeedRepository, recipeRepo domain.RecipeRepository, service domain.RecipeService) *FeedService {
	return &FeedService{repo: repo, recipeRepo: recipeRepo, recipeService: service}
}

func (s *FeedService) List(userID uuid.UUID, offset, limit int) ([]domain.Feed, int64, error) {
	return s.repo.List(userID, offset, limit)
}

func (s *FeedService) Stream(userID uuid.UUID, offset, limit int) ([]domain.Recipe, int64, error) {
	return s.repo.Stream(userID, offset, limit)
}

func (s *FeedService) Subscribe(userID uuid.UUID, url string) (*domain.Feed, error) {
	feed, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, domain.ErrRecordNotFound) {
		return nil, err
	}

	if errors.Is(err, domain.ErrRecordNotFound) {
		// New URL — verify and scrape metadata before persisting
		scrapedFeed, err := krip.ScrapeFeedUrl(url, model.FeedOptions{Quick: true})
		if err != nil {
			return nil, fmt.Errorf("invalid feed url: %w", err)
		}

		feed = &domain.Feed{Url: url}
		if len(scrapedFeed.Entries) > 0 && scrapedFeed.Entries[0].Publisher != nil {
			pub := scrapedFeed.Entries[0].Publisher
			feed.Name = pub.Name
			feed.WebsiteUrl = pub.Url
			feed.Description = pub.Description
		}

		if err := s.repo.Create(feed); err != nil {
			return nil, err
		}
	}

	if err := s.repo.AddFeed(userID, feed); err != nil {
		return nil, err
	}
	return feed, nil
}

func (s *FeedService) Unsubscribe(userID uuid.UUID, feedID uuid.UUID) error {
	return s.repo.DeleteFeed(userID, feedID)
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
		s.repo.Update(feed)
		return
	}

	feed.ErrorCount = 0
	feed.LastFetchedAt = time.Now()
	s.repo.Update(feed)

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
