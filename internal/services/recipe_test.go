package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
)

type recipeServiceDeps struct {
	repo       *stubRecipeRepo
	userRepo   *stubUserRepo
	imgService *stubImageService
}

// newTestRecipeService builds a recipeService wired up with the provided stubs.
func newTestRecipeService(deps recipeServiceDeps) domain.RecipeService {
	if deps.imgService == nil {
		deps.imgService = &stubImageService{}
	}
	return services.NewRecipeService(deps.repo, deps.userRepo, deps.imgService)
}

func TestRecipeService_ByID_GlobalRecipe_AnyHouseholdCanRead(t *testing.T) {
	globalRecipe := &domain.Recipe{ID: uuid.New(), HouseholdID: nil}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return globalRecipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	got, err := svc.ByID(globalRecipe.ID, uuid.New()) // any household

	require.NoError(t, err)
	assert.Equal(t, globalRecipe.ID, got.ID)
}

func TestRecipeService_ByID_HouseholdRecipe_SameHouseholdCanRead(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	got, err := svc.ByID(recipe.ID, hid)

	require.NoError(t, err)
	assert.Equal(t, recipe.ID, got.ID)
}

func TestRecipeService_ByID_HouseholdRecipe_OtherHouseholdForbidden(t *testing.T) {
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: new(uuid.New())}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	_, err := svc.ByID(recipe.ID, uuid.New()) // different household

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_ByID_NotFound(t *testing.T) {
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) {
		return nil, sentinels.ErrNotFound
	}}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	_, err := svc.ByID(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestRecipeService_Update_GlobalRecipe_ClonesBeforeUpdate(t *testing.T) {
	// A global recipe (HouseholdID == nil) must be cloned into the household
	// before the update is applied. After Update returns, recipe.ID should
	// point to the new cloned recipe, not the original.
	globalID := uuid.New()
	hid := uuid.New()
	uid := uuid.New()

	global := &domain.Recipe{
		ID:          globalID,
		HouseholdID: nil,
		Name:        ptr("Global Borsch"),
		Ingredients: []*domain.RecipeIngredient{
			{ID: uuid.New(), RawText: "beet"},
		},
		Instructions: []*domain.RecipeInstruction{},
	}

	var clonedID uuid.UUID // captures the UUID assigned during Create

	repo := &stubRecipeRepo{
		byIDFn: func(id uuid.UUID) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			txRepo := &stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					// Simulate GORM assigning a new UUID in BeforeCreate
					r.ID, _ = uuid.NewV7()
					clonedID = r.ID
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			}
			return fn(txRepo)
		},
		updateFn: func(r *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	patch := &domain.Recipe{ID: globalID, Name: ptr("My Borsch")}
	err := svc.Update(patch, uid, hid)

	require.NoError(t, err)
	assert.Equal(t, clonedID, patch.ID, "recipe.ID should be updated to the clone's ID")
	assert.NotEqual(t, globalID, patch.ID, "original global ID must not be used for the update")
	require.NotNil(t, patch.ParentID)
	assert.Equal(t, globalID, *patch.ParentID)
}

func TestRecipeService_Update_GlobalRecipe_SetsHouseholdAndUser(t *testing.T) {
	hid := uuid.New()
	uid := uuid.New()
	global := &domain.Recipe{ID: uuid.New(), HouseholdID: nil}

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			tx := &stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					assert.Equal(t, hid, *r.HouseholdID, "clone must belong to requesting household")
					assert.Equal(t, uid, *r.UserID, "clone must be owned by requesting user")
					assert.Equal(t, global.ID, *r.ParentID, "clone must reference original as parent")
					r.ID, _ = uuid.NewV7()
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			}
			return fn(tx)
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: global.ID}, uid, hid)
	require.NoError(t, err)
}

func TestRecipeService_Update_GlobalRecipe_DeepCopiesIngredients(t *testing.T) {
	// Ingredient IDs must be zeroed so GORM generates new UUIDs for the clone.
	origIngID := uuid.New()
	global := &domain.Recipe{
		ID:          uuid.New(),
		HouseholdID: nil,
		Ingredients: []*domain.RecipeIngredient{
			{ID: origIngID, RawText: "salt"},
		},
		Instructions: []*domain.RecipeInstruction{},
	}

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			tx := &stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					require.Len(t, r.Ingredients, 1)
					assert.Equal(t, uuid.Nil, r.Ingredients[0].ID, "ingredient ID must be zeroed for clone")
					assert.Equal(t, uuid.Nil, r.Ingredients[0].RecipeID, "ingredient RecipeID must be zeroed")
					assert.Equal(t, "salt", r.Ingredients[0].RawText, "ingredient content must be preserved")
					r.ID, _ = uuid.NewV7()
					return nil
				},
				replaceRecipePointersFn: func(_, _, _ uuid.UUID) error { return nil },
			}
			return fn(tx)
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: global.ID}, uuid.New(), uuid.New())
	require.NoError(t, err)
}

func TestRecipeService_Update_GlobalRecipe_MigratesPointers(t *testing.T) {
	// ReplaceRecipePointers must be called inside the same transaction with
	// the original and new IDs so no dangling pointers are left.
	globalID := uuid.New()
	hid := uuid.New()
	var capturedOld, capturedNew, capturedHID uuid.UUID

	global := &domain.Recipe{ID: globalID, HouseholdID: nil, Instructions: []*domain.RecipeInstruction{}}

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return global, nil },
		transactionFn: func(fn func(domain.RecipeRepository) error) error {
			tx := &stubRecipeRepo{
				createFn: func(r *domain.Recipe) error {
					r.ID, _ = uuid.NewV7()
					return nil
				},
				replaceRecipePointersFn: func(old, newID, h uuid.UUID) error {
					capturedOld, capturedNew, capturedHID = old, newID, h
					return nil
				},
			}
			return fn(tx)
		},
		updateFn: func(_ *domain.Recipe) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	patch := &domain.Recipe{ID: globalID}
	err := svc.Update(patch, uuid.New(), hid)

	require.NoError(t, err)
	assert.Equal(t, globalID, capturedOld, "old recipe ID must be the global one")
	assert.Equal(t, patch.ID, capturedNew, "new recipe ID must be the clone")
	assert.Equal(t, hid, capturedHID, "household ID must match the requester")
}

func TestRecipeService_Update_HouseholdRecipe_UpdatesDirectly(t *testing.T) {
	// A recipe already owned by the household must NOT trigger cloning.
	hid := uuid.New()
	uid := uuid.New()
	rid := uuid.New()
	recipe := &domain.Recipe{ID: rid, HouseholdID: &hid}

	transactionCalled := false
	updateCalled := false

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		transactionFn: func(_ func(domain.RecipeRepository) error) error {
			transactionCalled = true
			return nil
		},
		updateFn: func(r *domain.Recipe) error {
			updateCalled = true
			assert.Equal(t, rid, r.ID, "ID must not change for household-owned recipe")
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Update(&domain.Recipe{ID: rid}, uid, hid)

	require.NoError(t, err)
	assert.False(t, transactionCalled, "no transaction should occur for already-household-owned recipe")
	assert.True(t, updateCalled)
}

func TestRecipeService_Delete_GlobalRecipe_Forbidden(t *testing.T) {
	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) {
			return &domain.Recipe{ID: uuid.New(), HouseholdID: nil}, nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Delete(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Delete_DifferentHousehold_Forbidden(t *testing.T) {
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: new(uuid.New())}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.Delete(recipe.ID, uuid.New()) // requester is from a different household

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Delete_OriginalHouseholdRecipe_DeletesImages(t *testing.T) {
	// An original (no ParentID) household recipe must have its images deleted.
	hid := uuid.New()
	imgID := uuid.New()
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		HouseholdID: &hid,
		ParentID:    nil, // original, not a clone
		Images:      []*domain.Image{{ID: imgID}},
	}

	deletedIDs := make([]uuid.UUID, 0)
	imgService := &stubImageService{
		deleteFn: func(id uuid.UUID) error {
			deletedIDs = append(deletedIDs, id)
			return nil
		},
	}
	repo := &stubRecipeRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, imgService: imgService})
	err := svc.Delete(recipe.ID, hid)

	require.NoError(t, err)
	require.Len(t, deletedIDs, 1)
	assert.Equal(t, imgID, deletedIDs[0])
}

func TestRecipeService_Delete_ClonedRecipe_SkipsImageDeletion(t *testing.T) {
	// A cloned recipe (ParentID set) shares images with the global original —
	// deleting should NOT delete storage files.
	hid := uuid.New()
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		HouseholdID: &hid,
		ParentID:    new(uuid.New()), // clone: do not delete shared images
		Images:      []*domain.Image{{ID: uuid.New()}},
	}

	deleteImageCalled := false
	imgService := &stubImageService{
		deleteFn: func(_ uuid.UUID) error {
			deleteImageCalled = true
			return nil
		},
	}
	repo := &stubRecipeRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo, imgService: imgService})
	err := svc.Delete(recipe.ID, hid)

	require.NoError(t, err)
	assert.False(t, deleteImageCalled, "images shared with global recipe must not be deleted on clone removal")
}

func TestRecipeService_Create_SetsHouseholdAndUserAndSaves(t *testing.T) {
	hid := uuid.New()
	uid := uuid.New()

	createCalled := false
	userSaveCalled := false

	repo := &stubRecipeRepo{
		createFn: func(r *domain.Recipe) error {
			createCalled = true
			assert.Equal(t, hid, *r.HouseholdID)
			assert.Equal(t, uid, *r.UserID)
			r.ID, _ = uuid.NewV7()
			return nil
		},
		userSaveFn: func(rid, u, h uuid.UUID) error {
			userSaveCalled = true
			assert.Equal(t, uid, u)
			assert.Equal(t, hid, h)
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	recipe := &domain.Recipe{}
	err := svc.Create(recipe, uid, hid)

	require.NoError(t, err)
	assert.True(t, createCalled)
	assert.True(t, userSaveCalled, "newly created recipe must be saved for the creator")
	assert.NotEqual(t, uuid.Nil, recipe.ID)
}

func TestRecipeService_AddEquipment_WrongHousehold_Forbidden(t *testing.T) {
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: new(uuid.New())}
	addCalled := false

	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
	}
	// We intentionally don't set addEquipmentFn — if it's reached, the test panics.
	_ = addCalled

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.AddEquipment(recipe.ID, uuid.New(), uuid.New() /* different household */)

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_AddEquipment_SameHousehold_DelegatestoRepo(t *testing.T) {
	hid := uuid.New()
	recipeID := uuid.New()
	equipmentID := uuid.New()
	recipe := &domain.Recipe{ID: recipeID, HouseholdID: &hid}

	addCalled := false
	repo := &stubRecipeRepo{
		byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		addEquipmentFn: func(rid, eid uuid.UUID) error {
			addCalled = true
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, equipmentID, eid)
			return nil
		},
	}

	svc := newTestRecipeService(recipeServiceDeps{repo: repo})
	err := svc.AddEquipment(recipeID, equipmentID, hid)

	require.NoError(t, err)
	assert.True(t, addCalled)
}
