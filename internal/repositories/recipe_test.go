package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/database"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

// openTestDB creates a fresh in-memory SQLite database with the full production
// schema applied via database.Migrate, ensuring parity with production tables.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err, "failed to open in-memory SQLite")
	require.NoError(t, database.Migrate(db), "failed to apply migrations")
	return db
}

// seedRecipe creates a minimal Recipe row directly in the DB.
func seedRecipe(t *testing.T, db *gorm.DB, r *domain.Recipe) {
	t.Helper()
	if r.ID == uuid.Nil {
		r.ID, _ = uuid.NewV7()
	}
	require.NoError(t, db.Create(r).Error)
}

// seedHousehold creates a Household row and returns its ID.
func seedHousehold(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	hid, _ := uuid.NewV7()
	h := &domain.Household{ID: hid, Name: "Test Household"}
	require.NoError(t, db.Create(h).Error)
	return hid
}

// seedUser creates a User row belonging to the given household.
func seedUser(t *testing.T, db *gorm.DB, hid uuid.UUID) *domain.User {
	t.Helper()
	uid, _ := uuid.NewV7()
	u := &domain.User{ID: uid, HouseholdID: hid, Name: "Tester", Email: uid.String() + "@test.com"}
	require.NoError(t, db.Create(u).Error)
	return u
}

// seedFeed creates a Feed row directly in the DB.
func seedFeed(t *testing.T, db *gorm.DB) *domain.Feed {
	t.Helper()
	fid, _ := uuid.NewV7()
	f := &domain.Feed{ID: fid, Url: "https://feed.example.com/" + fid.String(), Name: "Test Feed"}
	require.NoError(t, db.Create(f).Error)
	return f
}

// defaultSearchOpts returns minimal valid SearchOptions for use in repository tests.
func defaultSearchOpts() types.SearchOptions {
	return types.SearchOptions{Sort: "id", Pagination: types.Pagination{Limit: 10}}
}

func TestRecipeRepository_ByID_ReturnsRecipe(t *testing.T) {
	db := openTestDB(t)
	name := "Borsch"
	r := &domain.Recipe{Name: &name}
	seedRecipe(t, db, r)

	repo := repositories.NewRecipeRepository(db)
	got, err := repo.ByID(r.ID)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, r.ID, got.ID)
	assert.Equal(t, name, *got.Name)
}

func TestRecipeRepository_ByID_NotFound_ReturnsErrRecordNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewRecipeRepository(db)

	_, err := repo.ByID(uuid.New())

	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}

func TestRecipeRepository_ByUrl_FindsByIsBasedOn(t *testing.T) {
	db := openTestDB(t)
	testURL := "https://example.com/recipe/borsch"
	r := &domain.Recipe{IsBasedOn: &testURL}
	seedRecipe(t, db, r)

	repo := repositories.NewRecipeRepository(db)
	got, err := repo.ByUrl(testURL)

	require.NoError(t, err)
	assert.Equal(t, r.ID, got.ID)
}

func TestRecipeRepository_ByUrl_NotFound_ReturnsErrRecordNotFound(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewRecipeRepository(db)

	_, err := repo.ByUrl("https://example.com/no-such-recipe")

	require.ErrorIs(t, err, sentinels.ErrRecordNotFound)
}

func TestRecipeRepository_ByParentIDsAndHousehold_EmptyIDs_ReturnsNil(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewRecipeRepository(db)

	got, err := repo.ByParentIDsAndHousehold([]uuid.UUID{}, uuid.New())

	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRecipeRepository_ByParentIDsAndHousehold_ReturnsMatchingOverrides(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)

	// Global recipe
	globalID, _ := uuid.NewV7()
	global := &domain.Recipe{ID: globalID}
	seedRecipe(t, db, global)

	// Household copy pointing at the global recipe
	cloneID, _ := uuid.NewV7()
	clone := &domain.Recipe{ID: cloneID, ParentID: &globalID, HouseholdID: &hid}
	seedRecipe(t, db, clone)

	repo := repositories.NewRecipeRepository(db)
	got, err := repo.ByParentIDsAndHousehold([]uuid.UUID{globalID}, hid)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, cloneID, got[0].ID)
}

func TestRecipeRepository_ByParentIDsAndHousehold_OtherHousehold_ReturnsEmpty(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)

	globalID, _ := uuid.NewV7()
	global := &domain.Recipe{ID: globalID}
	seedRecipe(t, db, global)

	cloneID, _ := uuid.NewV7()
	clone := &domain.Recipe{ID: cloneID, ParentID: &globalID, HouseholdID: &hid}
	seedRecipe(t, db, clone)

	repo := repositories.NewRecipeRepository(db)
	// Search from a different household — must not see the clone
	got, err := repo.ByParentIDsAndHousehold([]uuid.UUID{globalID}, uuid.New())

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestRecipeRepository_UserSave_Idempotent(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)
	u := seedUser(t, db, hid)

	r := &domain.Recipe{}
	seedRecipe(t, db, r)

	repo := repositories.NewRecipeRepository(db)

	// Save twice — second call must not fail due to ON CONFLICT DO NOTHING.
	require.NoError(t, repo.UserSave(r.ID, u.ID, hid))
	require.NoError(t, repo.UserSave(r.ID, u.ID, hid), "duplicate save must be a no-op, not an error")

	var count int64
	db.Table("recipes_saved").Where("recipe_id = ? AND user_id = ?", r.ID, u.ID).Count(&count)
	assert.EqualValues(t, 1, count, "exactly one row must exist after duplicate saves")
}

func TestRecipeRepository_ReplaceRecipePointers_UpdatesRecipesSaved(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)
	u := seedUser(t, db, hid)

	oldRecipe := &domain.Recipe{}
	newRecipe := &domain.Recipe{}
	seedRecipe(t, db, oldRecipe)
	seedRecipe(t, db, newRecipe)

	// Create a saved entry pointing at oldRecipe
	require.NoError(t, db.Create(&domain.RecipeSaved{
		RecipeID:    oldRecipe.ID,
		UserID:      u.ID,
		HouseholdID: hid,
	}).Error)

	repo := repositories.NewRecipeRepository(db)
	err := repo.ReplaceRecipePointers(oldRecipe.ID, newRecipe.ID, hid)
	require.NoError(t, err)

	var count int64
	db.Table("recipes_saved").Where("recipe_id = ? AND household_id = ?", newRecipe.ID, hid).Count(&count)
	assert.EqualValues(t, 1, count, "recipes_saved must point to the new recipe ID after replacement")

	db.Table("recipes_saved").Where("recipe_id = ? AND household_id = ?", oldRecipe.ID, hid).Count(&count)
	assert.EqualValues(t, 0, count, "old recipe ID must no longer appear in recipes_saved")
}

func TestRecipeRepository_ReplaceRecipePointers_OtherHouseholdNotAffected(t *testing.T) {
	db := openTestDB(t)
	hid1 := seedHousehold(t, db)
	hid2 := seedHousehold(t, db)
	u1 := seedUser(t, db, hid1)
	u2 := seedUser(t, db, hid2)

	oldRecipe := &domain.Recipe{}
	newRecipe := &domain.Recipe{}
	seedRecipe(t, db, oldRecipe)
	seedRecipe(t, db, newRecipe)

	// Both households have the recipe saved
	require.NoError(t, db.Create(&domain.RecipeSaved{RecipeID: oldRecipe.ID, UserID: u1.ID, HouseholdID: hid1}).Error)
	require.NoError(t, db.Create(&domain.RecipeSaved{RecipeID: oldRecipe.ID, UserID: u2.ID, HouseholdID: hid2}).Error)

	repo := repositories.NewRecipeRepository(db)
	// Only migrate hid1's pointer
	err := repo.ReplaceRecipePointers(oldRecipe.ID, newRecipe.ID, hid1)
	require.NoError(t, err)

	// hid1 now points to newRecipe
	var count int64
	db.Table("recipes_saved").Where("recipe_id = ? AND household_id = ?", newRecipe.ID, hid1).Count(&count)
	assert.EqualValues(t, 1, count)

	// hid2 must still point to oldRecipe, untouched
	db.Table("recipes_saved").Where("recipe_id = ? AND household_id = ?", oldRecipe.ID, hid2).Count(&count)
	assert.EqualValues(t, 1, count, "other household's saved recipe must not be affected")
}

func TestRecipeRepository_Transaction_RollsBackOnError(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewRecipeRepository(db)

	var createdID uuid.UUID
	txErr := repo.Transaction(func(txRepo domain.RecipeRepository) error {
		r := &domain.Recipe{}
		require.NoError(t, txRepo.Create(r))
		createdID = r.ID
		return assert.AnError // trigger rollback
	})

	require.Error(t, txErr)

	var count int64
	db.Model(&domain.Recipe{}).Where("id = ?", createdID).Count(&count)
	assert.EqualValues(t, 0, count, "recipe created in a rolled-back transaction must not exist")
}

func TestRecipeRepository_Search_SavedPath_NoComputedColumnError(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)
	u := seedUser(t, db, hid)
	r := &domain.Recipe{}
	seedRecipe(t, db, r)
	require.NoError(t, db.Create(&domain.RecipeSaved{RecipeID: r.ID, UserID: u.ID, HouseholdID: hid}).Error)

	repo := repositories.NewRecipeRepository(db)
	results, total, err := repo.Search(u.ID, hid, domain.RecipeSearchOptions{
		SearchOptions: defaultSearchOpts(),
	})

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, results, 1)
	assert.Equal(t, r.ID, results[0].ID)
	assert.Nil(t, results[0].IsSaved, "IsSaved must be nil when 'saved' preload is not requested")
}

func TestRecipeRepository_Search_WithSavedPreload_PopulatesIsSaved(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)
	u := seedUser(t, db, hid)
	r := &domain.Recipe{}
	seedRecipe(t, db, r)
	require.NoError(t, db.Create(&domain.RecipeSaved{RecipeID: r.ID, UserID: u.ID, HouseholdID: hid}).Error)

	opts := defaultSearchOpts()
	opts.Preload = []string{"saved"}

	repo := repositories.NewRecipeRepository(db)
	results, _, err := repo.Search(u.ID, hid, domain.RecipeSearchOptions{SearchOptions: opts})

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].IsSaved, "IsSaved must be populated when 'saved' preload is requested")
	assert.True(t, *results[0].IsSaved)
}

func TestRecipeRepository_Search_FromFeeds_NoComputedColumnError(t *testing.T) {
	db := openTestDB(t)
	hid := seedHousehold(t, db)
	u := seedUser(t, db, hid)
	feed := seedFeed(t, db)
	require.NoError(t, db.Model(&domain.Household{ID: hid}).Association("Feeds").Append(feed))
	r := &domain.Recipe{FeedID: &feed.ID}
	seedRecipe(t, db, r)

	repo := repositories.NewRecipeRepository(db)
	results, total, err := repo.Search(u.ID, hid, domain.RecipeSearchOptions{
		SearchOptions: defaultSearchOpts(),
		FromFeeds:     true,
	})

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, results, 1)
	assert.Equal(t, r.ID, results[0].ID)
}

func TestRecipeRepository_Import_CreatesRecipeWithoutPublisher(t *testing.T) {
	db := openTestDB(t)
	repo := repositories.NewRecipeRepository(db)

	pub := &domain.Publisher{Name: "Test Publisher", Url: "https://pub.example.com"}
	recipe := &domain.Recipe{
		Publisher: pub,
	}

	err := repo.Import(recipe)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, recipe.ID, "recipe must receive a UUID after Import")

	// The publisher must NOT have been created by Import. It will be resolved later by matching on name and URL.
	var pubCount int64
	db.Model(&domain.Publisher{}).Where("name = ?", "Test Publisher").Count(&pubCount)
	assert.EqualValues(t, 0, pubCount, "Import must omit Publisher association creation")
}
