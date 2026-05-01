package services

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type importService struct {
	recipeService  domain.RecipeService
	recipeIngest   domain.RecipeIngestService
	feedService    domain.FeedService
	scraperService domain.ScraperService
}

func NewImportService(recipeService domain.RecipeService, recipeIngest domain.RecipeIngestService, feedService domain.FeedService, scraperService domain.ScraperService) domain.ImportService {
	return &importService{
		recipeService:  recipeService,
		recipeIngest:   recipeIngest,
		feedService:    feedService,
		scraperService: scraperService,
	}
}

// ImportFromURL scrapes the given URL and imports it as a recipe.
// Returns an error if the URL points to a feed rather than a single recipe.
func (s *importService) ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	url = utils.NormalizeURL(url)
	if !forceUpdate {
		existing, err := s.recipeService.ByUrl(url, householdID)
		if err != nil && !errors.Is(err, sentinels.ErrNotFound) && !errors.Is(err, sentinels.ErrForbidden) {
			return nil, err
		}
		if existing != nil {
			if err := s.recipeService.UserSave(existing.ID, userID, householdID); err != nil {
				return nil, err
			}
			return existing, nil
		}
	}

	scraped, err := s.scraperService.ScrapeRecipe(ctx, url)
	if err != nil {
		return nil, err
	}

	recipe, err := s.recipeIngest.ImportRecipe(ctx, scraped)
	if err != nil {
		return nil, err
	}
	if err := s.recipeService.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return recipe, nil
}

// DetectAndImport scrapes the given URL and imports it as either a recipe or a feed subscription.
func (s *importService) DetectAndImport(ctx context.Context, url string, requestedType string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.ImportResult, error) {
	url = utils.NormalizeURL(url)

	if !forceUpdate {
		// Check recipe cache before paying the cost of a network scrape.
		existing, err := s.recipeService.ByUrl(url, householdID)
		if err != nil && !errors.Is(err, sentinels.ErrNotFound) && !errors.Is(err, sentinels.ErrForbidden) {
			return nil, err
		}
		if existing != nil {
			if err := s.recipeService.UserSave(existing.ID, userID, householdID); err != nil {
				return nil, err
			}
			return &domain.ImportResult{Recipe: existing}, nil
		}
	}

	// Unknown URL — must scrape to determine whether it's a recipe or a feed.
	scraped, err := s.scraperService.ScrapeUrl(ctx, url, requestedType)
	if err != nil {
		return nil, err
	}

	if scraped.Type == domain.PageTypeFeed {
		feed, err := s.feedService.Subscribe(ctx, householdID, url, scraped.Feed)
		if err != nil {
			return nil, err
		}
		return &domain.ImportResult{Created: true, Feed: feed}, nil
	}

	recipe, err := s.recipeIngest.ImportRecipe(ctx, scraped.Recipe)
	if err != nil {
		return nil, err
	}
	if err := s.recipeService.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return &domain.ImportResult{Created: true, Recipe: recipe}, nil
}
