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

func ptrStr(s string) *string { return &s }

func TestAuthorRepository_FindOrCreate_CreatesNew(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	a := &domain.Author{Name: "Julia Child", Url: ptrStr("https://juliachild.com")}
	require.NoError(t, repo.FindOrCreate(a))

	assert.NotEmpty(t, a.ID)
	var count int64
	db.Model(&domain.Author{}).Where("name = ?", "Julia Child").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestAuthorRepository_FindOrCreate_ExistingByURL_ReturnsExistingID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	first := &domain.Author{Name: "Gordon Ramsay", Url: ptrStr("https://gordonramsay.com")}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Author{Name: "Gordon Ramsay", Url: ptrStr("https://gordonramsay.com")}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same URL must resolve to the same author")
	var count int64
	db.Model(&domain.Author{}).Where("url = ?", "https://gordonramsay.com").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestAuthorRepository_FindOrCreate_ExistingByName_NoURL_ReturnsExistingID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	first := &domain.Author{Name: "Yotam Ottolenghi"}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Author{Name: "Yotam Ottolenghi"}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same name must resolve to the same author when no URL is set")
}

func TestAuthorRepository_Search_ReturnsAll(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	require.NoError(t, db.Create(&domain.Author{Name: "Author One"}).Error)
	require.NoError(t, db.Create(&domain.Author{Name: "Author Two"}).Error)

	results, total, err := repo.Search(uuid.Nil, defaultSearchOpts())

	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, results, 2)
}

func TestAuthorRepository_Search_QueryFilters(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	require.NoError(t, db.Create(&domain.Author{Name: "Jamie Oliver"}).Error)
	require.NoError(t, db.Create(&domain.Author{Name: "Nigella Lawson"}).Error)

	opts := types.SearchOptions{SearchQuery: "jamie", Sort: "id", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(uuid.Nil, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Len(t, results, 1)
	assert.Equal(t, "Jamie Oliver", results[0].Name)
}

func TestAuthorRepository_Search_SortByTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	authorA := &domain.Author{Name: "Prolific Author"}
	authorB := &domain.Author{Name: "Sparse Author"}
	authorC := &domain.Author{Name: "One Recipe Author"}
	require.NoError(t, db.Create(authorA).Error)
	require.NoError(t, db.Create(authorB).Error)
	require.NoError(t, db.Create(authorC).Error)

	r1, r2, r3 := &domain.Recipe{AuthorID: &authorA.ID}, &domain.Recipe{AuthorID: &authorA.ID}, &domain.Recipe{AuthorID: &authorC.ID}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	seedRecipe(t, db, r3)

	opts := types.SearchOptions{Sort: "total_recipes", Order: "DESC", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(uuid.Nil, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, results, 3)
	assert.Equal(t, authorA.ID, results[0].ID, "author with 2 recipes must rank first")
	assert.Equal(t, authorC.ID, results[1].ID, "author with 1 recipe must rank second")
	assert.Equal(t, authorB.ID, results[2].ID, "author with 0 recipes must rank last")
}

func TestAuthorRepository_Search_PreloadTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	a := &domain.Author{Name: "Active Author"}
	require.NoError(t, db.Create(a).Error)
	r1, r2 := &domain.Recipe{AuthorID: &a.ID}, &domain.Recipe{AuthorID: &a.ID}
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

func TestAuthorRepository_Search_ScopeFeeds_FiltersToSubscribedFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewAuthorRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	feedAuthor := &domain.Author{Name: "Feed Author"}
	otherAuthor := &domain.Author{Name: "Other Author"}
	require.NoError(t, db.Create(feedAuthor).Error)
	require.NoError(t, db.Create(otherAuthor).Error)

	feedRecipe := &domain.Recipe{FeedID: &feed.ID, AuthorID: &feedAuthor.ID}
	otherRecipe := &domain.Recipe{AuthorID: &otherAuthor.ID}
	seedRecipe(t, db, feedRecipe)
	seedRecipe(t, db, otherRecipe)

	opts := types.SearchOptions{Sort: "id", Scope: "feeds", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, feedAuthor.ID, results[0].ID)
}
