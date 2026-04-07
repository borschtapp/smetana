package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type importService struct {
	recipeService    domain.RecipeService
	imageService     domain.ImageService
	publisherService domain.PublisherService
	authorService    domain.AuthorService
	foodService      domain.FoodService
	unitService      domain.UnitService
	taxonomyService  domain.TaxonomyService
	equipmentService domain.EquipmentService
	scraperService   domain.ScraperService
	fetchConcurrency int
}

func NewImportService(recipeService domain.RecipeService, imageService domain.ImageService, publisherService domain.PublisherService, authorService domain.AuthorService, foodService domain.FoodService, unitService domain.UnitService, taxonomyService domain.TaxonomyService, equipmentService domain.EquipmentService, scraperService domain.ScraperService) domain.ImportService {
	return &importService{
		recipeService:    recipeService,
		imageService:     imageService,
		publisherService: publisherService,
		authorService:    authorService,
		foodService:      foodService,
		unitService:      unitService,
		taxonomyService:  taxonomyService,
		equipmentService: equipmentService,
		scraperService:   scraperService,
		fetchConcurrency: utils.GetenvInt("FETCH_CONCURRENCY", 5),
	}
}

// ImportFromURL imports a recipe from URL and saves it for the given user.
func (s *importService) ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	existing, err := s.recipeService.ByUrl(url, householdID)
	if err != nil && !errors.Is(err, sentinels.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		if err := s.recipeService.UserSave(existing.ID, userID, householdID); err != nil {
			return nil, err
		}
		return existing, nil
	}

	scraped, err := s.scraperService.ScrapeRecipe(ctx, url)
	if err != nil {
		return nil, err
	}
	recipe, err := s.ImportRecipe(ctx, scraped)
	if err != nil {
		return nil, err
	}
	if err := s.recipeService.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *importService) ImportRecipe(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
	for _, ing := range recipe.Ingredients {
		if ing.Food != nil {
			if err := s.foodService.FindOrCreate(ctx, ing.Food); err == nil {
				ing.FoodID = &ing.Food.ID
				for _, taxonomy := range ing.Food.Taxonomies {
					if err := s.taxonomyService.FindOrCreate(taxonomy); err == nil {
						_ = s.foodService.AddTaxonomy(ing.Food.ID, taxonomy)
					} else {
						log.Warnw("error creating food taxonomy", "taxonomy", taxonomy, "error", err.Error())
					}
				}
			} else {
				log.Warnw("error creating food", "food", ing.Food, "error", err.Error())
				ing.Food = nil
			}
		}
		if ing.Unit != nil {
			if err := s.unitService.FindOrCreate(ing.Unit); err == nil {
				ing.UnitID = &ing.Unit.ID
			} else {
				log.Warnw("error creating unit", "unit", ing.Unit, "error", err.Error())
				ing.Unit = nil
			}
		}
	}

	resolved := recipe.Taxonomies[:0]
	for _, taxonomy := range recipe.Taxonomies {
		if err := s.taxonomyService.FindOrCreate(taxonomy); err == nil {
			resolved = append(resolved, taxonomy)
		} else {
			log.Warnw("error creating taxonomy", "taxonomy", taxonomy, "error", err.Error())
		}
	}
	recipe.Taxonomies = resolved

	resolvedEquipment := recipe.Equipment[:0]
	for _, equipment := range recipe.Equipment {
		if err := s.equipmentService.FindOrCreate(ctx, equipment); err == nil {
			resolvedEquipment = append(resolvedEquipment, equipment)
		} else {
			log.Warnw("error creating equipment", "equipment", equipment, "error", err.Error())
		}
	}
	recipe.Equipment = resolvedEquipment

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreate(ctx, recipe.Publisher); err != nil {
			log.Warnw("error creating publisher", "publisher", recipe.Publisher, "error", err.Error())
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	if recipe.Author != nil {
		if err := s.authorService.FindOrCreate(ctx, recipe.Author); err != nil {
			log.Warnw("error creating recipe author", "author", recipe.Author, "error", err.Error())
		} else {
			recipe.AuthorID = &recipe.Author.ID
		}
	}

	if err := s.recipeService.Import(recipe); err != nil {
		return nil, err
	}

	s.processRecipeImages(ctx, recipe)
	s.processInstructionImages(ctx, recipe)
	return recipe, nil
}

// selectBestImage picks the most suitable thumbnail from a set of downloaded images.
// Preference: largest image whose longest side is ≤ 1000 px (avoids oversized originals).
func selectBestImage(images []*domain.Image) *domain.Image {
	const maxPreferredDim = 1000

	var (
		best        *domain.Image
		bestDim     int
		fallback    *domain.Image
		fallbackDim int
	)

	for _, image := range images {
		if image == nil || image.Path == nil || image.Width == nil || image.Height == nil {
			continue
		}
		dim := max(*image.Width, *image.Height)
		if dim <= maxPreferredDim && dim > bestDim {
			best = image
			bestDim = dim
		}
		if dim > fallbackDim {
			fallback = image
			fallbackDim = dim
		}
	}

	if best != nil {
		return best
	}
	return fallback
}

func (s *importService) processRecipeImages(ctx context.Context, recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	var (
		mu      sync.Mutex
		results []*domain.Image
	)

	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, image := range recipe.Images {
		if image.SourceURL == "" {
			continue
		}
		g.Go(func() error {
			image.EntityType = "recipes"
			image.EntityID = recipe.ID

			if err := s.imageService.PersistRemote(ctx, image, ""); err != nil {
				return fmt.Errorf("failed to download recipe image %s: %w", image.SourceURL, err)
			}

			mu.Lock()
			results = append(results, image)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("recipe image processing completed with errors", "error", err.Error())
	}

	best := selectBestImage(results)
	if best == nil {
		return
	}

	if err := s.imageService.SetDefault(best); err != nil {
		log.Warnw("failed to set default recipe image", "error", err.Error())
		return
	}

	recipe.ImagePath = best.Path
}

func (s *importService) processInstructionImages(ctx context.Context, recipe *domain.Recipe) {
	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, instruction := range recipe.Instructions {
		ins := instruction // capture for closure
		if ins == nil || ins.ImagePath != nil || len(ins.Images) < 1 {
			continue // already has image, or has no new images
		}
		g.Go(func() error {
			// Group instruction images under recipes/{recipeID}/ alongside recipe images.
			path, err := s.imageService.PersistRemoteAsDefault(ctx, ins.Images[0], "recipe_instructions", ins.ID, "recipes/"+recipe.ID.String())
			if err != nil {
				return fmt.Errorf("failed to process instruction image %v: %w", ins.Images[0].SourceURL, err)
			}
			ins.ImagePath = path
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("instruction image processing completed with errors", "error", err.Error())
	}
}
