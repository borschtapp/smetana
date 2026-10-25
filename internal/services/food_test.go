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

// fakeFoodRepo is a minimal no-op implementation of domain.FoodRepository for service-level tests.
type fakeFoodRepo struct {
	createConversionFn func(*domain.FoodUnitConversion) error
	canonicalIDs       map[uuid.UUID]uuid.UUID     // alias -> canonical, applied by ByID and CanonicalIDs
	listedFoodID       uuid.UUID                   // last ListConversions argument
	merged             [][2]uuid.UUID              // keep, merge pairs passed to Merge
	combine            domain.ShoppingItemCombiner // last combiner passed to Merge
	conversionByIDFn   func(foodID, id uuid.UUID) (*domain.FoodUnitConversion, error)
	updateConversionFn func(*domain.FoodUnitConversion) error
	byIDFn             func(uuid.UUID) (*domain.Food, error)
	createPriceFn      func(*domain.FoodPrice) error
	updateFn           func(*domain.Food) error
}

func (r *fakeFoodRepo) ByID(id uuid.UUID) (*domain.Food, error) {
	if c, ok := r.canonicalIDs[id]; ok {
		id = c
	}
	if r.byIDFn != nil {
		return r.byIDFn(id)
	}
	return nil, nil
}
func (r *fakeFoodRepo) ByIDs(_ []uuid.UUID) (map[uuid.UUID]*domain.Food, error) {
	return make(map[uuid.UUID]*domain.Food), nil
}
func (r *fakeFoodRepo) FindOrCreate(_ *domain.Food) error                       { return nil }
func (r *fakeFoodRepo) Search(_ string, _, _ int) ([]domain.Food, int64, error) { return nil, 0, nil }
func (r *fakeFoodRepo) Merge(keepID, mergeID uuid.UUID, combine domain.ShoppingItemCombiner) error {
	r.merged = append(r.merged, [2]uuid.UUID{keepID, mergeID})
	r.combine = combine
	return nil
}
func (r *fakeFoodRepo) AddTaxonomy(_ uuid.UUID, _ *domain.Taxonomy) error { return nil }
func (r *fakeFoodRepo) Update(food *domain.Food) error {
	if r.updateFn != nil {
		return r.updateFn(food)
	}
	return nil
}
func (r *fakeFoodRepo) CreatePrice(price *domain.FoodPrice) error {
	if r.createPriceFn != nil {
		return r.createPriceFn(price)
	}
	return nil
}
func (r *fakeFoodRepo) DeletePrice(_, _ uuid.UUID) error { return nil }
func (r *fakeFoodRepo) ListPrices(_, _ uuid.UUID, _ types.Pagination) ([]domain.FoodPrice, int64, error) {
	return nil, 0, nil
}
func (r *fakeFoodRepo) LatestPrices(_ uuid.UUID, _ []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
	return nil, nil
}
func (r *fakeFoodRepo) CreateConversion(c *domain.FoodUnitConversion) error {
	if r.createConversionFn != nil {
		return r.createConversionFn(c)
	}
	return nil
}
func (r *fakeFoodRepo) ListConversions(foodID uuid.UUID) ([]domain.FoodUnitConversion, error) {
	r.listedFoodID = foodID
	return nil, nil
}
func (r *fakeFoodRepo) CanonicalIDs(_ []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	return r.canonicalIDs, nil
}
func (r *fakeFoodRepo) ConversionByID(foodID, id uuid.UUID) (*domain.FoodUnitConversion, error) {
	if r.conversionByIDFn != nil {
		return r.conversionByIDFn(foodID, id)
	}
	return nil, sentinels.ErrNotFound
}
func (r *fakeFoodRepo) UpdateConversion(c *domain.FoodUnitConversion) error {
	if r.updateConversionFn != nil {
		return r.updateConversionFn(c)
	}
	return nil
}
func (r *fakeFoodRepo) DeleteConversion(_, _ uuid.UUID) error { return nil }
func (r *fakeFoodRepo) ConversionsByFoodIDs(_ []uuid.UUID) (map[uuid.UUID][]domain.FoodUnitConversion, error) {
	return nil, nil
}

// compile-time interface check
var _ domain.FoodRepository = (*fakeFoodRepo)(nil)

func newTestFoodService() domain.FoodService {
	return services.NewFoodService(&fakeFoodRepo{}, &stubImageService{}, &stubUnitService{})
}

func TestFoodService_Merge_SameID_ReturnsBadRequest(t *testing.T) {
	svc := newTestFoodService()
	id := uuid.New()

	err := svc.Merge(id, id)

	var se *sentinels.Error
	require.ErrorAs(t, err, &se, "same-ID merge must return a typed sentinel error")
	assert.Equal(t, 400, se.Status)
}

func TestFoodService_RecordPrice_SetsDefaultUnitWhenMissing(t *testing.T) {
	foodID := uuid.New()
	unitID := uuid.New()

	var updated *domain.Food
	repo := &fakeFoodRepo{
		byIDFn: func(id uuid.UUID) (*domain.Food, error) {
			return &domain.Food{ID: id, DefaultUnitID: nil}, nil
		},
		updateFn: func(food *domain.Food) error {
			updated = food
			return nil
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.RecordPrice(uuid.New(), &domain.FoodPrice{FoodID: foodID, UnitID: unitID, Price: 4.99, Amount: 1})

	require.NoError(t, err)
	require.NotNil(t, updated, "food without a default unit must be updated")
	require.NotNil(t, updated.DefaultUnitID)
	assert.Equal(t, unitID, *updated.DefaultUnitID, "default unit must adopt the price's unit")
}

func TestFoodService_RecordPrice_KeepsExistingDefaultUnit(t *testing.T) {
	existingUnitID := uuid.New()

	repo := &fakeFoodRepo{
		byIDFn: func(id uuid.UUID) (*domain.Food, error) {
			return &domain.Food{ID: id, DefaultUnitID: &existingUnitID}, nil
		},
		updateFn: func(_ *domain.Food) error {
			t.Fatal("Update must not be called when the food already has a default unit")
			return nil
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.RecordPrice(uuid.New(), &domain.FoodPrice{FoodID: uuid.New(), UnitID: uuid.New(), Price: 4.99, Amount: 1})

	require.NoError(t, err)
}

func TestFoodService_CreateConversion_Duplicate_ReturnsAlreadyExists(t *testing.T) {
	repo := &fakeFoodRepo{
		createConversionFn: func(_ *domain.FoodUnitConversion) error {
			return sentinels.ErrAlreadyExists
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.CreateConversion(&domain.FoodUnitConversion{
		FoodID:       uuid.New(),
		UnitID:       uuid.New(),
		TargetAmount: 200,
		TargetUnitID: uuid.New(),
	})

	require.ErrorIs(t, err, sentinels.ErrAlreadyExists, "duplicate unit_id for the same food must surface as 409")
}

func TestFoodService_UpdateConversion_TargetEqualsStoredSource_ReturnsBadRequest(t *testing.T) {
	sourceUnitID := uuid.New()
	repo := &fakeFoodRepo{
		conversionByIDFn: func(foodID, id uuid.UUID) (*domain.FoodUnitConversion, error) {
			return &domain.FoodUnitConversion{ID: id, FoodID: foodID, UnitID: sourceUnitID}, nil
		},
		updateConversionFn: func(_ *domain.FoodUnitConversion) error {
			t.Fatal("identity conversion must not be persisted")
			return nil
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.UpdateConversion(&domain.FoodUnitConversion{
		ID:           uuid.New(),
		FoodID:       uuid.New(),
		TargetAmount: 5,
		TargetUnitID: sourceUnitID,
	})

	var se *sentinels.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 400, se.Status)
}

func TestFoodService_UpdateConversion_Missing_ReturnsNotFound(t *testing.T) {
	repo := &fakeFoodRepo{
		updateConversionFn: func(_ *domain.FoodUnitConversion) error {
			t.Fatal("a missing conversion must not be updated")
			return nil
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.UpdateConversion(&domain.FoodUnitConversion{
		ID:           uuid.New(),
		FoodID:       uuid.New(),
		TargetAmount: 200,
		TargetUnitID: uuid.New(),
	})

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestFoodService_CreateConversion_StoresCanonicalUnits(t *testing.T) {
	alias, canonical, target := uuid.New(), uuid.New(), uuid.New()
	var stored *domain.FoodUnitConversion
	repo := &fakeFoodRepo{createConversionFn: func(c *domain.FoodUnitConversion) error {
		stored = c
		return nil
	}}
	units := &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{alias: canonical}}
	svc := services.NewFoodService(repo, &stubImageService{}, units)

	err := svc.CreateConversion(&domain.FoodUnitConversion{FoodID: uuid.New(), UnitID: alias, TargetAmount: 200, TargetUnitID: target})

	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, canonical, stored.UnitID, "a merged-away source unit must be stored as its canonical unit")
	assert.Equal(t, target, stored.TargetUnitID)
}

func TestFoodService_RecordPrice_StoresCanonicalFood(t *testing.T) {
	alias, canonical := uuid.New(), uuid.New()
	var stored *domain.FoodPrice
	repo := &fakeFoodRepo{
		canonicalIDs: map[uuid.UUID]uuid.UUID{alias: canonical},
		byIDFn: func(id uuid.UUID) (*domain.Food, error) {
			return &domain.Food{ID: id, DefaultUnitID: new(uuid.New())}, nil
		},
		createPriceFn: func(p *domain.FoodPrice) error {
			stored = p
			return nil
		},
	}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	err := svc.RecordPrice(uuid.New(), &domain.FoodPrice{FoodID: alias, UnitID: uuid.New(), Amount: 1, Price: 2})

	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, canonical, stored.FoodID, "a price for a merged-away food must be stored on its canonical food")
}

func TestFoodService_ListConversions_AliasFood_ListsCanonicalFood(t *testing.T) {
	alias, canonical := uuid.New(), uuid.New()
	repo := &fakeFoodRepo{canonicalIDs: map[uuid.UUID]uuid.UUID{alias: canonical}}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	_, err := svc.ListConversions(alias)

	require.NoError(t, err)
	assert.Equal(t, canonical, repo.listedFoodID, "a merged-away food's conversions live on its canonical food")
}

func TestFoodService_CreateConversion_AliasOfTarget_ReturnsBadRequest(t *testing.T) {
	alias, canonical := uuid.New(), uuid.New()
	repo := &fakeFoodRepo{createConversionFn: func(*domain.FoodUnitConversion) error {
		t.Fatal("a conversion between a unit and its own alias must not be stored")
		return nil
	}}
	units := &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{alias: canonical}}
	svc := services.NewFoodService(repo, &stubImageService{}, units)

	err := svc.CreateConversion(&domain.FoodUnitConversion{FoodID: uuid.New(), UnitID: alias, TargetAmount: 1, TargetUnitID: canonical})

	var se *sentinels.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 400, se.Status)
}

func TestFoodService_Merge_AliasIDs_MergeTheirCanonicalFoods(t *testing.T) {
	tomatoes, tomato, onion := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeFoodRepo{canonicalIDs: map[uuid.UUID]uuid.UUID{tomatoes: tomato}}
	svc := services.NewFoodService(repo, &stubImageService{}, &stubUnitService{})

	require.NoError(t, svc.Merge(tomatoes, onion))
	assert.Equal(t, [][2]uuid.UUID{{tomato, onion}}, repo.merged, "merging into an alias merges into its canonical food")
}

func TestFoodService_Merge_CombinesListItemsLikeAddItems(t *testing.T) {
	g, kg, ml := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeFoodRepo{}
	units := &stubUnitService{convertFn: func(amount float64, from, to *uuid.UUID, _ []domain.FoodUnitConversion) (float64, error) {
		if *from == g && *to == kg {
			return amount / 1000, nil
		}
		return 0, sentinels.Unprocessable("incompatible")
	}}
	svc := services.NewFoodService(repo, &stubImageService{}, units)
	require.NoError(t, svc.Merge(uuid.New(), uuid.New()))
	require.NotNil(t, repo.combine)

	target := &domain.ShoppingItem{UnitID: &kg, Amount: new(1.0)}
	ok, err := repo.combine(target, &domain.ShoppingItem{UnitID: &g, Amount: new(500.0)})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.InDelta(t, 1.5, *target.Amount, 1e-9, "500 g is converted into the kept item's kg")

	ok, err = repo.combine(target, &domain.ShoppingItem{UnitID: &ml, Amount: new(50.0)})
	require.NoError(t, err)
	assert.False(t, ok, "unconvertible units stay apart")

	ok, err = repo.combine(target, &domain.ShoppingItem{UnitID: &kg, Amount: new(1.0), IsBought: true})
	require.NoError(t, err)
	assert.True(t, ok, "a bought item is dropped: nothing is left to buy")
	assert.InDelta(t, 1.5, *target.Amount, 1e-9)

	bought := &domain.ShoppingItem{UnitID: &kg, Amount: new(1.0), IsBought: true, Text: "milk"}
	ok, err = repo.combine(bought, &domain.ShoppingItem{UnitID: &g, Amount: new(200.0), Text: "oat milk"})
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, bought.IsBought, "an open item reopens a bought one, as AddItems does")
	assert.InDelta(t, 200.0, *bought.Amount, 1e-9)
	assert.Equal(t, g, *bought.UnitID)
	assert.Equal(t, "oat milk", bought.Text)
}
