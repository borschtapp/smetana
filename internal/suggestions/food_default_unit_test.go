package suggestions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
)

// fakeFoodRepo is an in-memory stand-in for domain.FoodRepository, covering only the
// methods FoodDefaultUnitApplier needs (ByID/Update).
type fakeFoodRepo struct {
	domain.FoodRepository
	foods map[uuid.UUID]*domain.Food
	usage []domain.FoodUnitUsage
}

func newFakeFoodRepo(foods ...*domain.Food) *fakeFoodRepo {
	r := &fakeFoodRepo{foods: make(map[uuid.UUID]*domain.Food)}
	for _, f := range foods {
		r.foods[f.ID] = f
	}
	return r
}

func (r *fakeFoodRepo) ByID(id uuid.UUID) (*domain.Food, error) {
	f, ok := r.foods[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *f
	return &cp, nil
}

func (r *fakeFoodRepo) Update(f *domain.Food) error {
	r.foods[f.ID] = f
	return nil
}

func (r *fakeFoodRepo) DefaultUnitUsageSince(_ time.Time) ([]domain.FoodUnitUsage, error) {
	return r.usage, nil
}

func TestFoodDefaultUnitApplier_Apply_SetsDefaultUnit(t *testing.T) {
	unitID := uuid.New()
	updated := time.Now().Truncate(time.Second)
	food := &domain.Food{ID: uuid.New(), Name: "Carrot", Updated: updated}
	repo := newFakeFoodRepo(food)
	applier := NewFoodDefaultUnitApplier(repo)

	payload, _ := json.Marshal(map[string]any{
		"food_id":      food.ID,
		"unit_id":      unitID,
		"food_updated": updated,
	})
	sg := &domain.Suggestion{Payload: payload}

	require.NoError(t, applier.Apply(context.Background(), sg))
	assert.Equal(t, &unitID, repo.foods[food.ID].DefaultUnitID)
}

func TestFoodDefaultUnitApplier_Apply_SupersededWhenAlreadySet(t *testing.T) {
	unitID := uuid.New()
	otherUnitID := uuid.New()
	updated := time.Now().Truncate(time.Second)
	food := &domain.Food{ID: uuid.New(), Name: "Carrot", Updated: updated, DefaultUnitID: &otherUnitID}
	repo := newFakeFoodRepo(food)
	applier := NewFoodDefaultUnitApplier(repo)

	payload, _ := json.Marshal(map[string]any{
		"food_id":      food.ID,
		"unit_id":      unitID,
		"food_updated": updated,
	})
	sg := &domain.Suggestion{Payload: payload}

	err := applier.Apply(context.Background(), sg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "food changed since suggestion was generated")
	// The pre-existing default unit must be untouched.
	assert.Equal(t, &otherUnitID, repo.foods[food.ID].DefaultUnitID)
}

func TestFoodDefaultUnitApplier_Apply_SupersededWhenFoodChanged(t *testing.T) {
	unitID := uuid.New()
	capturedAt := time.Now().Add(-time.Hour).Truncate(time.Second)
	food := &domain.Food{ID: uuid.New(), Name: "Carrot", Updated: time.Now().Truncate(time.Second)}
	repo := newFakeFoodRepo(food)
	applier := NewFoodDefaultUnitApplier(repo)

	payload, _ := json.Marshal(map[string]any{
		"food_id":      food.ID,
		"unit_id":      unitID,
		"food_updated": capturedAt, // stale relative to food.Updated
	})
	sg := &domain.Suggestion{Payload: payload}

	err := applier.Apply(context.Background(), sg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "food changed since suggestion was generated")
}

func TestFoodDefaultUnitDetector_Detect_PicksMostUsedUnit(t *testing.T) {
	foodID, unitA, unitB := uuid.New(), uuid.New(), uuid.New()
	updated := time.Now().Truncate(time.Second)
	repo := &fakeFoodRepo{usage: []domain.FoodUnitUsage{
		{FoodID: foodID, UnitID: unitA, Count: 3, Updated: updated},
		{FoodID: foodID, UnitID: unitB, Count: 1, Updated: updated},
	}}
	detector := NewFoodDefaultUnitDetector(repo)

	got, err := detector.Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)

	require.Len(t, got, 1)
	assert.Equal(t, &unitA, got[0].TargetID, "the more-used unit must be picked")
	assert.InDelta(t, 0.75, got[0].Confidence, 0.001, "confidence is usage / total across both units")
}
