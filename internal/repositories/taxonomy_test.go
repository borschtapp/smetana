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

func seedTaxonomy(t *testing.T, db *gorm.DB, label, taxType string) *domain.Taxonomy {
	t.Helper()
	tax := &domain.Taxonomy{Label: label, Slug: uuid.New().String(), Type: taxType}
	require.NoError(t, db.Create(tax).Error)
	return tax
}

func linkRecipeTaxonomy(t *testing.T, db *gorm.DB, recipeID, taxonomyID uuid.UUID) {
	t.Helper()
	require.NoError(t, db.Model(&recipeTaxonomy{}).Create(map[string]any{
		"recipe_id": recipeID, "taxonomy_id": taxonomyID,
	}).Error)
}

func seedSavedRecipe(t *testing.T, db *gorm.DB, recipeID, householdID uuid.UUID) {
	t.Helper()
	user := seedUser(t, db, householdID)
	require.NoError(t, db.Create(&domain.RecipeSaved{
		UserID: user.ID, RecipeID: recipeID, HouseholdID: householdID,
	}).Error)
}

func TestTaxonomyRepository_FindOrCreate_CreatesNew(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	tax := &domain.Taxonomy{Label: "Italian", Slug: "italian", Type: domain.TaxonomyTypeCuisine}
	require.NoError(t, repo.FindOrCreate(tax))

	assert.NotEmpty(t, tax.ID)
	var count int64
	db.Model(&domain.Taxonomy{}).Where("slug = ?", "italian").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestTaxonomyRepository_FindOrCreate_ReturnsExistingBySlug(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	first := &domain.Taxonomy{Label: "Vegan", Slug: "vegan", Type: domain.TaxonomyTypeDiet}
	require.NoError(t, repo.FindOrCreate(first))

	second := &domain.Taxonomy{Label: "Vegan", Slug: "vegan", Type: domain.TaxonomyTypeDiet}
	require.NoError(t, repo.FindOrCreate(second))

	assert.Equal(t, first.ID, second.ID, "same slug must resolve to the same taxonomy row")
	var count int64
	db.Model(&domain.Taxonomy{}).Where("slug = ?", "vegan").Count(&count)
	assert.EqualValues(t, 1, count)
}

func TestTaxonomyRepository_Search_SortByTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	taxA := seedTaxonomy(t, db, "Category A", domain.TaxonomyTypeCategory)
	taxB := seedTaxonomy(t, db, "Category B", domain.TaxonomyTypeCategory)
	taxC := seedTaxonomy(t, db, "Category C", domain.TaxonomyTypeCategory)

	r1 := &domain.Recipe{}
	r2 := &domain.Recipe{}
	r3 := &domain.Recipe{}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	seedRecipe(t, db, r3)

	// A gets 2 recipes, C gets 1, B gets 0
	linkRecipeTaxonomy(t, db, r1.ID, taxA.ID)
	linkRecipeTaxonomy(t, db, r2.ID, taxA.ID)
	linkRecipeTaxonomy(t, db, r3.ID, taxC.ID)

	opts := types.SearchOptions{Sort: "total_recipes", Order: "DESC", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search(domain.TaxonomyTypeCategory, uuid.Nil, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, results, 3)
	assert.Equal(t, taxA.ID, results[0].ID, "taxA with 2 recipes must rank first")
	assert.Equal(t, taxC.ID, results[1].ID, "taxC with 1 recipe must rank second")
	assert.Equal(t, taxB.ID, results[2].ID, "taxB with 0 recipes must rank last")
}

func TestTaxonomyRepository_Search_PreloadTotalRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	tax := seedTaxonomy(t, db, "Gluten Free", domain.TaxonomyTypeDiet)
	r1, r2 := &domain.Recipe{}, &domain.Recipe{}
	seedRecipe(t, db, r1)
	seedRecipe(t, db, r2)
	linkRecipeTaxonomy(t, db, r1.ID, tax.ID)
	linkRecipeTaxonomy(t, db, r2.ID, tax.ID)

	opts := types.SearchOptions{
		Sort:           "id",
		Pagination:     types.Pagination{Limit: 10},
		PreloadOptions: types.Preload("total_recipes"),
	}
	results, _, err := repo.Search("", uuid.Nil, opts)

	require.NoError(t, err)
	require.Len(t, results, 1)
	require.NotNil(t, results[0].TotalRecipes, "TotalRecipes must be populated when preloaded")
	assert.EqualValues(t, 2, *results[0].TotalRecipes)
}

func TestTaxonomyRepository_Search_ScopeFeeds_FiltersToSubscribedFeed(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	hid := seedHousehold(t, db)
	feed := seedFeed(t, db)
	seedFeedSubscription(t, db, hid, feed.ID)

	// feedRecipe belongs to the subscribed feed
	feedRecipe := &domain.Recipe{FeedID: &feed.ID}
	seedRecipe(t, db, feedRecipe)

	// otherRecipe is not in any feed
	otherRecipe := &domain.Recipe{}
	seedRecipe(t, db, otherRecipe)

	taxFeed := seedTaxonomy(t, db, "From Feed", domain.TaxonomyTypeCategory)
	taxOther := seedTaxonomy(t, db, "Not In Feed", domain.TaxonomyTypeCategory)
	linkRecipeTaxonomy(t, db, feedRecipe.ID, taxFeed.ID)
	linkRecipeTaxonomy(t, db, otherRecipe.ID, taxOther.ID)

	opts := types.SearchOptions{Sort: "id", Scope: "feeds", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search("", hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, taxFeed.ID, results[0].ID)
}

func TestTaxonomyRepository_Search_ScopeSaved_FiltersToSavedRecipes(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewTaxonomyRepository(db)

	hid := seedHousehold(t, db)

	savedRecipe := &domain.Recipe{}
	unseenRecipe := &domain.Recipe{}
	seedRecipe(t, db, savedRecipe)
	seedRecipe(t, db, unseenRecipe)
	seedSavedRecipe(t, db, savedRecipe.ID, hid)

	taxSaved := seedTaxonomy(t, db, "Saved Only", domain.TaxonomyTypeCategory)
	taxUnseen := seedTaxonomy(t, db, "Not Saved", domain.TaxonomyTypeCategory)
	linkRecipeTaxonomy(t, db, savedRecipe.ID, taxSaved.ID)
	linkRecipeTaxonomy(t, db, unseenRecipe.ID, taxUnseen.ID)

	opts := types.SearchOptions{Sort: "id", Scope: "saved", Pagination: types.Pagination{Limit: 10}}
	results, total, err := repo.Search("", hid, opts)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	assert.Equal(t, taxSaved.ID, results[0].ID)
}
