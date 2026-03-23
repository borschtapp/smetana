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
)

type importServiceDeps struct {
	recipeService    *stubRecipeService
	imgService       *stubImageService
	pubService       *stubPublisherService
	authorService    *stubAuthorService
	foodService      *stubFoodService
	unitService      *stubUnitService
	taxRepo          *stubTaxonomyRepo
	equipmentService *stubEquipmentService
	scraper          *stubScraperService
}

func newTestImportService(deps importServiceDeps) domain.ImportService {
	if deps.recipeService == nil {
		deps.recipeService = &stubRecipeService{}
	}
	if deps.imgService == nil {
		deps.imgService = &stubImageService{}
	}
	if deps.pubService == nil {
		deps.pubService = &stubPublisherService{}
	}
	if deps.authorService == nil {
		deps.authorService = &stubAuthorService{}
	}
	if deps.foodService == nil {
		deps.foodService = &stubFoodService{}
	}
	if deps.unitService == nil {
		deps.unitService = &stubUnitService{}
	}
	if deps.taxRepo == nil {
		deps.taxRepo = &stubTaxonomyRepo{}
	}
	if deps.equipmentService == nil {
		deps.equipmentService = &stubEquipmentService{}
	}
	if deps.scraper == nil {
		deps.scraper = &stubScraperService{}
	}
	return services.NewImportService(deps.recipeService, deps.imgService, deps.pubService, deps.authorService, deps.foodService, deps.unitService, deps.taxRepo, deps.equipmentService, deps.scraper)
}

func TestImportService_ImportRecipe_ResolvesFood(t *testing.T) {
	// food.ID must be populated via FindOrCreate and wired to ing.FoodID.
	assignedFoodID := uuid.New()
	food := &domain.Food{Name: "potato"}

	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{
			{Food: food},
		},
	}

	foodService := &stubFoodService{
		findOrCreateFn: func(_ context.Context, f *domain.Food) error {
			f.ID = assignedFoodID
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{foodService: foodService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Ingredients[0].FoodID)
	assert.Equal(t, assignedFoodID, *result.Ingredients[0].FoodID)
}

func TestImportService_ImportRecipe_ResolvesUnit(t *testing.T) {
	assignedUnitID := uuid.New()
	unit := &domain.Unit{Name: "cup"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{
			{Unit: unit},
		},
	}

	unitService := &stubUnitService{
		findOrCreateFn: func(u *domain.Unit) error {
			u.ID = assignedUnitID
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{unitService: unitService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.Ingredients[0].UnitID)
	assert.Equal(t, assignedUnitID, *result.Ingredients[0].UnitID)
}

func TestImportService_ImportRecipe_FoodError_NilsFood(t *testing.T) {
	// When FindOrCreate for Food fails, the ingredient's Food field is nilled
	// out (not propagated as an error) so the import can proceed.
	food := &domain.Food{Name: "mystery"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Food: food}},
	}

	foodService := &stubFoodService{
		findOrCreateFn: func(_ context.Context, _ *domain.Food) error { return errors.New("db error") },
	}

	svc := newTestImportService(importServiceDeps{foodService: foodService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err, "Food resolution failure must not abort the import")
	assert.Nil(t, result.Ingredients[0].Food, "Food must be cleared when FindOrCreate fails")
	assert.Nil(t, result.Ingredients[0].FoodID)
}

func TestImportService_ImportRecipe_ResolvesEquipment(t *testing.T) {
	// Equipment must be resolved via FindOrCreate. Only successfully resolved
	// equipment should remain in recipe.Equipment after import.
	assignedID := uuid.New()
	equip := &domain.Equipment{Name: "Dutch Oven"}

	recipe := &domain.Recipe{
		Equipment: []*domain.Equipment{equip},
	}

	equipService := &stubEquipmentService{
		findOrCreateFn: func(_ context.Context, e *domain.Equipment) error {
			e.ID = assignedID
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{equipmentService: equipService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.Len(t, result.Equipment, 1)
	assert.Equal(t, assignedID, result.Equipment[0].ID)
}

func TestImportService_ImportRecipe_EquipmentError_DropsItem(t *testing.T) {
	// When FindOrCreate fails for a piece of equipment, that item must be silently
	// dropped from the slice rather than aborting the whole import.
	recipe := &domain.Recipe{
		Equipment: []*domain.Equipment{{Name: "Wok"}},
	}

	equipService := &stubEquipmentService{
		findOrCreateFn: func(_ context.Context, _ *domain.Equipment) error {
			return errors.New("db error")
		},
	}

	svc := newTestImportService(importServiceDeps{equipmentService: equipService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err, "equipment resolution failure must not abort import")
	assert.Empty(t, result.Equipment, "failed equipment must be dropped")
}

func TestImportService_ImportRecipe_ResolvesTaxonomy(t *testing.T) {
	// Taxonomies must be resolved via FindOrCreate; only successfully resolved
	// ones are kept on the recipe.
	taxID := uuid.New()
	tax := &domain.Taxonomy{Label: "Italian"}

	recipe := &domain.Recipe{
		Taxonomies: []*domain.Taxonomy{tax},
	}

	taxRepo := &stubTaxonomyRepo{
		findOrCreateFn: func(t *domain.Taxonomy) error {
			t.ID = taxID
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{taxRepo: taxRepo})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.Len(t, result.Taxonomies, 1)
	assert.Equal(t, taxID, result.Taxonomies[0].ID)
}

func TestImportService_ImportRecipe_TaxonomyError_DropsItem(t *testing.T) {
	recipe := &domain.Recipe{
		Taxonomies: []*domain.Taxonomy{{Label: "Unknown"}},
	}

	taxRepo := &stubTaxonomyRepo{
		findOrCreateFn: func(_ *domain.Taxonomy) error { return errors.New("db error") },
	}

	svc := newTestImportService(importServiceDeps{taxRepo: taxRepo})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err, "taxonomy error must not abort import")
	assert.Empty(t, result.Taxonomies)
}

func TestImportService_ImportRecipe_ResolvesPublisher(t *testing.T) {
	pubID := uuid.New()
	publisher := &domain.Publisher{Name: "Food Network", Url: ptr("https://foodnetwork.com")}

	recipe := &domain.Recipe{
		Publisher: publisher,
	}

	pubService := &stubPublisherService{
		findOrCreateFn: func(_ context.Context, p *domain.Publisher) error {
			p.ID = pubID
			return nil
		},
	}

	svc := newTestImportService(importServiceDeps{pubService: pubService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.PublisherID)
	assert.Equal(t, pubID, *result.PublisherID)
}

func TestImportService_ImportRecipe_UnitError_NilsUnit(t *testing.T) {
	// When FindOrCreate for Unit fails, the ingredient's Unit field is nilled
	// so the import proceeds without an invalid FK reference.
	unit := &domain.Unit{Name: "pinch"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Unit: unit}},
	}

	unitService := &stubUnitService{
		findOrCreateFn: func(_ *domain.Unit) error { return errors.New("db error") },
	}

	svc := newTestImportService(importServiceDeps{unitService: unitService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err, "unit resolution failure must not abort import")
	assert.Nil(t, result.Ingredients[0].Unit)
	assert.Nil(t, result.Ingredients[0].UnitID)
}

func TestImportService_ImportFromURL_ExistingRecipe_SavesForUser(t *testing.T) {
	// When a recipe with the URL already exists, the service must save it for
	// the user and return it — no scraping should occur.
	rid := uuid.New()
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/recipe/borsch"

	existing := &domain.Recipe{ID: rid, SourceUrl: &testURL}

	userSaveCalled := false
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) { return existing, nil },
		userSaveFn: func(receivedRid, receivedUid, receivedHid uuid.UUID) error {
			userSaveCalled = true
			assert.Equal(t, rid, receivedRid)
			assert.Equal(t, uid, receivedUid)
			assert.Equal(t, hid, receivedHid)
			return nil
		},
	}
	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ context.Context, _ string) (*domain.Recipe, error) {
			t.Fatal("ScrapeRecipe must not be called when recipe already exists")
			return nil, nil
		},
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, scraper: scraper})
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	assert.Equal(t, rid, got.ID)
	assert.True(t, userSaveCalled, "existing recipe must be saved for the user")
}

func TestImportService_ImportFromURL_NewRecipe_ScrapesAndImports(t *testing.T) {
	uid := uuid.New()
	hid := uuid.New()
	testURL := "https://example.com/new"
	scrapedName := "Fresh Recipe"
	importedID := uuid.New()

	scraper := &stubScraperService{
		scrapeRecipeFn: func(_ context.Context, _ string) (*domain.Recipe, error) {
			return &domain.Recipe{Name: &scrapedName, SourceUrl: &testURL}, nil
		},
	}
	recipeSvc := &stubRecipeService{
		byUrlFn: func(_ string, _ uuid.UUID) (*domain.Recipe, error) { return nil, sentinels.ErrNotFound },
		importFn: func(r *domain.Recipe) error {
			r.ID = importedID
			return nil
		},
		userSaveFn: func(_, _, _ uuid.UUID) error { return nil },
	}

	svc := newTestImportService(importServiceDeps{recipeService: recipeSvc, scraper: scraper})
	got, err := svc.ImportFromURL(context.Background(), testURL, false, uid, hid)

	require.NoError(t, err)
	assert.Equal(t, importedID, got.ID)
}
