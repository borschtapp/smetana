package services_test

import (
	"testing"

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

func TestRecipeService_Search_FiltersByHousehold(t *testing.T) {
	hid := uuid.New()
	repo := &stubRecipeRepo{
		searchFn: func(_ uuid.UUID, h uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			assert.Equal(t, hid, h)
			return []domain.Recipe{{ID: uuid.New()}}, 1, nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	_, _, err := svc.Search(uuid.New(), hid, domain.RecipeSearchOptions{})

	require.NoError(t, err)
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
	global := &domain.Recipe{ID: globalID, HouseholdID: nil, Name: ptr("Global")}

	var clonedID uuid.UUID
	createCalled := false
	replaceCalled := false

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
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
	patch := &domain.Recipe{ID: globalID, Name: ptr("My Custom Version")}
	err := svc.Update(patch, uid, myHID)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, replaceCalled)
}

func TestRecipeService_EstimatePrice_CalculatesTotal(t *testing.T) {
	hid := uuid.New()
	foodID1 := uuid.New()
	foodID2 := uuid.New()
	unitID := uuid.New()

	recipe := &domain.Recipe{
		ID:    uuid.New(),
		Yield: ptr(4),
		Ingredients: []*domain.RecipeIngredient{
			{FoodID: &foodID1, Amount: ptr(200.0), UnitID: &unitID},
			{FoodID: &foodID2, Amount: ptr(500.0), UnitID: &unitID},
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
		convertFn: func(amount float64, from, to uuid.UUID) (float64, error) {
			assert.Equal(t, unitID, from)
			assert.Equal(t, unitID, to)
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
