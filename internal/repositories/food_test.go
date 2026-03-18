package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
)

func TestFoodRepository_FindOrCreate_CreatesNewFood(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	f := &domain.Food{Name: "Potato"}
	require.NoError(t, repo.FindOrCreate(f))

	assert.NotEmpty(t, f.ID)
	assert.Equal(t, "potato", f.Slug, "CreateTag lowercases and strips diacritics")

	var count int64
	db.Table("food").Where("slug = ?", "potato").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestFoodRepository_FindOrCreate_ExistingFood_ReturnsExistingID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	first := &domain.Food{Name: "Carrot"}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Food{Name: "Carrot"}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same food name must resolve to the same ID")

	var count int64
	db.Table("food").Where("slug = ?", "carrot").Count(&count)
	assert.EqualValues(t, 1, count, "slug uniqueness: only one row must exist after two FindOrCreate calls")
}

func TestFoodRepository_FindOrCreate_CaseInsensitiveName_ReturnsExisting(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	original := &domain.Food{Name: "salt", Slug: "salt"}
	require.NoError(t, db.Create(original).Error)

	lookup := &domain.Food{Name: "Salt"}
	require.NoError(t, repo.FindOrCreate(lookup))

	assert.Equal(t, original.ID, lookup.ID)

	var count int64
	db.Table("food").Where("lower(name) = 'salt'").Count(&count)
	assert.EqualValues(t, 1, count, "case-insensitive name fallback must not create a duplicate row")
}

func TestFoodRepository_FindOrCreate_DiacriticName_NormalisedSlug(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	first := &domain.Food{Name: "crème"}
	require.NoError(t, repo.FindOrCreate(first))

	assert.Equal(t, "creme", first.Slug, "CreateTag must strip diacritics from slug")

	second := &domain.Food{Name: "crème"}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID)

	var count int64
	db.Table("food").Where("slug = ?", "creme").Count(&count)
	assert.EqualValues(t, 1, count, "normalised slug must deduplicate the same accented name")
}
