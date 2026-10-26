package services

import (
	"context"
	"errors"
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

func (s *recipeService) Create(ctx context.Context, recipe *domain.Recipe, userID uuid.UUID, householdID uuid.UUID) error {
	recipe.HouseholdID = &householdID
	recipe.UserID = &userID
	detachNested(recipe)
	if err := s.canonicalizeIngredients(ctx, recipe.Ingredients...); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	err := s.repo.Transaction(func(txRepo domain.RecipeRepository) error {
		if err := txRepo.Create(recipe); err != nil {
			return err
		}
		return txRepo.UserSave(recipe.ID, userID, householdID)
	})
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func (s *recipeService) ImportTarget(id uuid.UUID, sourceUrl *string) (*domain.Recipe, error) {
	return s.repo.ImportTarget(id, sourceUrl)
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

	if existing.HouseholdID != nil && *existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	// Copy-on-write: if it doesn't belong to household yet, clone it into the household first
	if existing.HouseholdID == nil {
		cloned, err := s.cloneToHousehold(recipe.ID, userID, householdID)
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

// detachNested clears client IDs on nested rows: GORM upserts them by ID, moving other recipes' rows into this one.
// Collections are dropped, as GORM would join any collection by ID without an ownership check.
func detachNested(recipe *domain.Recipe) {
	recipe.Collections = nil
	for _, img := range recipe.Images {
		if img != nil {
			img.ID = uuid.Nil
		}
	}
	for _, ing := range recipe.Ingredients {
		if ing != nil {
			ing.ID = uuid.Nil
		}
	}
	for _, ins := range recipe.Instructions {
		if ins != nil {
			detachInstruction(ins)
			ins.ParentID = nil
		}
	}
}

// detachInstruction makes a client-supplied instruction and its images new rows.
func detachInstruction(ins *domain.RecipeInstruction) {
	ins.ID, ins.Parent = uuid.Nil, nil
	for _, img := range ins.Images {
		if img != nil {
			img.ID = uuid.Nil
		}
	}
}

// canonicalizeIngredients replaces alias unit and food IDs on client-supplied ingredients with their canonical IDs.
// Nested unit/food objects are reduced to their IDs: GORM would save them and write their IDs back over unit_id/food_id.
// Nested objects without an ID are found or created by name.
func (s *recipeService) canonicalizeIngredients(ctx context.Context, ingredients ...*domain.RecipeIngredient) error {
	findUnit := byNameOnce("unit", func(name, slug string) (uuid.UUID, error) {
		unit := &domain.Unit{Name: name, Slug: slug}
		err := s.unitService.FindOrCreate(unit)
		return unit.ID, err
	})
	// Only the name and slug are passed on, so client image URLs are never fetched.
	findFood := byNameOnce("food", func(name, slug string) (uuid.UUID, error) {
		food := &domain.Food{Name: name, Slug: slug}
		err := s.foodService.FindOrCreate(ctx, food)
		return food.ID, err
	})

	var unitIDs, foodIDs []*uuid.UUID // client-supplied only: IDs found by name are canonical already
	for _, ing := range ingredients {
		if ing == nil {
			continue
		}
		switch {
		case ing.UnitID != nil:
			unitIDs = append(unitIDs, ing.UnitID)
		case ing.Unit != nil && ing.Unit.ID != uuid.Nil:
			ing.UnitID = &ing.Unit.ID
			unitIDs = append(unitIDs, ing.UnitID)
		case ing.Unit != nil:
			id, err := findUnit(ing.Unit.Name, ing.Unit.Slug)
			if err != nil {
				return err
			}
			ing.UnitID = &id
		}
		switch {
		case ing.FoodID != nil:
			foodIDs = append(foodIDs, ing.FoodID)
		case ing.Food != nil && ing.Food.ID != uuid.Nil:
			ing.FoodID = &ing.Food.ID
			foodIDs = append(foodIDs, ing.FoodID)
		case ing.Food != nil:
			id, err := findFood(ing.Food.Name, ing.Food.Slug)
			if err != nil {
				return err
			}
			ing.FoodID = &id
		}
		ing.Unit, ing.Food = nil, nil
	}
	if err := s.unitService.Canonicalize(unitIDs...); err != nil {
		return err
	}
	return s.foodService.Canonicalize(foodIDs...)
}

// byNameOnce wraps findOrCreate so each key is looked up once per call site; a missing name is a bad request.
func byNameOnce(kind string, findOrCreate func(name, slug string) (uuid.UUID, error)) func(name, slug string) (uuid.UUID, error) {
	found := make(map[string]uuid.UUID)
	return func(name, slug string) (uuid.UUID, error) {
		if name == "" {
			return uuid.Nil, sentinels.BadRequest("ingredient " + kind + " needs an id or a name")
		}
		key := resolveKey(slug, name)
		if id, ok := found[key]; ok {
			return id, nil
		}
		id, err := findOrCreate(name, slug)
		if err != nil {
			return uuid.Nil, err
		}
		found[key] = id
		return id, nil
	}
}

// cloneInstructions copies instructions as new rows that keep their sub-step links. Parents come before
// their children, so each row's parent is already inserted, even where the database checks every row.
func cloneInstructions(instructions []*domain.RecipeInstruction) ([]*domain.RecipeInstruction, error) {
	newIDs := make(map[uuid.UUID]uuid.UUID, len(instructions))
	for _, ins := range instructions {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("new instruction id: %w", err)
		}
		newIDs[ins.ID] = id
	}

	copies := make([]*domain.RecipeInstruction, 0, len(instructions))
	add := func(ins *domain.RecipeInstruction, parentID *uuid.UUID) {
		copy_ := *ins
		detachInstruction(&copy_)
		copy_.ID = newIDs[ins.ID]
		copy_.RecipeID, copy_.Recipe, copy_.ParentID = uuid.Nil, nil, nil
		if parentID != nil {
			copy_.ParentID = new(newIDs[*parentID])
		}
		copies = append(copies, &copy_)
	}

	added := make(map[uuid.UUID]bool, len(instructions))
	for pending := instructions; len(pending) > 0; {
		var waiting []*domain.RecipeInstruction
		for _, ins := range pending {
			if ins.ParentID != nil && !added[*ins.ParentID] {
				waiting = append(waiting, ins)
				continue
			}
			add(ins, ins.ParentID)
			added[ins.ID] = true
		}
		if len(waiting) == len(pending) {
			// A parent outside the recipe, or a loop: keep the rest as top-level steps.
			for _, ins := range waiting {
				add(ins, nil)
			}
			break
		}
		pending = waiting
	}
	return copies, nil
}

// clonePreload lists the associations a household clone copies; images are shared via the parent, not copied.
var clonePreload = types.Preload("ingredients", "instructions", "equipment", "taxonomies", "nutrition")

// cloneToHousehold clones a global recipe, with its ingredients, instructions and other associations, into the given household.
func (s *recipeService) cloneToHousehold(globalID, userID, householdID uuid.UUID) (*domain.Recipe, error) {
	global, err := s.repo.ByIDPreload(globalID, userID, householdID, clonePreload)
	if err != nil {
		return nil, fmt.Errorf("fetch global recipe: %w", err)
	}

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
		copy_.Food, copy_.Unit = nil, nil // linked by FoodID/UnitID; GORM would otherwise insert them again
		clone.Ingredients[i] = &copy_
	}
	if clone.Instructions, err = cloneInstructions(global.Instructions); err != nil {
		return nil, err
	}

	clone.Equipment = global.Equipment // GORM re-creates junction rows for the new recipeID
	if global.Nutrition != nil {
		nutrition := *global.Nutrition
		clone.Nutrition = &nutrition
	}

	var newRecipe *domain.Recipe
	err = s.repo.Transaction(func(txRepo domain.RecipeRepository) error {
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

func (s *recipeService) CreateIngredient(ctx context.Context, ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return fmt.Errorf("create ingredient (auth check): %w", err)
	}
	if err := s.canonicalizeIngredients(ctx, ingredient); err != nil {
		return fmt.Errorf("create ingredient: %w", err)
	}
	if err := s.repo.CreateIngredient(ingredient); err != nil {
		return fmt.Errorf("create ingredient (persist): %w", err)
	}
	return nil
}

func (s *recipeService) UpdateIngredient(ctx context.Context, ingredient *domain.RecipeIngredient, householdID uuid.UUID) error {
	if _, err := s.ByID(ingredient.RecipeID, householdID); err != nil {
		return fmt.Errorf("update ingredient (auth check): %w", err)
	}
	if err := s.canonicalizeIngredients(ctx, ingredient); err != nil {
		return fmt.Errorf("update ingredient: %w", err)
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
	detachInstruction(instruction)
	if err := s.checkParentInstruction(instruction); err != nil {
		return fmt.Errorf("create instruction: %w", err)
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
	if err := s.checkParentInstruction(instruction); err != nil {
		return fmt.Errorf("update instruction: %w", err)
	}
	if err := s.repo.UpdateInstruction(instruction); err != nil {
		return fmt.Errorf("update instruction (persist): %w", err)
	}
	return nil
}

// checkParentInstruction rejects a parent_id that is not another instruction of the same recipe,
// or that would make the instruction its own ancestor.
func (s *recipeService) checkParentInstruction(instruction *domain.RecipeInstruction) error {
	if instruction.ParentID == nil {
		return nil
	}
	parents, err := s.repo.InstructionParents(instruction.RecipeID)
	if err != nil {
		return fmt.Errorf("fetch instruction parents: %w", err)
	}
	if _, ok := parents[*instruction.ParentID]; !ok {
		return sentinels.BadRequest("parent instruction must be another instruction of the same recipe")
	}
	seen := make(map[uuid.UUID]bool)
	for id := instruction.ParentID; id != nil && !seen[*id]; id = parents[*id] {
		if *id == instruction.ID {
			return sentinels.BadRequest("an instruction cannot be nested under itself")
		}
		seen[*id] = true
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

func (s *recipeService) EstimatePrice(recipeID uuid.UUID, householdID uuid.UUID) (*domain.RecipeCostEstimate, error) {
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

	// Fetch all foods to check pantry status and filter them out
	foods, err := s.foodService.ByIDs(foodIDs)
	if err != nil {
		return nil, fmt.Errorf("estimate price (fetch foods): %w", err)
	}

	pricedFoodIDs := make([]uuid.UUID, 0, len(foodIDs))
	for _, foodID := range foodIDs {
		if food, exists := foods[foodID]; !exists || !food.Pantry {
			pricedFoodIDs = append(pricedFoodIDs, foodID)
		}
	}

	latestPrices, err := s.foodService.LatestPrices(householdID, pricedFoodIDs)
	if err != nil {
		return nil, fmt.Errorf("estimate price (fetch food prices): %w", err)
	}

	foodIDsWithPrice := make([]uuid.UUID, 0, len(latestPrices))
	for id := range latestPrices {
		foodIDsWithPrice = append(foodIDsWithPrice, id)
	}
	conversions, err := s.foodService.ConversionsByFoodIDs(foodIDsWithPrice)
	if err != nil {
		return nil, fmt.Errorf("estimate price (fetch food conversions): %w", err)
	}

	// Ingredient and price units are preloaded; only conversion units may need a query.
	converter := s.unitService.NewConverter()
	converter.Expect(conversionUnitIDs(conversions)...)
	for _, price := range latestPrices {
		converter.Add(price.Unit)
	}
	for _, ing := range recipe.Ingredients {
		converter.Add(ing.Unit)
	}

	estimate := &domain.RecipeCostEstimate{
		Items: make([]*domain.RecipeIngredientCost, 0, len(recipe.Ingredients)),
	}

	for _, ing := range recipe.Ingredients {
		ingCost := &domain.RecipeIngredientCost{
			IngredientID: ing.ID,
		}

		// Unquantified ingredients (no amount/food) are skipped; a missing unit means pieces
		if ing.FoodID == nil || ing.Amount == nil {
			continue
		}

		// Pantry items are excluded from cost
		if food, exists := foods[*ing.FoodID]; exists && food.Pantry {
			ingCost.Status = "pantry"
			estimate.Items = append(estimate.Items, ingCost)
			continue
		}

		foodPrice, ok := latestPrices[*ing.FoodID]
		if !ok {
			ingCost.Status = "missing_price"
			estimate.Items = append(estimate.Items, ingCost)
			continue
		}
		ingCost.FoodPrice = foodPrice // also reported on incompatible_unit, so clients can show which price didn't fit

		convertedAmount, convErr := converter.Convert(*ing.Amount, ing.UnitID, &foodPrice.UnitID, conversions[*ing.FoodID])
		if errors.Is(convErr, sentinels.ErrUnitConversion) {
			// No conversion path (e.g. price in kg, ingredient in ml, no food conversion): treat as unpriced.
			ingCost.Status = "incompatible_unit"
			estimate.Items = append(estimate.Items, ingCost)
			continue
		}
		if convErr != nil {
			return nil, fmt.Errorf("estimate price (convert units): %w", convErr)
		}

		cost := (convertedAmount / foodPrice.Amount) * foodPrice.Price

		ingCost.Cost = &cost
		ingCost.Status = "calculated"

		estimate.Total += cost
		estimate.Items = append(estimate.Items, ingCost)
	}

	if recipe.Yield != nil && *recipe.Yield > 0 && estimate.Total > 0 {
		estimate.PerServing = new(estimate.Total / float64(*recipe.Yield))
	}

	return estimate, nil
}
