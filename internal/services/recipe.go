package services

import (
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type recipeService struct {
	repo         domain.RecipeRepository
	userRepo     domain.UserRepository
	imageService domain.ImageService
	foodService  domain.FoodService
	unitService  domain.UnitService
}

func NewRecipeService(repo domain.RecipeRepository, userRepo domain.UserRepository, imageService domain.ImageService, foodService domain.FoodService, unitService domain.UnitService) domain.RecipeService {
	return &recipeService{
		repo:         repo,
		userRepo:     userRepo,
		imageService: imageService,
		foodService:  foodService,
		unitService:  unitService,
	}
}

func (s *recipeService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.Recipe, error) {
	recipe, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	// Only anonymous and household-owned recipes are readable
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return recipe, sentinels.ErrForbidden
	}
	return recipe, nil
}

func (s *recipeService) ByIDPreload(id, userID, householdID uuid.UUID, preload types.PreloadOptions) (*domain.Recipe, error) {
	recipe, err := s.repo.ByIDPreload(id, userID, householdID, preload)
	if err != nil {
		return nil, err
	}
	// Only anonymous and household-owned recipes are readable
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return recipe, sentinels.ErrForbidden
	}
	return recipe, nil
}

func (s *recipeService) ByUrl(url string, householdID uuid.UUID) (*domain.Recipe, error) {
	recipe, err := s.repo.ByUrl(url)
	if err != nil {
		return nil, err
	}
	// Only anonymous and household-owned recipes are readable
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return recipe, nil
}

func (s *recipeService) ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]domain.Recipe, error) {
	return s.repo.ByParentIDsAndHousehold(parentIDs, householdID, preload)
}

func (s *recipeService) Search(userID uuid.UUID, householdID uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	return s.repo.Search(userID, householdID, opts)
}

func (s *recipeService) Create(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	recipe.HouseholdID = &householdID
	recipe.UserID = &userID
	if err := s.repo.Create(recipe); err != nil {
		return err
	}
	return s.UserSave(recipe.ID, userID, householdID)
}

func (s *recipeService) Import(recipe *domain.Recipe) error {
	return s.repo.Import(recipe)
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

func (s *recipeService) EstimatePrice(recipeID uuid.UUID, householdID uuid.UUID) (*domain.RecipePriceEstimate, error) {
	recipe, err := s.ByIDPreload(recipeID, uuid.Nil, householdID, types.Preload("ingredients"))
	if err != nil {
		return nil, err
	}

	foodIDs := make([]uuid.UUID, 0, len(recipe.Ingredients))
	for _, ing := range recipe.Ingredients {
		if ing.FoodID != nil {
			foodIDs = append(foodIDs, *ing.FoodID)
		}
	}

	latestPrices, err := s.foodService.LatestPrices(householdID, foodIDs)
	if err != nil {
		return nil, err
	}

	estimate := &domain.RecipePriceEstimate{
		MissingPrices: make([]uuid.UUID, 0, len(recipe.Ingredients)),
	}

	for _, ing := range recipe.Ingredients {
		if ing.FoodID == nil || ing.Amount == nil || ing.UnitID == nil {
			continue // unquantified ingredient; skip
		}

		foodPrice, ok := latestPrices[*ing.FoodID]
		if !ok {
			estimate.MissingPrices = append(estimate.MissingPrices, *ing.FoodID)
			continue
		}

		convertedAmount, convErr := s.unitService.Convert(*ing.Amount, *ing.UnitID, foodPrice.UnitID)
		if convErr != nil {
			// Incompatible units (e.g. price in kg, ingredient in ml): treat as unpriced.
			estimate.MissingPrices = append(estimate.MissingPrices, *ing.FoodID)
			continue
		}

		estimate.Total += (convertedAmount / foodPrice.Amount) * foodPrice.Price
	}

	if recipe.Yield != nil && *recipe.Yield > 0 && estimate.Total > 0 {
		estimate.PerServing = new(estimate.Total / float64(*recipe.Yield))
	}

	return estimate, nil
}
