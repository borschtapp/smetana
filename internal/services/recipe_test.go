package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/storage"
)

// newTestRecipeService builds a RecipeService wired up with the provided stubs.
func newTestRecipeService(repo *stubRecipeRepo, userRepo *stubUserRepo, imgSvc *stubImageService, pubSvc *stubPublisherService, foodRepo *stubFoodRepo, unitRepo *stubUnitRepo, scraperSvc *stubScraperService) domain.RecipeService {
	return services.NewRecipeService(repo, userRepo, imgSvc, pubSvc, foodRepo, unitRepo, scraperSvc)
}

func TestRecipeService_ByID_GlobalRecipe_AnyHouseholdCanRead(t *testing.T) {
	globalRecipe := &domain.Recipe{ID: uuid.New(), HouseholdID: nil}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return globalRecipe, nil }}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	got, err := svc.ByID(globalRecipe.ID, uuid.New()) // any household

	require.NoError(t, err)
	assert.Equal(t, globalRecipe.ID, got.ID)
}

func TestRecipeService_ByID_HouseholdRecipe_SameHouseholdCanRead(t *testing.T) {
	hid := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &hid}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	got, err := svc.ByID(recipe.ID, hid)

	require.NoError(t, err)
	assert.Equal(t, recipe.ID, got.ID)
}

func TestRecipeService_ByID_HouseholdRecipe_OtherHouseholdForbidden(t *testing.T) {
	ownerHID := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &ownerHID}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	_, err := svc.ByID(recipe.ID, uuid.New()) // different household

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_ByID_NotFound(t *testing.T) {
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) {
		return nil, sentinels.ErrRecordNotFound
	}}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	_, err := svc.ByID(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
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

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	err := svc.Delete(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Delete_DifferentHousehold_Forbidden(t *testing.T) {
	ownerHID := uuid.New()
	recipe := &domain.Recipe{ID: uuid.New(), HouseholdID: &ownerHID}
	repo := &stubRecipeRepo{byIDFn: func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil }}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	err := svc.Delete(recipe.ID, uuid.New()) // requester is from a different household

	require.ErrorIs(t, err, sentinels.ErrForbidden)
}

func TestRecipeService_Delete_OriginalHouseholdRecipe_DeletesImages(t *testing.T) {
	// An original (no ParentID) household recipe must have its images deleted.
	hid := uuid.New()
	imgPath := storage.Path("recipe/abc/img1.jpg")
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		HouseholdID: &hid,
		ParentID:    nil, // original, not a clone
		Images:      []*domain.RecipeImage{{DownloadUrl: &imgPath}},
	}

	deletedPaths := make([]storage.Path, 0)
	imgSvc := &stubImageService{
		deleteImageFn: func(p storage.Path) error {
			deletedPaths = append(deletedPaths, p)
			return nil
		},
	}
	repo := &stubRecipeRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, imgSvc, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	err := svc.Delete(recipe.ID, hid)

	require.NoError(t, err)
	require.Len(t, deletedPaths, 1)
	assert.Equal(t, imgPath, deletedPaths[0])
}

func TestRecipeService_Delete_ClonedRecipe_SkipsImageDeletion(t *testing.T) {
	// A cloned recipe (ParentID set) shares images with the global original —
	// deleting should NOT delete storage files.
	hid := uuid.New()
	parentID := uuid.New()
	imgPath := storage.Path("recipe/abc/shared.jpg")
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		HouseholdID: &hid,
		ParentID:    &parentID, // clone: do not delete shared images
		Images:      []*domain.RecipeImage{{DownloadUrl: &imgPath}},
	}

	deleteImageCalled := false
	imgSvc := &stubImageService{
		deleteImageFn: func(_ storage.Path) error {
			deleteImageCalled = true
			return nil
		},
	}
	repo := &stubRecipeRepo{
		byIDFn:   func(_ uuid.UUID) (*domain.Recipe, error) { return recipe, nil },
		deleteFn: func(_ uuid.UUID) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, imgSvc, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, &stubScraperService{})
	err := svc.Delete(recipe.ID, hid)

	require.NoError(t, err)
	assert.False(t, deleteImageCalled, "images shared with global recipe must not be deleted on clone removal")
}

func TestRecipeService_ImportRecipe_ResolvesFood(t *testing.T) {
	// food.ID must be populated via FindOrCreate and wired to ing.FoodID.
	assignedFoodID := uuid.New()
	food := &domain.Food{Name: "potato"}

	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{
			{Food: food},
		},
	}

	foodRepo := &stubFoodRepo{
		findOrCreateFn: func(f *domain.Food) error {
			f.ID = assignedFoodID
			return nil
		},
	}
	repo := &stubRecipeRepo{
		importFn:       func(_ *domain.Recipe) error { return nil },
		createImagesFn: func(_ []*domain.RecipeImage) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, foodRepo, &stubUnitRepo{}, &stubScraperService{})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Ingredients[0].FoodID)
	assert.Equal(t, assignedFoodID, *result.Ingredients[0].FoodID)
}

func TestRecipeService_ImportRecipe_ResolvesUnit(t *testing.T) {
	assignedUnitID := uuid.New()
	unit := &domain.Unit{Name: "cup"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{
			{Unit: unit},
		},
	}

	unitRepo := &stubUnitRepo{
		findOrCreateFn: func(u *domain.Unit) error {
			u.ID = assignedUnitID
			return nil
		},
	}
	repo := &stubRecipeRepo{
		importFn:       func(_ *domain.Recipe) error { return nil },
		createImagesFn: func(_ []*domain.RecipeImage) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, unitRepo, &stubScraperService{})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.Ingredients[0].UnitID)
	assert.Equal(t, assignedUnitID, *result.Ingredients[0].UnitID)
}

func TestRecipeService_ImportRecipe_FoodError_NilsFood(t *testing.T) {
	// When FindOrCreate for Food fails, the ingredient's Food field is nilled
	// out (not propagated as an error) so the import can proceed.
	food := &domain.Food{Name: "mystery"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Food: food}},
	}

	foodRepo := &stubFoodRepo{
		findOrCreateFn: func(_ *domain.Food) error { return errors.New("db error") },
	}
	repo := &stubRecipeRepo{
		importFn:       func(_ *domain.Recipe) error { return nil },
		createImagesFn: func(_ []*domain.RecipeImage) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, foodRepo, &stubUnitRepo{}, &stubScraperService{})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err, "Food resolution failure must not abort the import")
	assert.Nil(t, result.Ingredients[0].Food, "Food must be cleared when FindOrCreate fails")
	assert.Nil(t, result.Ingredients[0].FoodID)
}

func TestRecipeService_ImportFromURL_ExistingRecipe_SavesForUser(t *testing.T) {
	// When a recipe with the URL already exists, the service must save it for
	// the user and return it — no scraping should occur.
	existingID := uuid.New()
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/recipe/borsch"

	existing := &domain.Recipe{ID: existingID, IsBasedOn: &testURL}

	userSaveCalled := false
	repo := &stubRecipeRepo{
		byUrlFn: func(_ string) (*domain.Recipe, error) { return existing, nil },
		userSaveFn: func(rid, _, _ uuid.UUID) error {
			userSaveCalled = true
			assert.Equal(t, existingID, rid)
			return nil
		},
	}
	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ string) (*domain.Recipe, error) {
			t.Fatal("ScrapeRecipe must not be called when recipe already exists")
			return nil, nil
		},
	}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, scraper)
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	assert.Equal(t, existingID, got.ID)
	assert.True(t, userSaveCalled, "existing recipe must be saved for the user")
}

func TestRecipeService_ImportFromURL_NewRecipe_ScrapesAndImports(t *testing.T) {
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/new"
	scrapedName := "Fresh Recipe"
	importedID := uuid.New()

	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ string) (*domain.Recipe, error) {
			return &domain.Recipe{Name: &scrapedName, IsBasedOn: &testURL}, nil
		},
	}
	repo := &stubRecipeRepo{
		byUrlFn: func(_ string) (*domain.Recipe, error) { return nil, sentinels.ErrRecordNotFound },
		importFn: func(r *domain.Recipe) error {
			r.ID = importedID
			return nil
		},
		createImagesFn: func(_ []*domain.RecipeImage) error { return nil },
		userSaveFn:     func(_, _, _ uuid.UUID) error { return nil },
	}

	svc := newTestRecipeService(repo, nil, &stubImageService{}, &stubPublisherService{}, &stubFoodRepo{}, &stubUnitRepo{}, scraper)
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	assert.Equal(t, importedID, got.ID)
}
