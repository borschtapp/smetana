package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/types"
)

func linkRecipeEquipment(t *testing.T, db *gorm.DB, recipeID, equipmentID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Table("recipe_equipment").Create(map[string]any{
		"recipe_id": recipeID, "equipment_id": equipmentID,
	}).Error)
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

	results, total, err := repo.Search(uuid.Nil, types.SearchOptions{SearchQuery: "cooker", Sort: "id", Pagination: types.Pagination{Limit: 10}})

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, results, 2)
}

func TestEquipmentRepository_Search_EmptyQuery_ReturnsAll(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	require.NoError(t, db.Create(&domain.Equipment{Name: "Pan", Slug: "pan"}).Error)
	require.NoError(t, db.Create(&domain.Equipment{Name: "Pot", Slug: "pot"}).Error)

	_, total, err := repo.Search(uuid.Nil, types.SearchOptions{Sort: "id", Pagination: types.Pagination{Limit: 10}})

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
}

func TestEquipmentRepository_Search_SortByTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	eA := seedEquipment(t, db, "Instant Pot")
	eB := seedEquipment(t, db, "Spatula")
	eC := seedEquipment(t, db, "Whisk")

	r1, r2, r3 := &domain.Recipe{}, &domain.Recipe{}, &domain.Recipe{}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	seedRecipe(t, db, r3)

	// eA gets 2 recipes, eC gets 1, eB gets 0
	linkRecipeEquipment(t, db, r1.ID, eA.ID)
	linkRecipeEquipment(t, db, r2.ID, eA.ID)
	linkRecipeEquipment(t, db, r3.ID, eC.ID)

	opts := types.SearchOptions{Sort: "total_recipes", Order: "DESC", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(uuid.Nil, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, results, 3)
	assert.Equal(t, eA.ID, results[0].ID, "equipment with 2 recipes must rank first")
	assert.Equal(t, eC.ID, results[1].ID, "equipment with 1 recipe must rank second")
	assert.Equal(t, eB.ID, results[2].ID, "equipment with 0 recipes must rank last")
}

func TestEquipmentRepository_Search_PreloadTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	e := seedEquipment(t, db, "Cast Iron Skillet")
	r1, r2 := &domain.Recipe{}, &domain.Recipe{}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	linkRecipeEquipment(t, db, r1.ID, e.ID)
	linkRecipeEquipment(t, db, r2.ID, e.ID)

	opts := types.SearchOptions{
		Sort:           "id",
		Pagination:     types.Pagination{Limit: 10},
		PreloadOptions: types.Preload("total_recipes"),
	}
	results, _, err := repo.Search(uuid.Nil, opts)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].TotalRecipes, "TotalRecipes must be populated when preloaded")
	assert.EqualValues(t, 2, *results[0].TotalRecipes)
}

func TestEquipmentRepository_Search_ScopeFeeds_FiltersToSubscribedFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewEquipmentRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	feedEq := seedEquipment(t, db, "Feed Wok")
	otherEq := seedEquipment(t, db, "Other Wok")

	feedRecipe := &domain.Recipe{FeedID: &feed.ID}
	otherRecipe := &domain.Recipe{}
	seedRecipe(t, db, feedRecipe)
	seedRecipe(t, db, otherRecipe)
	linkRecipeEquipment(t, db, feedRecipe.ID, feedEq.ID)
	linkRecipeEquipment(t, db, otherRecipe.ID, otherEq.ID)

	opts := types.SearchOptions{Sort: "id", Scope: "feeds", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, feedEq.ID, results[0].ID)
}
