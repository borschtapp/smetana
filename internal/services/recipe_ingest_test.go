package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/services"
)

type recipeIngestDeps struct {
	recipeService    domain.RecipeService
	imageService     domain.ImageService
	foodService      domain.FoodService
	unitService      domain.UnitService
	publisherService domain.PublisherService
	authorService    domain.AuthorService
	taxonomyService  domain.TaxonomyService
	equipmentService domain.EquipmentService
}

func newTestRecipeIngestService(deps recipeIngestDeps) domain.RecipeIngestService {
	if deps.recipeService == nil {
		deps.recipeService = &stubRecipeService{}
	}
	if deps.imageService == nil {
		deps.imageService = &stubImageService{}
	}
	if deps.foodService == nil {
		deps.foodService = &stubFoodService{}
	}
	if deps.unitService == nil {
		deps.unitService = &stubUnitService{}
	}
	if deps.publisherService == nil {
		deps.publisherService = &stubPublisherService{}
	}
	if deps.authorService == nil {
		deps.authorService = &stubAuthorService{}
	}
	if deps.taxonomyService == nil {
		deps.taxonomyService = &stubTaxonomyService{}
	}
	if deps.equipmentService == nil {
		deps.equipmentService = &stubEquipmentService{}
	}
	return services.NewRecipeIngestService(
		deps.recipeService,
		deps.imageService,
		deps.foodService,
		deps.unitService,
		deps.publisherService,
		deps.authorService,
		deps.taxonomyService,
		deps.equipmentService,
	)
}

func TestRecipeIngestService_ImportRecipe_ResolvesFood(t *testing.T) {
	assignedFoodID := uuid.New()
	food := &domain.Food{Name: "potato"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Food: food}},
	}
	foodService := &stubFoodService{
		findOrCreateFn: func(_ context.Context, f *domain.Food) error {
			f.ID = assignedFoodID
			return nil
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{foodService: foodService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.Ingredients[0].FoodID)
	assert.Equal(t, assignedFoodID, *result.Ingredients[0].FoodID)
}

func TestRecipeIngestService_ImportRecipe_ResolvesUnit(t *testing.T) {
	assignedUnitID := uuid.New()
	unit := &domain.Unit{Name: "cup"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Unit: unit}},
	}
	unitService := &stubUnitService{
		findOrCreateFn: func(u *domain.Unit) error {
			u.ID = assignedUnitID
			return nil
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{unitService: unitService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.Ingredients[0].UnitID)
	assert.Equal(t, assignedUnitID, *result.Ingredients[0].UnitID)
}

func TestRecipeIngestService_ImportRecipe_FoodError_NilsFood(t *testing.T) {
	food := &domain.Food{Name: "mystery"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Food: food}},
	}
	foodService := &stubFoodService{
		findOrCreateFn: func(_ context.Context, _ *domain.Food) error { return errors.New("db error") },
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{foodService: foodService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.Nil(t, result.Ingredients[0].Food)
}

func TestRecipeIngestService_ImportRecipe_ResolvesEquipment(t *testing.T) {
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
	svc := newTestRecipeIngestService(recipeIngestDeps{equipmentService: equipService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.Len(t, result.Equipment, 1)
	assert.Equal(t, assignedID, result.Equipment[0].ID)
}

func TestRecipeIngestService_ImportRecipe_ResolvesTaxonomy(t *testing.T) {
	taxID := uuid.New()
	tax := &domain.Taxonomy{Label: "Italian"}
	recipe := &domain.Recipe{
		Taxonomies: []*domain.Taxonomy{tax},
	}
	taxonomyService := &stubTaxonomyService{
		findOrCreateFn: func(t *domain.Taxonomy) error {
			t.ID = taxID
			return nil
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{taxonomyService: taxonomyService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.Len(t, result.Taxonomies, 1)
	assert.Equal(t, taxID, result.Taxonomies[0].ID)
}

func TestRecipeIngestService_ImportRecipe_ResolvesPublisher(t *testing.T) {
	pubID := uuid.New()
	publisher := &domain.Publisher{Name: "Food Network"}
	recipe := &domain.Recipe{Publisher: publisher}
	pubService := &stubPublisherService{
		findOrCreateFn: func(_ context.Context, p *domain.Publisher) error {
			p.ID = pubID
			return nil
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{publisherService: pubService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	require.NotNil(t, result.PublisherID)
	assert.Equal(t, pubID, *result.PublisherID)
}

func TestRecipeIngestService_ImportRecipe_EquipmentError_DropsItem(t *testing.T) {
	recipe := &domain.Recipe{
		Equipment: []*domain.Equipment{{Name: "Wok"}},
	}
	equipService := &stubEquipmentService{
		findOrCreateFn: func(_ context.Context, _ *domain.Equipment) error {
			return errors.New("db error")
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{equipmentService: equipService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.Empty(t, result.Equipment)
}

func TestRecipeIngestService_ImportRecipe_TaxonomyError_DropsItem(t *testing.T) {
	recipe := &domain.Recipe{
		Taxonomies: []*domain.Taxonomy{{Label: "Unknown"}},
	}
	taxService := &stubTaxonomyService{
		findOrCreateFn: func(_ *domain.Taxonomy) error { return errors.New("db error") },
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{taxonomyService: taxService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.Empty(t, result.Taxonomies)
}

func TestRecipeIngestService_ImportRecipe_UnitError_NilsUnit(t *testing.T) {
	unit := &domain.Unit{Name: "pinch"}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{{Unit: unit}},
	}
	unitService := &stubUnitService{
		findOrCreateFn: func(_ *domain.Unit) error { return errors.New("db error") },
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{unitService: unitService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.Nil(t, result.Ingredients[0].Unit)
	assert.Nil(t, result.Ingredients[0].UnitID)
}
