package repositories_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/storage"
)

func TestFoodRepository_FindOrCreate_CreatesNewFood(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	f := &domain.Food{Name: "Potato"}
	require.NoError(t, repo.FindOrCreate(f))

	assert.NotEmpty(t, f.ID)
	assert.Equal(t, "potato", f.Slug, "CreateTag lowercases and strips diacritics")

	var count int64
	db.Model(&domain.Food{}).Where("slug = ?", "potato").Count(&count)
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
	db.Model(&domain.Food{}).Where("slug = ?", "carrot").Count(&count)
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
	db.Model(&domain.Food{}).Where("lower(name) = 'salt'").Count(&count)
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
	db.Model(&domain.Food{}).Where("slug = ?", "creme").Count(&count)
	assert.EqualValues(t, 1, count, "normalised slug must deduplicate the same accented name")
}

func TestFoodRepository_Merge_SetsCanonicalFoodID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Tomato", Slug: "tomato"}
	merge := &domain.Food{Name: "Tomatoe", Slug: "tomatoe"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var alias domain.Food
	require.NoError(t, db.First(&alias, "id = ?", merge.ID).Error)
	require.NotNil(t, alias.CanonicalFoodID, "merged food must have canonical_food_id set")
	assert.Equal(t, keep.ID, *alias.CanonicalFoodID)
}

func TestFoodRepository_Merge_FindOrCreate_ResolvesAlias(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Aubergine", Slug: "aubergine"}
	merge := &domain.Food{Name: "Eggplant", Slug: "eggplant"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)
	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	lookup := &domain.Food{Name: "Eggplant"}
	require.NoError(t, repo.FindOrCreate(lookup))

	assert.Equal(t, keep.ID, lookup.ID, "FindOrCreate on the old name must resolve to the canonical food")
}

func TestFoodRepository_Merge_Search_ExcludesAliases(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Zucchini", Slug: "zucchini"}
	alias := &domain.Food{Name: "Courgette", Slug: "courgette"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, repo.Merge(keep.ID, alias.ID))

	foods, total, err := repo.Search("", 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "alias must not appear in search results")
	assert.Equal(t, keep.ID, foods[0].ID)
}

func TestFoodRepository_Merge_ReassignsFoodPrices(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Beef", Slug: "beef"}
	merge := &domain.Food{Name: "Minced Meat", Slug: "minced-meat"}
	unit := &domain.Unit{Name: "kg", Slug: "kg", BaseFactor: 1}
	hid := seedHousehold(t, db)
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)
	require.NoError(t, db.Create(unit).Error)
	require.NoError(t, db.Create(&domain.FoodPrice{
		HouseholdID: hid, FoodID: merge.ID, UnitID: unit.ID, Price: 5.0, Amount: 1,
	}).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var kept []domain.FoodPrice
	require.NoError(t, db.Where("food_id = ?", keep.ID).Find(&kept).Error)
	assert.Len(t, kept, 1, "price must be reassigned to the kept food")

	var remaining []domain.FoodPrice
	require.NoError(t, db.Where("food_id = ?", merge.ID).Find(&remaining).Error)
	assert.Empty(t, remaining, "no prices must remain on the merged food")
}

func TestFoodRepository_Merge_KeepIsAlias_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	canonical := &domain.Food{Name: "Cilantro", Slug: "cilantro"}
	alias := &domain.Food{Name: "Coriander", Slug: "coriander"}
	other := &domain.Food{Name: "Parsley", Slug: "parsley"}
	require.NoError(t, db.Create(canonical).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, repo.Merge(canonical.ID, alias.ID))

	err := repo.Merge(alias.ID, other.ID)
	require.ErrorIs(t, err, sentinels.ErrNotFound, "alias cannot be used as the keep target")
}

func TestFoodRepository_Merge_MergeTargetIsAlias_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	canonical := &domain.Food{Name: "Rocket", Slug: "rocket"}
	alias := &domain.Food{Name: "Arugula", Slug: "arugula"}
	other := &domain.Food{Name: "Spinach", Slug: "spinach"}
	require.NoError(t, db.Create(canonical).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, repo.Merge(canonical.ID, alias.ID))

	err := repo.Merge(other.ID, alias.ID)
	require.ErrorIs(t, err, sentinels.ErrNotFound, "alias cannot be used as the merge source")
}

func TestFoodRepository_Merge_InheritsImageFromMerged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	imagePath := storage.Path("/images/olive.jpg")
	keep := &domain.Food{Name: "Olive Oil", Slug: "olive-oil"}
	merge := &domain.Food{Name: "Olive oil", Slug: "olive-oil-2", ImagePath: &imagePath}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	require.NotNil(t, result.ImagePath, "keep food must inherit image when it had none")
	assert.Equal(t, imagePath, *result.ImagePath)
}

func TestFoodRepository_Merge_DoesNotOverwriteExistingImage(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keepImage := storage.Path("/images/keep.jpg")
	mergeImage := storage.Path("/images/merge.jpg")
	keep := &domain.Food{Name: "Butter", Slug: "butter", ImagePath: &keepImage}
	merge := &domain.Food{Name: "Buter", Slug: "buter", ImagePath: &mergeImage}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	require.NotNil(t, result.ImagePath)
	assert.Equal(t, keepImage, *result.ImagePath, "keep food must retain its own image when both have one")
}

func TestFoodRepository_Merge_InheritsDefaultUnitFromMerged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	unit := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	require.NoError(t, db.Create(unit).Error)

	keep := &domain.Food{Name: "Flour", Slug: "flour"}
	merge := &domain.Food{Name: "Flour (variant)", Slug: "flour-variant", DefaultUnitID: &unit.ID}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	require.NotNil(t, result.DefaultUnitID, "keep food must inherit default_unit_id when it had none")
	assert.Equal(t, unit.ID, *result.DefaultUnitID)
}

func TestFoodRepository_Merge_DoesNotOverwriteExistingDefaultUnit(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keepUnit := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	mergeUnit := &domain.Unit{Name: "kilogram", Slug: "kilogram", BaseFactor: 1}
	require.NoError(t, db.Create(keepUnit).Error)
	require.NoError(t, db.Create(mergeUnit).Error)

	keep := &domain.Food{Name: "Sugar", Slug: "sugar", DefaultUnitID: &keepUnit.ID}
	merge := &domain.Food{Name: "Suger", Slug: "suger", DefaultUnitID: &mergeUnit.ID}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	require.NotNil(t, result.DefaultUnitID)
	assert.Equal(t, keepUnit.ID, *result.DefaultUnitID, "keep food must retain its own default unit when both have one")
}

func TestFoodRepository_Merge_InheritsPantryFromMerged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Olive Oil", Slug: "olive-oil", Pantry: false}
	merge := &domain.Food{Name: "Olive oil", Slug: "olive-oil-2", Pantry: true}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	assert.True(t, result.Pantry, "keep food must become pantry when merged food was pantry")
}

func TestFoodRepository_Merge_KeepsPantryTrue(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Salt", Slug: "salt", Pantry: true}
	merge := &domain.Food{Name: "Sea salt", Slug: "sea-salt", Pantry: false}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Food
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	assert.True(t, result.Pantry, "keep food must remain pantry when it already was")
}
