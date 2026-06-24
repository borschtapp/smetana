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
type fakeFoodRepo struct{}

func (r *fakeFoodRepo) ByID(_ uuid.UUID) (*domain.Food, error) { return nil, nil }
func (r *fakeFoodRepo) ByIDs(_ []uuid.UUID) (map[uuid.UUID]*domain.Food, error) {
	return make(map[uuid.UUID]*domain.Food), nil
}
func (r *fakeFoodRepo) FindOrCreate(_ *domain.Food) error                       { return nil }
func (r *fakeFoodRepo) Search(_ string, _, _ int) ([]domain.Food, int64, error) { return nil, 0, nil }
func (r *fakeFoodRepo) Merge(_, _ uuid.UUID) error                              { return nil }
func (r *fakeFoodRepo) AddTaxonomy(_ uuid.UUID, _ *domain.Taxonomy) error       { return nil }
func (r *fakeFoodRepo) Update(_ *domain.Food) error                             { return nil }
func (r *fakeFoodRepo) CreatePrice(_ *domain.FoodPrice) error                   { return nil }
func (r *fakeFoodRepo) DeletePrice(_, _ uuid.UUID) error                        { return nil }
func (r *fakeFoodRepo) ListPrices(_, _ uuid.UUID, _ types.Pagination) ([]domain.FoodPrice, int64, error) {
	return nil, 0, nil
}
func (r *fakeFoodRepo) LatestPrices(_ uuid.UUID, _ []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
	return nil, nil
}

// compile-time interface check
var _ domain.FoodRepository = (*fakeFoodRepo)(nil)

func newTestFoodService() domain.FoodService {
	return services.NewFoodService(&fakeFoodRepo{}, &stubImageService{})
}

func TestFoodService_Merge_SameID_ReturnsBadRequest(t *testing.T) {
	svc := newTestFoodService()
	id := uuid.New()

	err := svc.Merge(id, id)

	var se *sentinels.Error
	require.ErrorAs(t, err, &se, "same-ID merge must return a typed sentinel error")
	assert.Equal(t, 400, se.Status)
}
