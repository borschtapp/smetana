package services

import (
	"errors"
	"sync"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type RecipeService struct {
	repo             domain.RecipeRepository
	imageService     domain.ImageService
	publisherService domain.PublisherService
	foodRepo         domain.FoodRepository
	unitRepo         domain.UnitRepository
	userRepo         domain.UserRepository
}

func NewRecipeService(repo domain.RecipeRepository, imageService domain.ImageService, publisherService domain.PublisherService, foodRepo domain.FoodRepository, unitRepo domain.UnitRepository, userRepo domain.UserRepository) domain.RecipeService {
	return &RecipeService{
		repo:             repo,
		imageService:     imageService,
		publisherService: publisherService,
		foodRepo:         foodRepo,
		unitRepo:         unitRepo,
		userRepo:         userRepo,
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
	return s.repo.Search(userID, householdID, opts)
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
	if _, err := s.ByID(id, householdID); err != nil {
		return err
	}
	return s.repo.Delete(id)
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
	return s.repo.DeleteIngredient(id)
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
	return s.repo.DeleteInstruction(id)
}

// ImportFromURL imports a recipe from URL and saves it for the given user.
// If the recipe already exists: saves it for the user (unless forceUpdate=true, in which case it is re-imported).
func (s *RecipeService) ImportFromURL(url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	existing, err := s.repo.ByUrl(url)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return nil, err
	}
	if existing != nil {
		if !forceUpdate {
			if err := s.UserSave(existing.ID, userID, householdID); err != nil {
				return nil, err
			}
			return existing, nil
		}
		if err := s.repo.Delete(existing.ID); err != nil {
			return nil, err
		}
	}

	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return nil, err
	}
	recipe, err := s.ImportFromKripRecipe(kripRecipe, nil)

	if err != nil {
		return nil, err
	}
	if err := s.UserSave(recipe.ID, userID, householdID); err != nil {
		return nil, err
	}
	return recipe, nil
}

func (s *RecipeService) ImportFromKripRecipe(kripRecipe *model.Recipe, feedID *uuid.UUID) (*domain.Recipe, error) {
	recipe := domain.FromKripRecipe(kripRecipe)
	recipe.FeedID = feedID

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreate(recipe.Publisher); err != nil {
			log.Warnf("error creating publisher %v: %s", recipe.Publisher, err.Error())
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	s.parseAndEnrichIngredients(recipe.Ingredients, kripRecipe.Language)

	if err := s.repo.Import(recipe); err != nil {
		return nil, err
	}

	s.processRecipeImages(recipe)
	return recipe, nil
}

func (s *RecipeService) parseAndEnrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		parsed, err := kapusta.ParseIngredient(ingredient.RawText, language)
		if err != nil || parsed == nil {
			continue
		}

		ingredient.Amount = &parsed.Quantity
		if len(parsed.Annotation) != 0 {
			ingredient.Note = &parsed.Annotation
		}
		if len(parsed.Ingredient) != 0 {
			food := &domain.Food{Name: parsed.Ingredient}
			if err := s.foodRepo.FindOrCreate(food); err != nil {
				log.Warnf("error creating food %v: %s", food, err.Error())
			} else {
				ingredient.Food = food
				ingredient.FoodID = &food.ID
			}
		}
		if len(parsed.Unit) != 0 {
			unit := &domain.Unit{Name: parsed.Unit}
			if err := s.unitRepo.FindOrCreate(unit); err != nil {
				log.Warnf("error creating unit %v: %s", unit, err.Error())
			} else {
				ingredient.Unit = unit
				ingredient.UnitID = &unit.ID
			}
		}
	}
}

func (s *RecipeService) processRecipeImages(recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	for _, image := range recipe.Images {
		image.RecipeID = recipe.ID
	}

	if err := s.repo.CreateImages(recipe.Images); err != nil {
		log.Warnf("failed to save images: %v", err)
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, image := range recipe.Images {
		wg.Add(1)
		go func(img *domain.RecipeImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			basePath := "recipe/" + img.RecipeID.String() + "/" + img.ID.String()
			if info, err := s.imageService.DownloadAndSaveImage(img.RemoteUrl, basePath); err != nil {
				log.Warnf("failed to download image: %v", err)
			} else {
				img.DownloadUrl = &info.Path
				img.Width = info.Width
				img.Height = info.Height
			}
		}(image)
	}

	wg.Wait()

	for _, img := range recipe.Images {
		if img.DownloadUrl != nil {
			err := s.repo.UpdateImage(img)
			if err != nil {
				log.Warnf("failed to update image: %v", err)
			}
		}
	}
}
