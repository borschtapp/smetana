package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type importService struct {
	recipeService  domain.RecipeService
	recipeIngest   domain.RecipeIngestService
	feedService    domain.FeedService
	scraperService domain.ScraperService
	refreshGroup   singleflight.Group
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
			return nil, fmt.Errorf("from url (lookup cache): %w", err)
		}
		if existing != nil {
			if err := s.recipeService.UserSave(existing.ID, userID, householdID); err != nil {
				return nil, fmt.Errorf("from url (save existing): %w", err)
			}
			return existing, nil
		}
	}

	scraped, err := s.scraperService.ScrapeRecipe(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("from url (scrape): %w", err)
	}

	recipe, err := s.recipeIngest.ImportRecipe(ctx, scraped)
	if err != nil {
		return nil, fmt.Errorf("from url (ingest): %w", err)
	}
	if err := s.recipeService.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, fmt.Errorf("from url (save new): %w", err)
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
			return nil, fmt.Errorf("detect (lookup cache): %w", err)
		}
		if existing != nil {
			if err := s.recipeService.UserSave(existing.ID, userID, householdID); err != nil {
				return nil, fmt.Errorf("detect (save existing): %w", err)
			}
			return &domain.ImportResult{Recipe: existing}, nil
		}
	}

	// Unknown URL — must scrape to determine whether it's a recipe or a feed.
	scraped, err := s.scraperService.ScrapeUrl(ctx, url, requestedType)
	if err != nil {
		return nil, fmt.Errorf("detect (scrape): %w", err)
	}

	if scraped.Type == domain.PageTypeFeed {
		feed, err := s.feedService.Subscribe(ctx, householdID, url, scraped.Feed)
		if err != nil {
			return nil, fmt.Errorf("detect (subscribe): %w", err)
		}
		return &domain.ImportResult{Created: true, Feed: feed}, nil
	}

	recipe, err := s.recipeIngest.ImportRecipe(ctx, scraped.Recipe)
	if err != nil {
		return nil, fmt.Errorf("detect (ingest): %w", err)
	}
	if err := s.recipeService.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, fmt.Errorf("detect (save new): %w", err)
	}
	return &domain.ImportResult{Created: true, Recipe: recipe}, nil
}

func (s *importService) RefreshRecipe(ctx context.Context, id uuid.UUID, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	// Lookup and reload depend on the caller, so only scrape and ingest are shared.
	recipe, err := s.recipeService.ByID(id, householdID)
	if err != nil {
		return nil, fmt.Errorf("refresh (lookup): %w", err)
	}

	if recipe.HouseholdID != nil {
		return nil, sentinels.BadRequest("cannot refresh a customized recipe")
	}

	if recipe.SourceUrl == nil || *recipe.SourceUrl == "" {
		return nil, sentinels.BadRequest("recipe has no source url")
	}

	v, err, _ := s.refreshGroup.Do(id.String(), func() (any, error) {
		// Detached, so one caller cancelling doesn't fail the others waiting on it.
		return s.refreshFromSource(context.WithoutCancel(ctx), recipe)
	})
	if err != nil {
		return nil, err
	}

	reloaded, err := s.recipeService.ByIDPreload(id, userID, householdID, types.Preload("all"))
	if err != nil {
		// The refresh is committed; return the unpreloaded recipe instead of failing.
		log.Warnw("refresh reload failed", "recipeID", id, "error", err.Error())
		return v.(*domain.Recipe), nil
	}
	return reloaded, nil
}

// refreshFromSource re-scrapes the recipe and overwrites the stored global row.
func (s *importService) refreshFromSource(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
	scraped, err := s.scraperService.ScrapeRecipe(ctx, *recipe.SourceUrl)
	if err != nil {
		return nil, fmt.Errorf("refresh (scrape): %w", err)
	}

	// Keep the stored ID and URL; the page may now report a different canonical URL.
	scraped.ID = recipe.ID
	scraped.SourceUrl = recipe.SourceUrl

	imported, err := s.recipeIngest.ImportRecipe(ctx, scraped)
	if err != nil {
		return nil, fmt.Errorf("refresh (ingest): %w", err)
	}
	return imported, nil
}
