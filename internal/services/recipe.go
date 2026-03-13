package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type RecipeService struct {
	repo             domain.RecipeRepository
	userRepo         domain.UserRepository
	imageService     domain.ImageService
	publisherService domain.PublisherService
	foodRepo         domain.FoodRepository
	unitRepo         domain.UnitRepository
	taxonomyRepo     domain.TaxonomyRepository
	scraperService   domain.ScraperService
	fetchConcurrency int
}

func NewRecipeService(repo domain.RecipeRepository, userRepo domain.UserRepository, imageService domain.ImageService, publisherService domain.PublisherService, foodRepo domain.FoodRepository, unitRepo domain.UnitRepository, taxonomyRepo domain.TaxonomyRepository, scraperService domain.ScraperService) domain.RecipeService {
	return &RecipeService{
		repo:             repo,
		userRepo:         userRepo,
		imageService:     imageService,
		publisherService: publisherService,
		foodRepo:         foodRepo,
		unitRepo:         unitRepo,
		taxonomyRepo:     taxonomyRepo,
		scraperService:   scraperService,
		fetchConcurrency: utils.GetenvInt("FETCH_CONCURRENCY", 5),
	}
}

func (s *RecipeService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	recipe, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	// Anonymous are publicly readable
	if recipe.HouseholdID == nil {
		return recipe, nil
	}
	// Household-owned: same household has full access
	if *recipe.HouseholdID == householdID {
		return recipe, nil
	}

	return nil, sentinels.ErrForbidden
}

func (s *RecipeService) Search(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	return s.repo.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts})
}

func (s *RecipeService) Create(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	recipe.HouseholdID = &householdID
	recipe.UserID = &userID
	if err := s.repo.Create(recipe); err != nil {
		return err
	}
	return s.UserSave(recipe.ID, userID, householdID)
}

func (s *RecipeService) Update(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(recipe.ID)
	if err != nil {
		return err
	}

	// Copy-on-write: if it doesn't belong to household yet, clone it into the household first
	if existing.HouseholdID == nil {
		cloned, err := s.cloneToHousehold(existing, userID, householdID)
		if err != nil {
			return err
		}

		// Apply the incoming patch fields onto the clone
		recipe.ID = cloned.ID
		recipe.HouseholdID = cloned.HouseholdID
		recipe.ParentID = cloned.ParentID
		recipe.UserID = cloned.UserID
	}
	return s.repo.Update(recipe)
}

// cloneToHousehold clones a global recipe into the given household. Make sure to preload all relevant associations
func (s *RecipeService) cloneToHousehold(global *domain.Recipe, userID, householdID uuid.UUID) (*domain.Recipe, error) {
	clone := *global
	clone.ID = uuid.Nil // BeforeCreate hook will assign a new UUID
	clone.HouseholdID = &householdID
	clone.UserID = &userID
	clone.ParentID = &global.ID

	clone.Images = nil // images are shared (same remote URLs) — don't duplicate storage
	clone.Collections = nil
	clone.Taxonomies = global.Taxonomies // keep taxonomy associations (many2many OK to share)
	clone.Publisher = global.Publisher
	clone.Feed = nil

	clone.Ingredients = make([]*domain.RecipeIngredient, len(global.Ingredients))
	for i, ing := range global.Ingredients {
		copy_ := *ing
		copy_.ID = uuid.Nil
		copy_.RecipeID = uuid.Nil // will be set by GORM after Create
		copy_.Recipe = nil
		copy_.Food = ing.Food
		copy_.Unit = ing.Unit
		clone.Ingredients[i] = &copy_
	}
	clone.Instructions = make([]*domain.RecipeInstruction, len(global.Instructions))
	for i, ins := range global.Instructions {
		copy_ := *ins
		copy_.ID = uuid.Nil
		copy_.RecipeID = uuid.Nil
		copy_.Recipe = nil
		copy_.ParentID = nil // instruction parent is within same recipe
		copy_.Parent = nil
		clone.Instructions[i] = &copy_
	}

	var newRecipe *domain.Recipe
	err := s.repo.Transaction(func(txRepo domain.RecipeRepository) error {
		if err := txRepo.Create(&clone); err != nil {
			return err
		}
		// Migrate household pointers: RecipeSaved, MealPlan, CollectionRecipes
		if err := txRepo.ReplaceRecipePointers(global.ID, clone.ID, householdID); err != nil {
			return err
		}
		newRecipe = &clone
		return nil
	})

	if err != nil {
		return nil, err
	}
	return newRecipe, nil
}

func (s *RecipeService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if existing.HouseholdID == nil || *existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	if existing.ParentID == nil {
		s.deleteImages(existing.Images)
	}

	return s.repo.Delete(id)
}

func (s *RecipeService) deleteImages(images []*domain.RecipeImage) {
	for _, img := range images {
		if img.DownloadUrl != nil {
			if err := s.imageService.DeleteImage(*img.DownloadUrl); err != nil {
				log.Warnw("failed to delete image file", "path", *img.DownloadUrl, "error", err)
			}
		}
	}
}

func (s *RecipeService) UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error {
	return s.repo.UserSave(recipeID, userID, householdID)
}

func (s *RecipeService) UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error {
	return s.repo.UserUnsave(recipeID, userID)
}

func (s *RecipeService) CreateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.CreateIngredient(ingredient)
}

func (s *RecipeService) UpdateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.UpdateIngredient(ingredient)
}

func (s *RecipeService) DeleteIngredient(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.DeleteIngredient(id, recipeID)
}

func (s *RecipeService) CreateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.CreateInstruction(instruction)
}

func (s *RecipeService) UpdateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.UpdateInstruction(instruction)
}

func (s *RecipeService) DeleteInstruction(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.DeleteInstruction(id, recipeID)
}

// ImportFromURL imports a recipe from URL and saves it for the given user.
func (s *RecipeService) ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	existing, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		if err := s.UserSave(existing.ID, userID, householdID); err != nil {
			return nil, err
		}
		return existing, nil
	}

	scraped, err := s.scraperService.ScrapeRecipe(url)
	if err != nil {
		return nil, err
	}
	recipe, err := s.ImportRecipe(ctx, scraped)
	if err != nil {
		return nil, err
	}
	if err := s.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *RecipeService) ImportRecipe(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
	for _, ing := range recipe.Ingredients {
		if ing.Food != nil {
			if err := s.foodRepo.FindOrCreate(ing.Food); err == nil {
				ing.FoodID = &ing.Food.ID
				for _, tax := range ing.Food.Taxonomies {
					if err := s.taxonomyRepo.FindOrCreate(tax); err == nil {
						_ = s.foodRepo.AddTaxonomy(ing.Food.ID, tax)
					} else {
						log.Warnw("error creating food taxonomy", "taxonomy", tax, "error", err)
					}
				}
			} else {
				log.Warnw("error creating food", "food", ing.Food, "error", err)
				ing.Food = nil
			}
		}
		if ing.Unit != nil {
			if err := s.unitRepo.FindOrCreate(ing.Unit); err == nil {
				ing.UnitID = &ing.Unit.ID
			} else {
				log.Warnw("error creating unit", "unit", ing.Unit, "error", err)
				ing.Unit = nil
			}
		}
	}

	resolved := recipe.Taxonomies[:0]
	for _, taxonomy := range recipe.Taxonomies {
		if err := s.taxonomyRepo.FindOrCreate(taxonomy); err == nil {
			resolved = append(resolved, taxonomy)
		} else {
			log.Warnw("error creating taxonomy", "taxonomy", taxonomy, "error", err)
		}
	}
	recipe.Taxonomies = resolved

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreate(ctx, recipe.Publisher); err != nil {
			log.Warnw("error creating publisher", "publisher", recipe.Publisher, "error", err)
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	if err := s.repo.Import(recipe); err != nil {
		return nil, err
	}

	s.processRecipeImages(ctx, recipe)
	s.processInstructionImages(ctx, recipe)
	return recipe, nil
}

func (s *RecipeService) processRecipeImages(ctx context.Context, recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	for _, image := range recipe.Images {
		image.RecipeID = recipe.ID
	}

	if err := s.repo.CreateImages(recipe.Images); err != nil {
		log.Warnw("failed to save images", "error", err)
		return
	}

	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, image := range recipe.Images {
		g.Go(func() error {
			basePath := "recipe/" + image.RecipeID.String() + "/" + image.ID.String()
			if info, err := s.imageService.DownloadAndSaveImage(ctx, image.RemoteUrl, basePath); err != nil {
				return fmt.Errorf("failed to download image from url %s: %w", image.RemoteUrl, err)
			} else {
				image.DownloadUrl = &info.Path
				image.Width = info.Width
				image.Height = info.Height
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("image processing completed with errors", "error", err)
	}

	for _, img := range recipe.Images {
		if img.DownloadUrl != nil {
			err := s.repo.UpdateImage(img)
			if err != nil {
				log.Warnw("failed to update image", "image_id", img.ID, "error", err)
			}
		}
	}
}

func (s *RecipeService) processInstructionImages(ctx context.Context, recipe *domain.Recipe) {
	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, instruction := range recipe.Instructions {
		if instruction.Image == nil || *instruction.Image == "" {
			continue
		}
		g.Go(func() error {
			remoteUrl := *instruction.Image
			basePath := "recipe/" + recipe.ID.String() + "/instruction/" + instruction.ID.String()
			if info, err := s.imageService.DownloadAndSaveImage(ctx, remoteUrl, basePath); err != nil {
				return fmt.Errorf("failed to download instruction from url %s: %w", remoteUrl, err)
			} else {
				instruction.DownloadUrl = &info.Path
				if err := s.repo.UpdateInstruction(instruction); err != nil {
					return fmt.Errorf("failed to update instruction: %w", err)
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("instruction image processing completed with errors", "error", err)
	}
}
