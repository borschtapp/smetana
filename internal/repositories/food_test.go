package repositories_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestFoodRepository_ConversionsByFoodIDs_BatchGroupsCorrectly(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	food1 := &domain.Food{Name: "Apple", Slug: "apple"}
	food2 := &domain.Food{Name: "Banana", Slug: "banana"}
	food3 := &domain.Food{Name: "Orange", Slug: "orange"}
	require.NoError(t, db.Create(food1).Error)
	require.NoError(t, db.Create(food2).Error)
	require.NoError(t, db.Create(food3).Error)

	unitMl := &domain.Unit{Name: "ml", Slug: "ml", BaseFactor: 1}
	unitCup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	unitG := &domain.Unit{Name: "g", Slug: "g", BaseFactor: 1}
	require.NoError(t, db.Create(unitMl).Error)
	require.NoError(t, db.Create(unitCup).Error)
	require.NoError(t, db.Create(unitG).Error)

	require.NoError(t, repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: food1.ID, UnitID: unitCup.ID, TargetAmount: 120, TargetUnitID: unitG.ID,
	}))
	require.NoError(t, repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: food2.ID, UnitID: unitCup.ID, TargetAmount: 150, TargetUnitID: unitG.ID,
	}))

	conversions, err := repo.ConversionsByFoodIDs([]uuid.UUID{food1.ID, food2.ID, food3.ID})
	require.NoError(t, err)

	assert.Len(t, conversions, 2)
	assert.Len(t, conversions[food1.ID], 1)
	assert.Len(t, conversions[food2.ID], 1)
	assert.Len(t, conversions[food3.ID], 0)
	assert.Equal(t, 120.0, conversions[food1.ID][0].TargetAmount)
	assert.Equal(t, 150.0, conversions[food2.ID][0].TargetAmount)
}

func TestFoodRepository_CreateConversion_UniqueConstraint(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	food := &domain.Food{Name: "Milk", Slug: "milk"}
	require.NoError(t, db.Create(food).Error)

	unitCup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	unitG := &domain.Unit{Name: "g", Slug: "g", BaseFactor: 1}
	unitMl := &domain.Unit{Name: "ml", Slug: "ml", BaseFactor: 1}
	require.NoError(t, db.Create(unitCup).Error)
	require.NoError(t, db.Create(unitG).Error)
	require.NoError(t, db.Create(unitMl).Error)

	err := repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: food.ID, UnitID: unitCup.ID, TargetAmount: 240, TargetUnitID: unitG.ID,
	})
	require.NoError(t, err)

	err = repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: food.ID, UnitID: unitCup.ID, TargetAmount: 240, TargetUnitID: unitMl.ID,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint failed")
}

func TestFoodRepository_Detach_ClearsAliasButKeepsReassignedRows(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Slug: "tomato", Name: "Tomato"}
	alias := &domain.Food{Slug: "tomat", Name: "Tomat"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(alias).Error)

	recipe := &domain.Recipe{Name: new("Salad")}
	require.NoError(t, db.Create(recipe).Error)
	ing := &domain.RecipeIngredient{RecipeID: recipe.ID, FoodID: &alias.ID, RawText: "tomat"}
	require.NoError(t, db.Create(ing).Error)

	require.NoError(t, repo.Merge(keep.ID, alias.ID))
	require.NoError(t, repo.Detach(alias.ID))

	var detached domain.Food
	require.NoError(t, db.First(&detached, "id = ?", alias.ID).Error)
	assert.Nil(t, detached.CanonicalFoodID, "detach must clear the alias pointer")

	// Documents the deliberate limitation: Merge moved this row and Detach does not move it back.
	var reloaded domain.RecipeIngredient
	require.NoError(t, db.First(&reloaded, "id = ?", ing.ID).Error)
	assert.Equal(t, keep.ID, *reloaded.FoodID, "reassigned rows stay with the kept food")
}

func TestFoodRepository_Merge_MovesConversionsAndDeduplicates(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Keep", Slug: "keep"}
	merge := &domain.Food{Name: "Merge", Slug: "merge"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	unit1 := &domain.Unit{Name: "u1", Slug: "u1", BaseFactor: 1}
	unit2 := &domain.Unit{Name: "u2", Slug: "u2", BaseFactor: 1}
	unitG := &domain.Unit{Name: "g", Slug: "g", BaseFactor: 1}
	require.NoError(t, db.Create(unit1).Error)
	require.NoError(t, db.Create(unit2).Error)
	require.NoError(t, db.Create(unitG).Error)

	// keep has a conversion from u1 to g
	require.NoError(t, repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: keep.ID, UnitID: unit1.ID, TargetAmount: 100, TargetUnitID: unitG.ID,
	}))

	// merge has a conversion from u1 to g (should be deleted upon merge because keep already has it)
	require.NoError(t, repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: merge.ID, UnitID: unit1.ID, TargetAmount: 120, TargetUnitID: unitG.ID,
	}))

	// merge has a conversion from u2 to g (should be moved to keep)
	require.NoError(t, repo.CreateConversion(&domain.FoodUnitConversion{
		FoodID: merge.ID, UnitID: unit2.ID, TargetAmount: 200, TargetUnitID: unitG.ID,
	}))

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	conversions, err := repo.ListConversions(keep.ID)
	require.NoError(t, err)

	assert.Len(t, conversions, 2)

	byUnit := make(map[uuid.UUID]float64)
	for _, c := range conversions {
		byUnit[c.UnitID] = c.TargetAmount
	}

	assert.Equal(t, 100.0, byUnit[unit1.ID])
	assert.Equal(t, 200.0, byUnit[unit2.ID])

	// verify merge food has no conversions left
	var remaining []domain.FoodUnitConversion
	require.NoError(t, db.Where("food_id = ?", merge.ID).Find(&remaining).Error)
	assert.Empty(t, remaining)
}

func TestFoodRepository_DefaultUnitUsageSince_AggregatesPerFoodUnit(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)
	since := time.Now().Add(-time.Hour)

	recipe := &domain.Recipe{Name: new("Test Recipe")}
	require.NoError(t, db.Create(recipe).Error)

	unitGram := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	unitCup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	require.NoError(t, db.Create(unitGram).Error)
	require.NoError(t, db.Create(unitCup).Error)

	// Candidate food: no default unit yet, used 3x with gram and 1x with cup.
	candidate := &domain.Food{Name: "Flour", Slug: "flour"}
	require.NoError(t, db.Create(candidate).Error)
	for i := 0; i < 3; i++ {
		require.NoError(t, db.Create(&domain.RecipeIngredient{
			RecipeID: recipe.ID, FoodID: &candidate.ID, UnitID: &unitGram.ID, RawText: "flour",
		}).Error)
	}
	require.NoError(t, db.Create(&domain.RecipeIngredient{
		RecipeID: recipe.ID, FoodID: &candidate.ID, UnitID: &unitCup.ID, RawText: "flour",
	}).Error)

	// Food already has a default unit: must be excluded even though it has ingredient rows.
	settled := &domain.Food{Name: "Sugar", Slug: "sugar", DefaultUnitID: &unitGram.ID}
	require.NoError(t, db.Create(settled).Error)
	require.NoError(t, db.Create(&domain.RecipeIngredient{
		RecipeID: recipe.ID, FoodID: &settled.ID, UnitID: &unitGram.ID, RawText: "sugar",
	}).Error)

	rows, err := repo.DefaultUnitUsageSince(since)
	require.NoError(t, err)

	byUnit := make(map[uuid.UUID]int)
	var updated time.Time
	for _, r := range rows {
		require.Equal(t, candidate.ID, r.FoodID, "settled food must not appear")
		byUnit[r.UnitID] = r.Count
		updated = r.Updated
	}
	assert.Equal(t, 3, byUnit[unitGram.ID])
	assert.Equal(t, 1, byUnit[unitCup.ID])

	total := 0
	for _, c := range byUnit {
		total += c
	}
	assert.Equal(t, 4, total, "per-unit counts must sum to the food's full ingredient usage")
	assert.WithinDuration(t, candidate.Updated, updated, time.Second)
}

func TestFoodRepository_BySlugsOrNames_MatchesEither(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	bySlug := &domain.Food{Name: "Carrot", Slug: "carrot"}
	byName := &domain.Food{Name: "onion", Slug: "yellow-onion"} // matched by name, not slug
	alias := &domain.Food{Name: "Tomat", Slug: "tomat"}
	require.NoError(t, db.Create(bySlug).Error)
	require.NoError(t, db.Create(byName).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, repo.Merge(bySlug.ID, alias.ID)) // alias.canonical_food_id now set

	found, err := repo.BySlugsOrNames([]string{"carrot", "tomat"}, []string{"onion"})
	require.NoError(t, err)

	ids := make([]uuid.UUID, len(found))
	for i, f := range found {
		ids[i] = f.ID
	}
	assert.Contains(t, ids, bySlug.ID)
	assert.Contains(t, ids, byName.ID)
	assert.NotContains(t, ids, alias.ID, "aliases must not resolve as merge candidates")
}

func TestFoodRepository_MergeCandidates_FiltersAliasesAndSince(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	stale := &domain.Food{Name: "Stale", Slug: "stale"}
	fresh := &domain.Food{Name: "Fresh", Slug: "fresh"}
	require.NoError(t, db.Create(stale).Error)
	require.NoError(t, db.Create(fresh).Error)

	since := stale.Updated.Add(time.Millisecond)
	// UpdateColumn bypasses the autoUpdateTime hook, so this sets updated without GORM
	// immediately overwriting it back to time.Now().
	require.NoError(t, db.Model(fresh).UpdateColumn("updated", since.Add(time.Millisecond)).Error)

	alias := &domain.Food{Name: "Alias", Slug: "alias"}
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, repo.Merge(fresh.ID, alias.ID))

	workSet, catalog, err := repo.MergeCandidates(since)
	require.NoError(t, err)

	workIDs := make([]uuid.UUID, len(workSet))
	for i, f := range workSet {
		workIDs[i] = f.ID
	}
	assert.Contains(t, workIDs, fresh.ID)
	assert.NotContains(t, workIDs, stale.ID, "work set excludes rows updated before since")
	assert.NotContains(t, workIDs, alias.ID, "work set excludes aliases")

	catalogIDs := make([]uuid.UUID, len(catalog))
	for i, f := range catalog {
		catalogIDs[i] = f.ID
	}
	assert.Contains(t, catalogIDs, stale.ID, "catalog includes canonical rows regardless of since")
	assert.NotContains(t, catalogIDs, alias.ID, "catalog excludes aliases")
}
