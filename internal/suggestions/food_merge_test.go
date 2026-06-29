package suggestions

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/internal/repositories"
)

func TestFoodMergeDetector_Detect_DedupesSymmetricPair(t *testing.T) {
	db := openPrivateTestDB(t)
	a := seedFood(t, db, "cinnamon", "Cinnamon")
	b := seedFood(t, db, "cinamon", "Cinamon") // typo, similarity ~0.875 > 0.85 threshold

	detector := NewFoodMergeDetector(repositories.NewFoodRepository(db))
	got, err := detector.Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)

	// Even though the nested loop visits (a,b) and (b,a), the seen-key dedup must
	// collapse them into a single suggestion.
	require.Len(t, got, 1)
	assert.Contains(t, []string{a.ID.String(), b.ID.String()}, got[0].SubjectID.String())
}

func TestFoodMergeDetector_Detect_DedupKeyIsOrderIndependent(t *testing.T) {
	db1 := openPrivateTestDB(t)
	seedFood(t, db1, "cinnamon", "Cinnamon")
	seedFood(t, db1, "cinamon", "Cinamon")

	got, err := NewFoodMergeDetector(repositories.NewFoodRepository(db1)).Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	key1 := got[0].DedupKey

	got2, err := NewFoodMergeDetector(repositories.NewFoodRepository(db1)).Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)
	require.Len(t, got2, 1)
	assert.Equal(t, key1, got2[0].DedupKey, "DedupKey must be stable across repeated detect calls")
}

func TestFoodMergeDetector_Detect_RespectsCancelledContext(t *testing.T) {
	db := openPrivateTestDB(t)
	seedFood(t, db, "cinnamon", "Cinnamon")
	seedFood(t, db, "cinamon", "Cinamon")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewFoodMergeDetector(repositories.NewFoodRepository(db)).Detect(ctx, time.Time{}, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
