package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/types"
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

func TestPublisherRepository_Search_SortByTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	pubA := seedPublisher(t, db)
	pubB := seedPublisher(t, db)
	pubC := seedPublisher(t, db)

	r1, r2, r3 := &domain.Recipe{PublisherID: &pubA.ID}, &domain.Recipe{PublisherID: &pubA.ID}, &domain.Recipe{PublisherID: &pubC.ID}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	seedRecipe(t, db, r3)

	opts := types.SearchOptions{Sort: "total_recipes", Order: "DESC", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(uuid.Nil, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, results, 3)
	assert.Equal(t, pubA.ID, results[0].ID, "publisher with 2 recipes must rank first")
	assert.Equal(t, pubC.ID, results[1].ID, "publisher with 1 recipe must rank second")
	assert.Equal(t, pubB.ID, results[2].ID, "publisher with 0 recipes must rank last")
}

func TestPublisherRepository_Search_PreloadTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	pub := seedPublisher(t, db)
	r1, r2 := &domain.Recipe{PublisherID: &pub.ID}, &domain.Recipe{PublisherID: &pub.ID}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)

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

func TestPublisherRepository_Search_ScopeFeeds_FiltersToSubscribedFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewPublisherRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	feedPub := seedPublisher(t, db)
	otherPub := seedPublisher(t, db)

	feedRecipe := &domain.Recipe{FeedID: &feed.ID, PublisherID: &feedPub.ID}
	otherRecipe := &domain.Recipe{PublisherID: &otherPub.ID}
	seedRecipe(t, db, feedRecipe)
	seedRecipe(t, db, otherRecipe)

	opts := types.SearchOptions{Sort: "id", Scope: "feeds", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, feedPub.ID, results[0].ID)
}
