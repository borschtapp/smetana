package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
)

func TestPublisherRepository_FindOrCreate_CreatesNewPublisher(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	pub := &domain.Publisher{Name: "Serious Eats", Url: new("https://seriouseats.com")}
	require.NoError(t, repo.FindOrCreate(pub))

	assert.NotEmpty(t, pub.ID)

	var count int64
	db.Model(&domain.Publisher{}).Where("name = ?", "Serious Eats").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestPublisherRepository_FindOrCreate_ExistingByURL_ReturnsExistingID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	first := &domain.Publisher{Name: "NYT Cooking", Url: new("https://cooking.nytimes.com")}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Publisher{Name: "NYT Cooking", Url: new("https://cooking.nytimes.com")}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same URL must resolve to the same publisher ID")

	var count int64
	db.Model(&domain.Publisher{}).Where("url = ?", "https://cooking.nytimes.com").Count(&count)
	assert.EqualValues(t, 1, count, "URL uniqueness: only one publisher row must exist after two FindOrCreate calls")
}

func TestPublisherRepository_FindOrCreate_ExistingByName_ReturnsExistingID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	first := &domain.Publisher{Name: "Bon Appétit"}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Publisher{Name: "Bon Appétit"}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID)

	var count int64
	db.Model(&domain.Publisher{}).Where("lower(name) = lower(?)", "Bon Appétit").Count(&count)
	assert.EqualValues(t, 1, count, "name fallback (no URL) must not create a duplicate publisher row")
}

func TestPublisherRepository_FindOrCreate_CaseInsensitiveName_ReturnsExisting(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	original := &domain.Publisher{Name: "food network"}
	require.NoError(t, db.Create(original).Error)

	lookup := &domain.Publisher{Name: "Food Network"}
	require.NoError(t, repo.FindOrCreate(lookup))

	assert.Equal(t, original.ID, lookup.ID, "case-insensitive name match must return the existing row, not create a new one")
}
