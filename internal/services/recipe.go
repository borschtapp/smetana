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
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type recipeService struct {
	repo                domain.RecipeRepository
	userRepo            domain.UserRepository
	imageService        domain.ImageService
	publisherService    domain.PublisherService
	recipeAuthorService domain.RecipeAuthorService
	foodRepo            domain.FoodRepository
	unitRepo            domain.UnitRepository
	taxonomyRepo        domain.TaxonomyRepository
	equipmentRepo       domain.EquipmentRepository
	scraperService      domain.ScraperService
	fetchConcurrency    int
}

func NewRecipeService(repo domain.RecipeRepository, userRepo domain.UserRepository, imageService domain.ImageService, publisherService domain.PublisherService, recipeAuthorService domain.RecipeAuthorService, foodRepo domain.FoodRepository, unitRepo domain.UnitRepository, taxonomyRepo domain.TaxonomyRepository, equipmentRepo domain.EquipmentRepository, scraperService domain.ScraperService) domain.RecipeService {
	return &recipeService{
		repo:                repo,
		userRepo:            userRepo,
		imageService:        imageService,
		publisherService:    publisherService,
		recipeAuthorService: recipeAuthorService,
		foodRepo:            foodRepo,
		unitRepo:            unitRepo,
		taxonomyRepo:        taxonomyRepo,
		equipmentRepo:       equipmentRepo,
		scraperService:      scraperService,
		fetchConcurrency:    utils.GetenvInt("FETCH_CONCURRENCY", 5),
	}
}

func (s *recipeService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
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

func (s *recipeService) Search(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	return s.repo.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts})
}

func (s *recipeService) Create(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	recipe.HouseholdID = &householdID
	recipe.UserID = &userID
	if err := s.repo.Create(recipe); err != nil {
		return err
	}
	return s.UserSave(recipe.ID, userID, householdID)
}

func (s *recipeService) Update(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
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
func (s *recipeService) cloneToHousehold(global *domain.Recipe, userID, householdID uuid.UUID) (*domain.Recipe, error) {
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

	clone.Equipment = global.Equipment // GORM re-creates junction rows for the new recipeID

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

func (s *recipeService) Delete(id uuid.UUID, householdID uuid.UUID) error {
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

func (s *recipeService) deleteImages(images []*domain.Image) {
	for _, image := range images {
		if image.ID == uuid.Nil {
			continue
		}
		if err := s.imageService.Delete(image.ID); err != nil {
			log.Warnw("failed to delete image", "image_id", image.ID, "error", err)
		}
	}
}

func (s *recipeService) UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error {
	return s.repo.UserSave(recipeID, userID, householdID)
}

func (s *recipeService) UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error {
	return s.repo.UserUnsave(recipeID, userID)
}

func (s *recipeService) CreateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.CreateIngredient(ingredient)
}

func (s *recipeService) UpdateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.UpdateIngredient(ingredient)
}

func (s *recipeService) DeleteIngredient(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.DeleteIngredient(id, recipeID)
}

func (s *recipeService) AddEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.AddEquipment(recipeID, equipmentID)
}

func (s *recipeService) RemoveEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.RemoveEquipment(recipeID, equipmentID)
}

func (s *recipeService) CreateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.CreateInstruction(instruction)
}

func (s *recipeService) UpdateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return err
	}
	return s.repo.UpdateInstruction(instruction)
}

func (s *recipeService) DeleteInstruction(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return err
	}
	return s.repo.DeleteInstruction(id, recipeID)
}

// ImportFromURL imports a recipe from URL and saves it for the given user.
func (s *recipeService) ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	existing, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrNotFound) {
		return nil, err
	}
	if existing != nil {
		if err := s.UserSave(existing.ID, userID, householdID); err != nil {
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
	if err := s.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *recipeService) ImportRecipe(ctx context.Context, recipe *domain.Recipe) (*domain.Recipe, error) {
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

	resolvedEquipment := recipe.Equipment[:0]
	for _, equipment := range recipe.Equipment {
		remoteImage := equipment.RemoteImage
		if err := s.equipmentRepo.FindOrCreate(equipment); err == nil {
			equipment.RemoteImage = remoteImage // restore for processEquipmentImages
			resolvedEquipment = append(resolvedEquipment, equipment)
		} else {
			log.Warnw("error creating equipment", "equipment", equipment, "error", err)
		}
	}
	recipe.Equipment = resolvedEquipment

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreate(ctx, recipe.Publisher); err != nil {
			log.Warnw("error creating publisher", "publisher", recipe.Publisher, "error", err)
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	if recipe.Author != nil {
		if err := s.recipeAuthorService.FindOrCreate(ctx, recipe.Author); err != nil {
			log.Warnw("error creating recipe author", "author", recipe.Author, "error", err)
		} else {
			recipe.AuthorID = &recipe.Author.ID
		}
	}

	if err := s.repo.Import(recipe); err != nil {
		return nil, err
	}

	s.processRecipeImages(ctx, recipe)
	s.processInstructionImages(ctx, recipe)
	s.processEquipmentImages(ctx, recipe)
	s.processFoodIcons(ctx, recipe)
	return recipe, nil
}

func (s *recipeService) processRecipeImages(ctx context.Context, recipe *domain.Recipe) {
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
		log.Warnw("recipe image processing completed with errors", "error", err)
	}

	best := selectBestImage(results)
	if best == nil {
		return
	}

	if err := s.imageService.SetDefault(best); err != nil {
		log.Warnw("failed to set default recipe image", "error", err)
		return
	}

	recipe.ImagePath = best.Path
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
		if image == nil || image.Path == nil {
			continue
		}
		dim := max(image.Width, image.Height)
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

func (s *recipeService) processEquipmentImages(ctx context.Context, recipe *domain.Recipe) {
	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, equipment := range recipe.Equipment {
		if equipment == nil || equipment.RemoteImage == nil || *equipment.RemoteImage == "" {
			continue
		}
		// Skip if equipment already has an image
		if equipment.ImagePath != nil {
			continue
		}
		eq := equipment // capture for closure
		g.Go(func() error {
			image := &domain.Image{
				EntityType: "equipment",
				EntityID:   eq.ID,
				SourceURL:  *eq.RemoteImage,
			}

			if err := s.imageService.PersistRemote(ctx, image, ""); err != nil {
				return fmt.Errorf("failed to download equipment image %s: %w", *eq.RemoteImage, err)
			}

			if err := s.imageService.SetDefault(image); err != nil {
				return fmt.Errorf("failed to set equipment image default: %w", err)
			}

			eq.ImagePath = image.Path
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("equipment image processing completed with errors", "error", err)
	}
}

func (s *recipeService) processInstructionImages(ctx context.Context, recipe *domain.Recipe) {
	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, instruction := range recipe.Instructions {
		if instruction.RemoteImage == nil || *instruction.RemoteImage == "" {
			continue
		}
		g.Go(func() error {
			image := &domain.Image{
				EntityType: "recipe_instructions",
				EntityID:   instruction.ID,
				SourceURL:  *instruction.RemoteImage,
			}

			// Group instruction images under recipes/{recipeID}/ alongside recipe images.
			if err := s.imageService.PersistRemote(ctx, image, "recipes/"+recipe.ID.String()); err != nil {
				return fmt.Errorf("failed to download instruction image %v: %w", instruction.RemoteImage, err)
			}

			if err := s.imageService.SetDefault(image); err != nil {
				return fmt.Errorf("failed to set instruction image default: %w", err)
			}

			instruction.ImagePath = image.Path
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("instruction image processing completed with errors", "error", err)
	}
}

func (s *recipeService) processFoodIcons(ctx context.Context, recipe *domain.Recipe) {
	var g errgroup.Group
	g.SetLimit(s.fetchConcurrency)

	for _, ingredient := range recipe.Ingredients {
		food := ingredient.Food
		if food == nil || food.RemoteImage == nil || food.ImagePath != nil {
			continue // skip if no remote icon, or food already has a local image
		}
		g.Go(func() error {
			image := &domain.Image{
				EntityType: "food",
				EntityID:   food.ID,
				SourceURL:  *food.RemoteImage,
			}

			if err := s.imageService.PersistRemote(ctx, image, ""); err != nil {
				return fmt.Errorf("failed to download food icon %s: %w", *food.RemoteImage, err)
			}

			if err := s.imageService.SetDefault(image); err != nil {
				return fmt.Errorf("failed to set food icon default: %w", err)
			}

			food.ImagePath = image.Path
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Warnw("food icon processing completed with errors", "error", err)
	}
}
