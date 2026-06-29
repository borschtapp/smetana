package suggestions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
)

// fakeUnitRepo is an in-memory stand-in for domain.UnitRepository, covering only
// NonConvertibleUsageSince, which is all FoodConversionDetector needs.
type fakeUnitRepo struct {
	domain.UnitRepository
	pairs []domain.FoodUnitPair
}

func (r *fakeUnitRepo) NonConvertibleUsageSince(_ time.Time) ([]domain.FoodUnitPair, error) {
	return r.pairs, nil
}

func TestFoodConversionDetector_Detect_DedupesPerFood(t *testing.T) {
	foodID, unitA, unitB := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeUnitRepo{pairs: []domain.FoodUnitPair{
		{FoodID: foodID, UnitID: unitA},
		{FoodID: foodID, UnitID: unitB}, // same food, second non-convertible unit: still one suggestion
	}}
	detector := NewFoodConversionDetector(repo)

	got, err := detector.Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)

	require.Len(t, got, 1, "a food with multiple non-convertible units still gets one suggestion")
	assert.Equal(t, foodID, got[0].SubjectID)
}
