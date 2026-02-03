package services

import (
	"errors"
	"fmt"
	"time"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"

	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FeedService struct {
	recipeService *RecipeService
}

func NewFeedService(recipeService *RecipeService) *FeedService {
	return &FeedService{
		recipeService: recipeService,
	}
}

func (s *FeedService) Subscribe(userID uuid.UUID, url string) (*domain.Feed, error) {
	// Check if feed already exists
	var feed domain.Feed
	err := database.DB.Where("url = ?", url).First(&feed).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// Verify feed if it uses a new URL (quick mode - don't scrape entries yet)
		scrapedFeed, err := krip.ScrapeFeedUrl(url, model.FeedOptions{Quick: true})
		if err != nil {
			return nil, fmt.Errorf("invalid feed url: %w", err)
		}

		feed = domain.Feed{
			Url:    url,
			Active: true,
		}

		// Extract feed info from first entry if available
		if len(scrapedFeed.Entries) > 0 && scrapedFeed.Entries[0].Publisher != nil {
			pub := scrapedFeed.Entries[0].Publisher
			feed.Name = pub.Name
			feed.WebsiteUrl = pub.Url
			feed.Description = pub.Description
		}

		if err := database.DB.Create(&feed).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	if err := database.DB.Model(&domain.User{ID: userID}).Association("Feeds").Append(&feed); err != nil {
		return nil, err
	}

	return &feed, nil
}

func (s *FeedService) Unsubscribe(userID uuid.UUID, feedID uuid.UUID) error {
	return database.DB.Model(&domain.User{ID: userID}).Association("Feeds").Delete(&domain.Feed{ID: feedID})
}

func (s *FeedService) ListSubscriptions(userID uuid.UUID, offset, limit int) ([]domain.Feed, int64, error) {
	var feeds []domain.Feed
	baseQuery := database.DB.
		Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
		Where("feed_subscriptions.user_id = ?", userID)

	var total int64
	if err := baseQuery.Model(&domain.Feed{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := baseQuery.
		Offset(offset).
		Limit(limit).
		Find(&feeds).Error
	return feeds, total, err
}

func (s *FeedService) GetStream(userID uuid.UUID, page, limit int) ([]domain.Recipe, int64, error) {
	var recipes []domain.Recipe
	offset := (page - 1) * limit

	baseQuery := database.DB.
		Joins("JOIN feeds ON feeds.id = recipes.feed_id").
		Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
		Where("feed_subscriptions.user_id = ?", userID)

	var total int64
	if err := baseQuery.Model(&domain.Recipe{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := baseQuery.
		Preload("Images").
		Order("recipes.created DESC").
		Limit(limit).
		Offset(offset).
		Find(&recipes).Error

	return recipes, total, err
}

func (s *FeedService) FetchUpdates() {
	var feeds []domain.Feed
	// Fetch active feeds, maybe filter by LastFetchedAt to avoid spamming
	if err := database.DB.Where("active = ?", true).Find(&feeds).Error; err != nil {
		log.Errorf("Failed to fetch feeds: %v", err)
		return
	}

	log.Infof("Checking %d feeds for updates...", len(feeds))
	for _, feed := range feeds {
		// Should run in goroutines for scale, but sequential is fine for MVP
		s.processFeed(&feed)
	}
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
		// If too many errors, maybe deactivate?
		if feed.ErrorCount > 10 {
			feed.Active = false
		}
		database.DB.Save(feed)
		return
	}

	// Reset error count if successful
	feed.ErrorCount = 0
	feed.LastFetchedAt = time.Now()
	database.DB.Save(feed)

	for _, kripRecipe := range scrapedFeed.Entries {
		if kripRecipe.Url == "" {
			continue
		}

		// Check if recipe exists
		var existingCount int64
		database.DB.Model(&domain.Recipe{}).Where("is_based_on = ?", kripRecipe.Url).Count(&existingCount)
		if existingCount > 0 {
			continue
		}

		// Import recipe directly from scraped data
		if _, err := s.recipeService.ImportFromKripRecipe(kripRecipe, &feed.ID); err != nil {
			log.Warnf("Failed to import recipe from %s: %v", kripRecipe.Url, err)
		} else {
			log.Infof("Imported new recipe: %s", kripRecipe.Name)
		}
	}
}
