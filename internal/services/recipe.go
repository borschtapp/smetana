package services

import (
	"fmt"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
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
		return nil, fmt.Errorf("by id: %w", err)
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
		return nil, fmt.Errorf("by id preload: %w", err)
	}
	// Only anonymous and household-owned recipes are readable
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return recipe, sentinels.ErrForbidden
	}
	return recipe, nil
}

func (s *recipeService) ByUrl(url string, householdID uuid.UUID) (*domain.Recipe, error) {
	recipe, err := s.repo.ByUrl(utils.NormalizeURL(url))
	if err != nil {
		return nil, fmt.Errorf("by url: %w", err)
	}
	// Only anonymous and household-owned recipes are readable
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return recipe, nil
}

func (s *recipeService) ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]domain.Recipe, error) {
	recipes, err := s.repo.ByParentIDsAndHousehold(parentIDs, householdID, preload)
	if err != nil {
		return nil, fmt.Errorf("by parent ids and household: %w", err)
	}
	return recipes, nil
}

func (s *recipeService) Search(userID uuid.UUID, householdID uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	recipes, total, err := s.repo.Search(userID, householdID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	return recipes, total, nil
}

func (s *recipeService) Create(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	recipe.HouseholdID = &householdID
	recipe.UserID = &userID
	if err := s.repo.Create(recipe); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if err := s.UserSave(recipe.ID, userID, householdID); err != nil {
		return fmt.Errorf("auto-save after create: %w", err)
	}
	return nil
}

func (s *recipeService) Import(recipe *domain.Recipe) error {
	if err := s.repo.Import(recipe); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	return nil
}

func (s *recipeService) SetFeedID(recipeID, feedID uuid.UUID) error {
	if err := s.repo.Update(&domain.Recipe{ID: recipeID, FeedID: &feedID}); err != nil {
		return fmt.Errorf("set feed id: %w", err)
	}
	return nil
}

func (s *recipeService) Update(recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(recipe.ID)
	if err != nil {
		return fmt.Errorf("update (fetch existing): %w", err)
	}

	// Copy-on-write: if it doesn't belong to household yet, clone it into the household first
	if existing.HouseholdID == nil {
		cloned, err := s.cloneToHousehold(existing, userID, householdID)
		if err != nil {
			return fmt.Errorf("update (clone to household): %w", err)
		}

		// Apply the incoming patch fields onto the clone
		recipe.ID = cloned.ID
		recipe.HouseholdID = cloned.HouseholdID
		recipe.ParentID = cloned.ParentID
		recipe.UserID = cloned.UserID
	}
	if err := s.repo.Update(recipe); err != nil {
		return fmt.Errorf("update (persist): %w", err)
	}
	return nil
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
		return nil, fmt.Errorf("clone to household transaction: %w", err)
	}
	return newRecipe, nil
}

func (s *recipeService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(id)
	if err != nil {
		return fmt.Errorf("delete (fetch existing): %w", err)
	}
	if existing.HouseholdID == nil || *existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	if existing.ParentID == nil {
		s.deleteImages(existing.Images)
	}

	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("delete (persist): %w", err)
	}
	return nil
}

func (s *recipeService) deleteImages(images []*domain.Image) {
	for _, image := range images {
		if image.ID == uuid.Nil {
			continue
		}
		if err := s.imageService.Delete(image.ID); err != nil {
			log.Warnw("failed to delete image", "image_id", image.ID, "error", err.Error())
		}
	}
}

func (s *recipeService) UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error {
	if err := s.repo.UserSave(recipeID, userID, householdID); err != nil {
		return fmt.Errorf("user save: %w", err)
	}
	return nil
}

func (s *recipeService) UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error {
	if err := s.repo.UserUnsave(recipeID, userID); err != nil {
		return fmt.Errorf("user unsave: %w", err)
	}
	return nil
}

func (s *recipeService) CreateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return fmt.Errorf("create ingredient (auth check): %w", err)
	}
	if err := s.repo.CreateIngredient(ingredient); err != nil {
		return fmt.Errorf("create ingredient (persist): %w", err)
	}
	return nil
}

func (s *recipeService) UpdateIngredient(ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return fmt.Errorf("update ingredient (auth check): %w", err)
	}
	if err := s.repo.UpdateIngredient(ingredient); err != nil {
		return fmt.Errorf("update ingredient (persist): %w", err)
	}
	return nil
}

func (s *recipeService) DeleteIngredient(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return fmt.Errorf("delete ingredient (auth check): %w", err)
	}
	if err := s.repo.DeleteIngredient(id, recipeID); err != nil {
		return fmt.Errorf("delete ingredient (persist): %w", err)
	}
	return nil
}

func (s *recipeService) AddEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return fmt.Errorf("add equipment (auth check): %w", err)
	}
	if err := s.repo.AddEquipment(recipeID, equipmentID); err != nil {
		return fmt.Errorf("add equipment (persist): %w", err)
	}
	return nil
}

func (s *recipeService) RemoveEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return fmt.Errorf("remove equipment (auth check): %w", err)
	}
	if err := s.repo.RemoveEquipment(recipeID, equipmentID); err != nil {
		return fmt.Errorf("remove equipment (persist): %w", err)
	}
	return nil
}

func (s *recipeService) CreateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return fmt.Errorf("create instruction (auth check): %w", err)
	}
	if err := s.repo.CreateInstruction(instruction); err != nil {
		return fmt.Errorf("create instruction (persist): %w", err)
	}
	return nil
}

func (s *recipeService) UpdateInstruction(instruction *domain.RecipeInstruction, householdID uuid.UUID) error {
	if _, err := s.ByID(instruction.RecipeID, householdID); err != nil {
		return fmt.Errorf("update instruction (auth check): %w", err)
	}
	if err := s.repo.UpdateInstruction(instruction); err != nil {
		return fmt.Errorf("update instruction (persist): %w", err)
	}
	return nil
}

func (s *recipeService) DeleteInstruction(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ByID(recipeID, householdID); err != nil {
		return fmt.Errorf("delete instruction (auth check): %w", err)
	}
	if err := s.repo.DeleteInstruction(id, recipeID); err != nil {
		return fmt.Errorf("delete instruction (persist): %w", err)
	}
	return nil
}

func (s *recipeService) EstimatePrice(recipeID uuid.UUID, householdID uuid.UUID) (*domain.RecipePriceEstimate, error) {
	recipe, err := s.ByIDPreload(recipeID, uuid.Nil, householdID, types.Preload("ingredients"))
	if err != nil {
		return nil, fmt.Errorf("estimate price (fetch recipe): %w", err)
	}

	foodIDs := make([]uuid.UUID, 0, len(recipe.Ingredients))
	for _, ing := range recipe.Ingredients {
		if ing.FoodID != nil {
			foodIDs = append(foodIDs, *ing.FoodID)
		}
	}

	latestPrices, err := s.foodService.LatestPrices(householdID, foodIDs)
	if err != nil {
		return nil, fmt.Errorf("estimate price (fetch food prices): %w", err)
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
