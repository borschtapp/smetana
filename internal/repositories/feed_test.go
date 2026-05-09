package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

func seedFeedSubscription(t *testing.T, db *gorm.DB, householdID, feedID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&feedSubscription{}).Create(map[string]any{
		"household_id": householdID, "feed_id": feedID,
	}).Error)
}

func TestFeedRepository_ByIDForHousehold_ReturnsSubscribedFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	got, err := repo.ByIDForHousehold(feed.ID, hid)

	require.NoError(t, err)
	assert.Equal(t, feed.ID, got.ID)
}

func TestFeedRepository_ByIDForHousehold_NotSubscribed_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	// intentionally no subscription

	_, err := repo.ByIDForHousehold(feed.ID, hid)

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestFeedRepository_ByUrl_ReturnsMatchingFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	feed := seedFeed(t, db)

	got, err := repo.ByUrl(feed.Url)

	require.NoError(t, err)
	assert.Equal(t, feed.ID, got.ID)
}

func TestFeedRepository_ByUrl_NotFound_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	_, err := repo.ByUrl("https://no-such-feed.example.com")

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

func TestFeedRepository_ListActive_ReturnsOnlyActiveFeeds(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	active := seedFeed(t, db)
	inactive := seedFeed(t, db)
	require.NoError(t, db.Model(inactive).Update("active", false).Error)

	feeds, err := repo.ListActive()

	require.NoError(t, err)
	ids := make([]uuid.UUID, len(feeds))
	for i, f := range feeds {
		ids[i] = f.ID
	}
	assert.Contains(t, ids, active.ID)
	assert.NotContains(t, ids, inactive.ID)
}

func TestFeedRepository_Search_ReturnsOnlySubscribedFeeds(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	subscribed := seedFeed(t, db)
	_ = seedFeed(t, db) // not subscribed
	seedFeedSubscription(t, db, hid, subscribed.ID)

	results, total, err := repo.Search(hid, defaultSearchOpts())

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, results, 1)
	assert.Equal(t, subscribed.ID, results[0].ID)
}

func TestFeedRepository_Search_QueryFilters(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	fid1, _ := uuid.NewV7()
	fid2, _ := uuid.NewV7()
	f1 := &domain.Feed{ID: fid1, Url: "https://example.com/1", Name: "Baking Weekly"}
	f2 := &domain.Feed{ID: fid2, Url: "https://example.com/2", Name: "Vegan Daily"}
	require.NoError(t, db.Create(f1).Error)
	require.NoError(t, db.Create(f2).Error)
	seedFeedSubscription(t, db, hid, f1.ID)
	seedFeedSubscription(t, db, hid, f2.ID)

	opts := types.SearchOptions{SearchQuery: "baking", Sort: "id", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, f1.ID, results[0].ID)
}

func TestFeedRepository_Search_PreloadTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	r1, r2 := &domain.Recipe{FeedID: &feed.ID}, &domain.Recipe{FeedID: &feed.ID}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)

	opts := types.SearchOptions{
		Sort:           "id",
		Pagination:     types.Pagination{Limit: 10},
		PreloadOptions: types.Preload("total_recipes"),
	}
	results, _, err := repo.Search(hid, opts)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].TotalRecipes, "TotalRecipes must be populated when preloaded")
	assert.EqualValues(t, 2, *results[0].TotalRecipes)
}

func TestFeedRepository_AddFeed_CreatesSubscription(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)

	require.NoError(t, repo.AddFeed(hid, feed))

	var count int64
	db.Model(&feedSubscription{}).Where("household_id = ? AND feed_id = ?", hid, feed.ID).Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestFeedRepository_AddFeed_ReactivatesInactiveFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	require.NoError(t, db.Model(feed).Update("active", false).Error)
	feed.Active = false // Update does not mutate the struct; set explicitly for AddFeed's branch check

	require.NoError(t, repo.AddFeed(hid, feed))

	var got domain.Feed
	require.NoError(t, db.First(&got, feed.ID).Error)
	assert.True(t, got.Active, "re-subscribing must reactivate the feed")
}

func TestFeedRepository_DeleteFeed_RemovesSubscription(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	require.NoError(t, repo.DeleteFeed(hid, feed.ID))

	var count int64
	db.Model(&feedSubscription{}).Where("household_id = ? AND feed_id = ?", hid, feed.ID).Count(&count)
	assert.EqualValues(t, 0, count)
}

func TestFeedRepository_DeleteFeed_DeactivatesFeedWhenNoSubscribersRemain(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	require.NoError(t, repo.DeleteFeed(hid, feed.ID))

	var got domain.Feed
	require.NoError(t, db.First(&got, feed.ID).Error)
	assert.False(t, got.Active, "feed must be deactivated when the last subscriber unsubscribes")
}

func TestFeedRepository_DeleteFeed_KeepsFeedActiveWithOtherSubscribers(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFeedRepository(db)

	hid1 := seedHousehold(t, db)
	hid2 := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid1, feed.ID)
	seedFeedSubscription(t, db, hid2, feed.ID)

	require.NoError(t, repo.DeleteFeed(hid1, feed.ID))

	var got domain.Feed
	require.NoError(t, db.First(&got, feed.ID).Error)
	assert.True(t, got.Active, "feed must stay active while other households are still subscribed")
}
