package services_test

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/types"
)

type recipeServiceDeps struct {
	repo        *stubRecipeRepo
	userRepo    *stubUserRepo
	imgService  *stubImageService
	foodService domain.FoodService
	unitService domain.UnitService
}

// newTestRecipeService builds a recipeService wired up with the provided stubs.
func newTestRecipeService(deps recipeServiceDeps) domain.RecipeService {
	if deps.imgService == nil {
		deps.imgService = &stubImageService{}
	}
	if deps.foodService == nil {
		deps.foodService = &stubFoodService{}
	}
	if deps.unitService == nil {
		deps.unitService = &stubUnitService{}
	}
	return services.NewRecipeService(deps.repo, deps.userRepo, deps.imgService, deps.foodService, deps.unitService)
}

func TestRecipeService_ByID_GlobalRecipe_AnyHouseholdCanRead(t *testing.T) {
	globalRecipe := &domain.Recipe{ID: uuid.New(), HouseholdID: nil}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return globalRecipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	got, err := svc.ByID(globalRecipe.ID, uuid.New()) // any household

	require.NoError(t, err)
	assert.Equal(t, globalRecipe.ID, got.ID)
}

func TestRecipeService_ByID_OwnedByOtherHousehold_ReturnsForbidden(t *testing.T) {
	otherHID := uuid.New()
	myHID := uuid.New()
	privateRecipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &otherHID}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return privateRecipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	_, err := svc.ByID(privateRecipe.ID, myHID)

	assert.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Update_OwnedByHousehold_UpdatesDirectly(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}

	updateCalled := false
	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		updateFn: func(r *domain.Recipe) error {
			updateCalled = true
			assert.Equal(t, recipe.ID, r.ID)
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(recipe, uuid.New(), hid)

	require.NoError(t, err)
	assert.True(t, updateCalled)
}

func TestRecipeService_Update_GlobalRecipe_ClonesBeforeUpdate(t *testing.T) {
	// When updating a recipe that has no HouseholdID (global/feed recipe),
	// the service must clone it into the household first (Copy-on-Write).
	globalID := uuid.New()
	myHID := uuid.New()
	uid := uuid.New()
	global := &domain.Recipe{ID: globalID, HouseholdID: nil, Name: new("Global")}

	var clonedID uuid.UUID
	createCalled := false
	replaceCalled := false

	repo := &stubRecipeRepo{
		byIDFn:        func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					createCalled = true
					assert.Equal(t, myHID, *r.HouseholdID)
					assert.Equal(t, globalID, *r.ParentID)
					r.ID = uuid.New() // simulate DB ID assignment
					clonedID = r.ID
					return nil
				},
				replaceRecipePointersFn: func(oldID, newID, h uuid.UUID) error {
					replaceCalled = true
					assert.Equal(t, globalID, oldID)
					assert.Equal(t, clonedID, newID)
					assert.Equal(t, myHID, h)
					return nil
				},
			})
		},
		updateFn: func(r *domain.Recipe) error {
			assert.Equal(t, clonedID, r.ID, "update must be called on the clone, not the original")
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	patch := &domain.Recipe{ID: globalID, Name: new("My Custom Version")}
	err := svc.Update(patch, uid, myHID)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, replaceCalled)
}

func TestRecipeService_Update_GlobalRecipe_ClonesFromPreloadedRecipe(t *testing.T) {
	hid := uuid.New()
	global := &domain.Recipe{ID: uuid.New()}
	var preload types.PreloadOptions
	repo := &stubRecipeRepo{
		byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return global, nil }, // no associations loaded
		byIDPreloadFn: func(_, _, _ uuid.UUID, p types.PreloadOptions) (*domain.Recipe, error) {
			preload = p
			return global, nil
		},
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn:                func(r *domain.Recipe) error { r.ID = uuid.New(); return nil },
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			})
		},
		updateFn: func(*domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	require.NoError(t, svc.Update(&domain.Recipe{ID: global.ID}, uuid.New(), hid))

	for _, name := range []string{"ingredients", "instructions", "equipment", "taxonomies", "nutrition"} {
		assert.True(t, preload.Has(name), "the clone must copy %s", name)
	}
}

func TestRecipeService_Update_OtherHouseholdsRecipe_Forbidden(t *testing.T) {
	owner := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &owner}
	repo := &stubRecipeRepo{
		byIDFn:   func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		updateFn: func(*domain.Recipe) error { t.Fatal("another household's recipe must not be updated"); return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: recipe.ID}, uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Update_GlobalRecipe_DeepClonesIngredientsAndInstructions(t *testing.T) {
	globalID := uuid.New()
	myHID := uuid.New()
	uid := uuid.New()

	foodID := uuid.New()
	unitID := uuid.New()

	global := &domain.Recipe{
		ID:          globalID,
		HouseholdID: nil,
		Ingredients: []*domain.RecipeIngredient{
			{ID: uuid.New(), FoodID: &foodID, UnitID: &unitID, Amount: new(100.0)},
		},
		Instructions: []*domain.RecipeInstruction{
			{ID: uuid.New(), Text: "Step 1"},
		},
	}

	var cloned *domain.Recipe
	repo := &stubRecipeRepo{
		byIDFn:        func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					r.ID = uuid.New()
					cloned = r
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			})
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: globalID}, uid, myHID)

	require.NoError(t, err)
	require.NotNil(t, cloned)

	// Verify ingredients deep clone
	assert.Len(t, cloned.Ingredients, 1)
	assert.Equal(t, uuid.Nil, cloned.Ingredients[0].ID, "cloned ingredient must have Nil ID before create")
	assert.Equal(t, foodID, *cloned.Ingredients[0].FoodID)
	assert.NotSame(t, global.Ingredients[0], cloned.Ingredients[0], "ingredient must be a deep copy")

	// Verify instructions deep clone
	assert.Len(t, cloned.Instructions, 1)
	assert.NotEqual(t, global.Instructions[0].ID, cloned.Instructions[0].ID, "cloned instruction must be a new row")
	assert.NotEqual(t, uuid.Nil, cloned.Instructions[0].ID, "IDs are assigned up front so sub-steps can point at them")
	assert.Equal(t, "Step 1", cloned.Instructions[0].Text)
	assert.NotSame(t, global.Instructions[0], cloned.Instructions[0], "instruction must be a deep copy")
}

func TestRecipeService_Update_GlobalRecipe_KeepsSubSteps(t *testing.T) {
	step, sub := uuid.New(), uuid.New()
	foodID := uuid.New()
	global := &domain.Recipe{
		ID:          uuid.New(),
		Ingredients: []*domain.RecipeIngredient{{ID: uuid.New(), FoodID: &foodID, Food: &domain.Food{ID: foodID}}},
		// The sub-step comes first, as Order may put it.
		Instructions: []*domain.RecipeInstruction{{ID: sub, ParentID: &step, Text: "2a"}, {ID: step, Text: "2"}},
	}
	var cloned *domain.Recipe
	repo := &stubRecipeRepo{
		byIDFn:        func(uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn:                func(r *domain.Recipe) error { cloned = r; return nil },
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			})
		},
		updateFn: func(*domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	require.NoError(t, svc.Update(&domain.Recipe{ID: global.ID}, uuid.New(), uuid.New()))

	require.NotNil(t, cloned)
	require.Len(t, cloned.Instructions, 2)
	parent, child := cloned.Instructions[0], cloned.Instructions[1]
	assert.Equal(t, "2", parent.Text, "a parent is inserted before its sub-steps")
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID, "the sub-step points at the cloned parent")
	assert.Nil(t, cloned.Ingredients[0].Food, "foods are linked by ID, not inserted again")
	assert.Equal(t, foodID, *cloned.Ingredients[0].FoodID)
	assert.Equal(t, &step, global.Instructions[0].ParentID, "the global recipe is left untouched")
}

func TestRecipeService_Update_GlobalRecipe_SharesAssociations(t *testing.T) {
	globalID := uuid.New()
	myHID := uuid.New()
	uid := uuid.New()

	publisher := &domain.Publisher{ID: uuid.New(), Name: "The Times"}
	author := &domain.Author{ID: uuid.New(), Name: "John Doe"}
	taxonomy := &domain.Taxonomy{ID: uuid.New(), Label: "Italian"}
	equipment := &domain.Equipment{ID: uuid.New(), Name: "Pan"}

	global := &domain.Recipe{
		ID:          globalID,
		HouseholdID: nil,
		Publisher:   publisher,
		Author:      author,
		Taxonomies:  []*domain.Taxonomy{taxonomy},
		Equipment:   []*domain.Equipment{equipment},
	}

	var cloned *domain.Recipe
	repo := &stubRecipeRepo{
		byIDFn:        func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					r.ID = uuid.New()
					cloned = r
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			})
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: globalID}, uid, myHID)

	require.NoError(t, err)
	require.NotNil(t, cloned)

	// Verify shared associations
	assert.Equal(t, publisher, cloned.Publisher)
	assert.Equal(t, author, cloned.Author)
	assert.Equal(t, global.Taxonomies, cloned.Taxonomies)
	assert.Equal(t, global.Equipment, cloned.Equipment)
}

func TestRecipeService_Update_GlobalRecipe_ClearsSpecificFields(t *testing.T) {
	globalID := uuid.New()
	myHID := uuid.New()
	uid := uuid.New()

	global := &domain.Recipe{
		ID:          globalID,
		HouseholdID: nil,
		Images:      []*domain.Image{{ID: uuid.New()}},
		Collections: []*domain.Collection{{ID: uuid.New()}},
		Feed:        &domain.Feed{ID: uuid.New()},
	}

	var cloned *domain.Recipe
	repo := &stubRecipeRepo{
		byIDFn:        func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					r.ID = uuid.New()
					cloned = r
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			})
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: globalID}, uid, myHID)

	require.NoError(t, err)
	require.NotNil(t, cloned)

	// Verify cleared fields
	assert.Nil(t, cloned.Images, "images must be cleared on clone")
	assert.Nil(t, cloned.Collections, "collections must be cleared on clone")
	assert.Nil(t, cloned.Feed, "feed must be cleared on clone")
}

func TestRecipeService_Update_GlobalRecipe_TransactionFailure(t *testing.T) {
	globalID := uuid.New()
	myHID := uuid.New()
	uid := uuid.New()
	global := &domain.Recipe{ID: globalID, HouseholdID: nil}

	expectedErr := sentinels.BadRequest("migration failed")

	repo := &stubRecipeRepo{
		byIDFn:        func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			return fn(&stubRecipeRepo{
				createFn:                func(_ *domain.Recipe) error { return nil },
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return expectedErr },
			})
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: globalID}, uid, myHID)

	assert.ErrorIs(t, err, expectedErr)
}

func TestRecipeService_EstimatePrice_CalculatesTotal(t *testing.T) {
	hid := uuid.New()
	foodID1 := uuid.New()
	foodID2 := uuid.New()
	unitID := uuid.New()

	recipe := &domain.Recipe{
		ID:    uuid.New(),
		Yield: new(4),
		Ingredients: []*domain.RecipeIngredient{
			{FoodID: &foodID1, Amount: new(200.0), UnitID: &unitID},
			{FoodID: &foodID2, Amount: new(500.0), UnitID: &unitID},
		},
	}

	repo := &stubRecipeRepo{
		byIDPreloadFn: func(_ uuid.UUID, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			return recipe, nil
		},
	}

	latestPrices := map[uuid.UUID]*domain.FoodPrice{
		foodID1: {FoodID: foodID1, Amount: 1000, Price: 10, UnitID: unitID},
		foodID2: {FoodID: foodID2, Amount: 1000, Price: 20, UnitID: unitID},
	}

	foodSvc := &stubFoodService{
		latestPricesFn: func(h uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
			assert.Equal(t, hid, h)
			return latestPrices, nil
		},
	}

	unitSvc := &stubUnitService{
		convertFn: func(amount float64, from, to *uuid.UUID, _ []domain.FoodUnitConversion) (float64, error) {
			assert.Equal(t, &unitID, from)
			assert.Equal(t, &unitID, to)
			return amount, nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, foodService: foodSvc, unitService: unitSvc})
	estimate, err := svc.EstimatePrice(recipe.ID, hid)

	require.NoError(t, err)
	assert.InDelta(t, 12.0, estimate.Total, 0.01) // 2.0 + 10.0
	require.NotNil(t, estimate.PerServing)
	assert.InDelta(t, 3.0, *estimate.PerServing, 0.01)
}

func TestRecipeService_EstimatePrice_ConversionErrors(t *testing.T) {
	hid, foodID, g, ml := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), Ingredients: []*domain.RecipeIngredient{{FoodID: &foodID, Amount: new(100.0), UnitID: &ml}}}
	repo := &stubRecipeRepo{byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) { return recipe, nil }}
	foodSvc := &stubFoodService{latestPricesFn: func(uuid.UUID, []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
		return map[uuid.UUID]*domain.FoodPrice{foodID: {FoodID: foodID, Amount: 1000, Price: 5, UnitID: g}}, nil
	}}

	t.Run("incompatible units leave the ingredient unpriced", func(t *testing.T) {
		units := &stubUnitService{convertFn: func(float64, *uuid.UUID, *uuid.UUID, []domain.FoodUnitConversion) (float64, error) {
			return 0, sentinels.Unprocessable("mass and volume")
		}}
		svc := newTestRecipeService(recipeServiceDeps{repo: repo, foodService: foodSvc, unitService: units})

		estimate, err := svc.EstimatePrice(recipe.ID, hid)

		require.NoError(t, err)
		require.Len(t, estimate.Items, 1)
		assert.Equal(t, "incompatible_unit", estimate.Items[0].Status)
	})

	t.Run("a lookup failure is returned, not reported as a unit mismatch", func(t *testing.T) {
		units := &stubUnitService{convertFn: func(float64, *uuid.UUID, *uuid.UUID, []domain.FoodUnitConversion) (float64, error) {
			return 0, sentinels.ErrNotFound
		}}
		svc := newTestRecipeService(recipeServiceDeps{repo: repo, foodService: foodSvc, unitService: units})

		_, err := svc.EstimatePrice(recipe.ID, hid)

		require.ErrorIs(t, err, sentinels.ErrNotFound)
	})
}

func TestRecipeService_EstimatePrice_UsesPreloadedUnits(t *testing.T) {
	flour, sugar, egg := uuid.New(), uuid.New(), uuid.New()
	mass := massFixtures()
	g, kg, pc := mass[0], mass[1], pieceFixture()
	unitRepo := newFakeUnitRepo(g, kg, pc)

	// As the repositories load them: ingredients and prices come with their Unit.
	recipe := &domain.Recipe{
		ID: uuid.New(),
		Ingredients: []*domain.RecipeIngredient{
			{FoodID: &flour, Amount: new(0.5), UnitID: &kg.ID, Unit: kg},
			{FoodID: &sugar, Amount: new(100.0), UnitID: &g.ID, Unit: g},
			{FoodID: &egg, Amount: new(2.0)},
		},
	}
	repo := &stubRecipeRepo{
		byIDPreloadFn: func(_ uuid.UUID, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			return recipe, nil
		},
	}
	foodSvc := &stubFoodService{
		latestPricesFn: func(uuid.UUID, []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
			return map[uuid.UUID]*domain.FoodPrice{
				flour: {FoodID: flour, Amount: 1000, Price: 2, UnitID: g.ID, Unit: g},
				sugar: {FoodID: sugar, Amount: 1, Price: 3, UnitID: kg.ID, Unit: kg},
				egg:   {FoodID: egg, Amount: 10, Price: 4, UnitID: pc.ID, Unit: pc},
			}, nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, foodService: foodSvc, unitService: services.NewUnitService(unitRepo)})
	estimate, err := svc.EstimatePrice(recipe.ID, uuid.New())

	require.NoError(t, err)
	assert.InDelta(t, 1.0+0.3+0.8, estimate.Total, 1e-9)
	assert.Zero(t, unitRepo.byIDsCalls, "preloaded ingredient and price units must not be fetched again")
	assert.Zero(t, unitRepo.bySlugCalls, "the piece unit arrives with the egg price and must not be looked up")
}

func TestRecipeService_Create_CanonicalizesIngredientAliases(t *testing.T) {
	aliasUnit, canonicalUnit := uuid.New(), uuid.New()
	aliasFood, canonicalFood := uuid.New(), uuid.New()

	var stored *domain.Recipe
	repo := &stubRecipeRepo{
		createFn: func(r *domain.Recipe) error {
			stored = r
			return nil
		},
		userSaveFn: func(uuid.UUID, uuid.UUID, uuid.UUID) error { return nil },
	}
	svc := newTestRecipeService(recipeServiceDeps{
		repo:        repo,
		unitService: &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{aliasUnit: canonicalUnit}},
		foodService: &stubFoodService{aliases: map[uuid.UUID]uuid.UUID{aliasFood: canonicalFood}},
	})

	clientUnit, clientFood := aliasUnit, aliasFood // rewritten in place
	recipe := &domain.Recipe{Ingredients: []*domain.RecipeIngredient{{UnitID: &clientUnit, FoodID: &clientFood}, {}}}
	require.NoError(t, svc.Create(t.Context(), recipe, uuid.New(), uuid.New()))

	require.NotNil(t, stored)
	assert.Equal(t, canonicalUnit, *stored.Ingredients[0].UnitID, "a merged-away unit must be stored as its canonical unit")
	assert.Equal(t, canonicalFood, *stored.Ingredients[0].FoodID, "a merged-away food must be stored as its canonical food")
	assert.Nil(t, stored.Ingredients[1].UnitID)
}

func TestRecipeService_Create_ClientIDsOnNestedRows_AreReplaced(t *testing.T) {
	var stored *domain.Recipe
	repo := &stubRecipeRepo{
		createFn: func(r *domain.Recipe) error {
			stored = r
			return nil
		},
		userSaveFn: func(uuid.UUID, uuid.UUID, uuid.UUID) error { return nil },
	}
	svc := newTestRecipeService(recipeServiceDeps{repo: repo, unitService: &stubUnitService{}, foodService: &stubFoodService{}})

	// IDs of rows that belong to another recipe.
	parentID := uuid.New()
	recipe := &domain.Recipe{
		Images:       []*domain.Image{{ID: uuid.New()}},
		Ingredients:  []*domain.RecipeIngredient{{ID: uuid.New(), RawText: "salt"}},
		Instructions: []*domain.RecipeInstruction{{ID: uuid.New(), ParentID: &parentID, Text: "stir", Images: []*domain.Image{{ID: uuid.New()}}}},
		Collections:  []*domain.Collection{{ID: uuid.New()}},
	}
	require.NoError(t, svc.Create(t.Context(), recipe, uuid.New(), uuid.New()))

	require.NotNil(t, stored)
	assert.Equal(t, uuid.Nil, stored.Images[0].ID)
	assert.Equal(t, uuid.Nil, stored.Ingredients[0].ID)
	assert.Equal(t, uuid.Nil, stored.Instructions[0].ID)
	assert.Nil(t, stored.Instructions[0].ParentID)
	assert.Equal(t, uuid.Nil, stored.Instructions[0].Images[0].ID)
	assert.Nil(t, stored.Collections, "collections must be added via their own endpoints")
}

func TestRecipeService_Create_NestedFoodAndUnitByName_AreFoundOrCreated(t *testing.T) {
	foodID, unitID := uuid.New(), uuid.New()
	var stored *domain.Recipe
	repo := &stubRecipeRepo{
		createFn: func(r *domain.Recipe) error {
			stored = r
			return nil
		},
		userSaveFn: func(uuid.UUID, uuid.UUID, uuid.UUID) error { return nil },
	}
	var lookedUpFood *domain.Food
	unitLookups := 0
	svc := newTestRecipeService(recipeServiceDeps{
		repo: repo,
		unitService: &stubUnitService{findOrCreateFn: func(u *domain.Unit) error {
			unitLookups++
			u.ID = unitID
			return nil
		}},
		foodService: &stubFoodService{findOrCreateFn: func(_ context.Context, f *domain.Food) error {
			lookedUpFood = f
			f.ID = foodID
			return nil
		}},
	})

	ing := &domain.RecipeIngredient{
		RawText: "2 cloves shallot",
		Food:    &domain.Food{Name: "Shallot", Images: []*domain.Image{{SourceURL: "http://internal.example/"}}},
		Unit:    &domain.Unit{Name: "clove"},
	}
	other := &domain.RecipeIngredient{RawText: "1 clove garlic", Unit: &domain.Unit{Name: "Clove"}}
	require.NoError(t, svc.Create(t.Context(), &domain.Recipe{Ingredients: []*domain.RecipeIngredient{ing, other}}, uuid.New(), uuid.New()))

	require.NotNil(t, stored)
	require.NotNil(t, stored.Ingredients[0].FoodID)
	require.NotNil(t, stored.Ingredients[0].UnitID)
	assert.Equal(t, foodID, *stored.Ingredients[0].FoodID, "a nested food is linked by name")
	assert.Equal(t, unitID, *stored.Ingredients[0].UnitID)
	assert.Equal(t, unitID, *stored.Ingredients[1].UnitID)
	assert.Equal(t, 1, unitLookups, "a unit named twice is looked up once")
	require.NotNil(t, lookedUpFood)
	assert.Empty(t, lookedUpFood.Images, "client images must not be passed on")
}

func TestRecipeService_CreateIngredient_NestedFoodWithoutIDOrName_IsRejected(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	repo := &stubRecipeRepo{byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}
	svc := newTestRecipeService(recipeServiceDeps{repo: repo, unitService: &stubUnitService{}, foodService: &stubFoodService{}})

	err := svc.CreateIngredient(t.Context(), &domain.RecipeIngredient{RecipeID: recipe.ID, Food: &domain.Food{}}, hid)

	var sentinelErr *sentinels.Error
	require.ErrorAs(t, err, &sentinelErr)
	assert.Equal(t, fiber.StatusBadRequest, sentinelErr.Status)
}

func TestRecipeService_UpdateIngredient_CanonicalizesAliases(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	aliasUnit, canonicalUnit := uuid.New(), uuid.New()

	var stored *domain.RecipeIngredient
	repo := &stubRecipeRepo{
		byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		updateIngredientFn: func(i *domain.RecipeIngredient) error {
			stored = i
			return nil
		},
	}
	svc := newTestRecipeService(recipeServiceDeps{
		repo:        repo,
		unitService: &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{aliasUnit: canonicalUnit}},
	})

	clientUnit := aliasUnit // rewritten in place
	err := svc.UpdateIngredient(t.Context(), &domain.RecipeIngredient{ID: uuid.New(), RecipeID: recipe.ID, UnitID: &clientUnit}, hid)

	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, canonicalUnit, *stored.UnitID)
}

func TestRecipeService_CreateIngredient_NestedAliasUnit_StoredAsCanonicalID(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	aliasUnit, canonicalUnit := uuid.New(), uuid.New()

	var stored *domain.RecipeIngredient
	repo := &stubRecipeRepo{
		byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		createIngredientFn: func(i *domain.RecipeIngredient) error {
			stored = i
			return nil
		},
	}
	svc := newTestRecipeService(recipeServiceDeps{
		repo:        repo,
		unitService: &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{aliasUnit: canonicalUnit}},
	})

	// A cached payload carrying the nested unit object instead of unit_id.
	err := svc.CreateIngredient(t.Context(), &domain.RecipeIngredient{RecipeID: recipe.ID, Unit: &domain.Unit{ID: aliasUnit}}, hid)

	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.UnitID)
	assert.Equal(t, canonicalUnit, *stored.UnitID)
	assert.Nil(t, stored.Unit, "a nested unit would be saved by GORM and overwrite unit_id")
}

func TestRecipeService_Delete_OwnedByHousehold_DeletesImagesAndRepoRecord(t *testing.T) {
	hid := uuid.New()
	recipeID := uuid.New()
	imageID := uuid.New()

	recipe := &domain.Recipe{
		ID:          recipeID,
		HouseholdID: &hid,
		Images:      []*domain.Image{{ID: imageID}},
	}

	repoDeleted := false
	imageDeleted := false

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(id uuid.UUID) error {
			repoDeleted = true
			assert.Equal(t, recipeID, id)
			return nil
		},
	}

	imgSvc := &stubImageService{
		deleteFn: func(id uuid.UUID) error {
			imageDeleted = true
			assert.Equal(t, imageID, id)
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, imgService: imgSvc})
	err := svc.Delete(recipeID, hid)

	require.NoError(t, err)
	assert.True(t, repoDeleted)
	assert.True(t, imageDeleted, "images of a household recipe must be deleted from storage")
}

func TestRecipeService_Delete_GlobalRecipeClone_DoesNotDeleteSharedImages(t *testing.T) {
	// When deleting a household's clone of a global recipe, we must NOT delete
	// the images because they are shared with the global recipe and potentially other clones.
	hid := uuid.New()
	recipeID := uuid.New()
	parentID := uuid.New()
	imageID := uuid.New()

	recipe := &domain.Recipe{
		ID:          recipeID,
		ParentID:    &parentID, // it's a clone
		HouseholdID: &hid,
		Images:      []*domain.Image{{ID: imageID}},
	}

	imageDeleted := false
	repo := &stubRecipeRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}
	imgSvc := &stubImageService{
		deleteFn: func(_ uuid.UUID) error {
			imageDeleted = true
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, imgService: imgSvc})
	err := svc.Delete(recipeID, hid)

	require.NoError(t, err)
	assert.False(t, imageDeleted, "shared images must not be deleted when a clone is removed")
}

func TestRecipeService_CreateInstruction_ParentFromAnotherRecipe_IsRejected(t *testing.T) {
	hid := uuid.New()
	sibling := &domain.RecipeInstruction{ID: uuid.New()}
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	var created *domain.RecipeInstruction
	repo := &stubRecipeRepo{
		byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		instructionParentsFn: func(uuid.UUID) (map[uuid.UUID]*uuid.UUID, error) {
			return map[uuid.UUID]*uuid.UUID{sibling.ID: nil}, nil
		},
		createInstructionFn: func(i *domain.RecipeInstruction) error {
			created = i
			return nil
		},
	}
	svc := newTestRecipeService(recipeServiceDeps{repo: repo})

	foreign := uuid.New() // an instruction of another household's recipe
	err := svc.CreateInstruction(&domain.RecipeInstruction{RecipeID: recipe.ID, ParentID: &foreign, Text: "x"}, hid)
	var sentinelErr *sentinels.Error
	require.ErrorAs(t, err, &sentinelErr)
	assert.Equal(t, fiber.StatusBadRequest, sentinelErr.Status)
	assert.Nil(t, created)

	require.NoError(t, svc.CreateInstruction(&domain.RecipeInstruction{RecipeID: recipe.ID, ParentID: &sibling.ID, Text: "y"}, hid))
	require.NotNil(t, created, "a sibling of the same recipe is a valid parent")
}

func TestRecipeService_UpdateInstruction_ParentLoop_IsRejected(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	repo := &stubRecipeRepo{
		byIDFn: func(uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		instructionParentsFn: func(uuid.UUID) (map[uuid.UUID]*uuid.UUID, error) {
			return map[uuid.UUID]*uuid.UUID{a: &b, b: &c, c: nil}, nil // a -> b -> c
		},
		updateInstructionFn: func(*domain.RecipeInstruction) error { return nil },
	}
	svc := newTestRecipeService(recipeServiceDeps{repo: repo})

	err := svc.UpdateInstruction(&domain.RecipeInstruction{ID: c, RecipeID: recipe.ID, ParentID: &a, Text: "x"}, hid)
	var sentinelErr *sentinels.Error
	require.ErrorAs(t, err, &sentinelErr, "c under a would close the loop a -> b -> c -> a")
	assert.Equal(t, fiber.StatusBadRequest, sentinelErr.Status)

	require.NoError(t, svc.UpdateInstruction(&domain.RecipeInstruction{ID: a, RecipeID: recipe.ID, ParentID: &c, Text: "x"}, hid))
}
