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

func TestRecipeIngestService_ImportRecipe_ResolvesUnits(t *testing.T) {
	tests := []struct {
		name      string
		units     []string
		wantCalls int
	}{
		{name: "single", units: []string{"cup"}, wantCalls: 1},
		{name: "duplicates resolve once", units: []string{"tbsp", "tbsp", "tbsp"}, wantCalls: 1},
		// slugifies to "", so it is keyed by name
		{name: "symbol only", units: []string{"%"}, wantCalls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assignedUnitID := uuid.New()
			calls := 0
			unitService := &stubUnitService{
				findOrCreateFn: func(u *domain.Unit) error {
					calls++
					assert.Equal(t, tt.units[0], u.Name)
					u.ID = assignedUnitID
					return nil
				},
			}
			recipe := &domain.Recipe{}
			for _, name := range tt.units {
				recipe.Ingredients = append(recipe.Ingredients, &domain.RecipeIngredient{Unit: &domain.Unit{Name: name}})
			}

			svc := newTestRecipeIngestService(recipeIngestDeps{unitService: unitService})
			result, err := svc.ImportRecipe(context.Background(), recipe)

			require.NoError(t, err)
			assert.Equal(t, tt.wantCalls, calls)
			require.Len(t, result.Ingredients, len(tt.units))
			for _, ing := range result.Ingredients {
				require.NotNil(t, ing.UnitID)
				assert.Equal(t, assignedUnitID, *ing.UnitID)
			}
		})
	}
}

// FindOrCreate may replace the row in place (alias → canonical); every duplicate must still be linked.
func TestRecipeIngestService_ImportRecipe_LinksWhenResolveRewritesSlug(t *testing.T) {
	canonicalUnitID := uuid.New()
	canonicalFoodID := uuid.New()
	unitService := &stubUnitService{
		findOrCreateFn: func(u *domain.Unit) error {
			*u = domain.Unit{ID: canonicalUnitID, Name: "tablespoon", Slug: "tablespoon"}
			return nil
		},
	}
	foodService := &stubFoodService{
		findOrCreateFn: func(_ context.Context, f *domain.Food) error {
			*f = domain.Food{ID: canonicalFoodID, Name: "beetroot", Slug: "beetroot"}
			return nil
		},
	}
	recipe := &domain.Recipe{
		Ingredients: []*domain.RecipeIngredient{
			{Unit: &domain.Unit{Name: "tbsp"}, Food: &domain.Food{Name: "beet", Slug: "beet"}},
			{Unit: &domain.Unit{Name: "tbsp"}, Food: &domain.Food{Name: "beet", Slug: "beet"}},
		},
	}
	svc := newTestRecipeIngestService(recipeIngestDeps{unitService: unitService, foodService: foodService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	for _, ing := range result.Ingredients {
		require.NotNil(t, ing.UnitID)
		assert.Equal(t, canonicalUnitID, *ing.UnitID)
		require.NotNil(t, ing.FoodID)
		assert.Equal(t, canonicalFoodID, *ing.FoodID)
	}
}

func TestRecipeIngestService_ImportRecipe_IncompleteUpdate_RejectedBeforeImages(t *testing.T) {
	url := "https://example.com/paywalled"
	recipeService := &stubRecipeService{
		importTargetFn: func(_ uuid.UUID, _ *string) (*domain.Recipe, error) {
			return &domain.Recipe{
				ID:           uuid.New(),
				Ingredients:  []*domain.RecipeIngredient{{}},
				Instructions: []*domain.RecipeInstruction{{}},
			}, nil
		},
		importFn: func(*domain.Recipe) error {
			t.Fatal("an incomplete update must not reach the repository")
			return nil
		},
	}
	imageService := &stubImageService{
		persistRemoteFn: func(context.Context, *domain.Image, string) error {
			t.Fatal("images must not be downloaded for a rejected update")
			return nil
		},
	}
	recipe := &domain.Recipe{
		SourceUrl:   &url,
		Images:      []*domain.Image{{SourceURL: "https://example.com/hero.jpg"}},
		Ingredients: []*domain.RecipeIngredient{{RawText: "2 beets"}},
	}

	svc := newTestRecipeIngestService(recipeIngestDeps{recipeService: recipeService, imageService: imageService})
	_, err := svc.ImportRecipe(context.Background(), recipe)

	require.ErrorIs(t, err, sentinels.ErrIncompleteScrape)
}

// A recipe stored without instructions can still be refreshed.
func TestRecipeIngestService_ImportRecipe_UpdateOfIncompleteRecipeAllowed(t *testing.T) {
	url := "https://example.com/assembly"
	imported := false
	recipeService := &stubRecipeService{
		importTargetFn: func(_ uuid.UUID, _ *string) (*domain.Recipe, error) {
			return &domain.Recipe{ID: uuid.New(), Ingredients: []*domain.RecipeIngredient{{}}}, nil
		},
		importFn: func(*domain.Recipe) error {
			imported = true
			return nil
		},
	}
	recipe := &domain.Recipe{SourceUrl: &url, Ingredients: []*domain.RecipeIngredient{{RawText: "1 bun"}}}

	svc := newTestRecipeIngestService(recipeIngestDeps{recipeService: recipeService})
	_, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.True(t, imported)
}

// A forced re-import has no ID; its images must be stored under the existing recipe's ID.
func TestRecipeIngestService_ImportRecipe_ReimportPersistsImagesUnderExistingID(t *testing.T) {
	url := "https://example.com/borscht"
	existingID := uuid.New()
	recipeService := &stubRecipeService{
		importTargetFn: func(id uuid.UUID, sourceUrl *string) (*domain.Recipe, error) {
			assert.Equal(t, uuid.Nil, id)
			assert.Equal(t, url, *sourceUrl)
			return &domain.Recipe{ID: existingID}, nil
		},
	}
	var entityID uuid.UUID
	imageService := &stubImageService{
		persistRemoteFn: func(_ context.Context, img *domain.Image, _ string) error {
			entityID = img.EntityID
			return nil
		},
	}
	recipe := &domain.Recipe{SourceUrl: &url, Images: []*domain.Image{{SourceURL: "https://example.com/hero.jpg"}}}

	svc := newTestRecipeIngestService(recipeIngestDeps{recipeService: recipeService, imageService: imageService})
	result, err := svc.ImportRecipe(context.Background(), recipe)

	require.NoError(t, err)
	assert.Equal(t, existingID, result.ID)
	assert.Equal(t, existingID, entityID)
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
