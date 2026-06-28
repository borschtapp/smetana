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
	byIDFn             func(uuid.UUID) (*domain.Food, error)
	createPriceFn      func(*domain.FoodPrice) error
	updateFn           func(*domain.Food) error
}

func (r *fakeFoodRepo) ByID(id uuid.UUID) (*domain.Food, error) {
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
func (r *fakeFoodRepo) Merge(_, _ uuid.UUID) error                              { return nil }
func (r *fakeFoodRepo) AddTaxonomy(_ uuid.UUID, _ *domain.Taxonomy) error       { return nil }
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
func (r *fakeFoodRepo) ListConversions(_ uuid.UUID) ([]domain.FoodUnitConversion, error) {
	return nil, nil
}
func (r *fakeFoodRepo) UpdateConversion(_ *domain.FoodUnitConversion) error { return nil }
func (r *fakeFoodRepo) DeleteConversion(_, _ uuid.UUID) error               { return nil }
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
