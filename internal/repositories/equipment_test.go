package repositories_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/database"
	"borscht.app/smetana/internal/repositories"
)

func openPrivateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=private", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "failed to open private in-memory SQLite")
	require.NoError(t, database.Migrate(db), "failed to apply migrations")
	return db
}

func TestEquipmentRepository_FindOrCreate_CreatesNewEquipment(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	e := &domain.Equipment{Name: "Dutch Oven"}
	require.NoError(t, repo.FindOrCreate(e))

	assert.NotEmpty(t, e.ID, "ID must be assigned after creation")
	assert.Equal(t, "dutch oven", e.Slug, "CreateTag lowercases and preserves spaces")

	var count int64
	db.Model(&domain.Equipment{}).Where("slug = ?", "dutch oven").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestEquipmentRepository_FindOrCreate_ReturnsExistingBySlug(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	first := &domain.Equipment{Name: "Stand Mixer"}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Equipment{Name: "Stand Mixer"}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same name must resolve to the same equipment ID")

	var count int64
	db.Model(&domain.Equipment{}).Where("slug = ?", "stand mixer").Count(&count)
	assert.EqualValues(t, 1, count, "slug uniqueness: only one row must exist after two FindOrCreate calls")
}

func TestEquipmentRepository_FindOrCreate_CaseInsensitiveNameFallback(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	original := &domain.Equipment{Name: "food processor", Slug: "food processor"}
	require.NoError(t, db.Create(original).Error)

	lookup := &domain.Equipment{Name: "Food Processor", Slug: "food processor variant"}
	require.NoError(t, repo.FindOrCreate(lookup))

	var count int64
	db.Model(&domain.Equipment{}).Where("lower(name) = lower(?)", "food processor").Count(&count)
	assert.EqualValues(t, 1, count, "case-insensitive name fallback must not create a duplicate row")
}

func TestEquipmentRepository_FindOrCreate_ConflictOnCreate_FetchesExisting(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	competitor := &domain.Equipment{Name: "Wok", Slug: "wok"}
	require.NoError(t, db.Create(competitor).Error)

	e := &domain.Equipment{Name: "Wok"}
	require.NoError(t, repo.FindOrCreate(e))
	assert.Equal(t, competitor.ID, e.ID, "OnConflict DoNothing must trigger a re-fetch instead of returning an error")
}

func TestEquipmentRepository_Search_ReturnsMatchingByName(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	require.NoError(t, db.Create(&domain.Equipment{Name: "Pressure Cooker", Slug: "pressure cooker"}).Error)
	require.NoError(t, db.Create(&domain.Equipment{Name: "Rice Cooker", Slug: "rice cooker"}).Error)
	require.NoError(t, db.Create(&domain.Equipment{Name: "Blender", Slug: "blender"}).Error)

	results, total, err := repo.Search("cooker", 0, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, results, 2)
}

func TestEquipmentRepository_Search_EmptyQuery_ReturnsAll(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	require.NoError(t, db.Create(&domain.Equipment{Name: "Pan", Slug: "pan"}).Error)
	require.NoError(t, db.Create(&domain.Equipment{Name: "Pot", Slug: "pot"}).Error)

	_, total, err := repo.Search("", 0, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
}
