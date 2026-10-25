package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/database"
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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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
	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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
	require.NoError(t, repo.Merge(keep.ID, alias.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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
	require.NoError(t, repo.Merge(canonical.ID, alias.ID, nil))

	err := repo.Merge(alias.ID, other.ID, nil)
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
	require.NoError(t, repo.Merge(canonical.ID, alias.ID, nil))

	err := repo.Merge(other.ID, alias.ID, nil)
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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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
	require.ErrorIs(t, err, sentinels.ErrAlreadyExists)
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

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))

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

func TestFoodRepository_UpdateConversion_OtherFood_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	milk := &domain.Food{Name: "Milk", Slug: "milk"}
	egg := &domain.Food{Name: "Egg", Slug: "egg"}
	require.NoError(t, db.Create(milk).Error)
	require.NoError(t, db.Create(egg).Error)

	unitCup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	unitG := &domain.Unit{Name: "g", Slug: "g", BaseFactor: 1}
	require.NoError(t, db.Create(unitCup).Error)
	require.NoError(t, db.Create(unitG).Error)

	conv := &domain.FoodUnitConversion{FoodID: milk.ID, UnitID: unitCup.ID, TargetAmount: 240, TargetUnitID: unitG.ID}
	require.NoError(t, repo.CreateConversion(conv))

	err := repo.UpdateConversion(&domain.FoodUnitConversion{
		ID: conv.ID, FoodID: egg.ID, TargetAmount: 1, TargetUnitID: unitG.ID,
	})
	require.ErrorIs(t, err, sentinels.ErrNotFound)

	stored, err := repo.ConversionByID(milk.ID, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, 240.0, stored.TargetAmount, "conversion of another food must not be modified")
}

func TestFoodRepository_Merge_RepointsExistingAliases(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Tomato", Slug: "tomato"}
	merge := &domain.Food{Name: "Tomatoes", Slug: "tomatoes"}
	alias := &domain.Food{Name: "Tomatos", Slug: "tomatos"}
	for _, f := range []*domain.Food{keep, merge, alias} {
		require.NoError(t, db.Create(f).Error)
	}
	require.NoError(t, repo.Merge(merge.ID, alias.ID, nil)) // tomatos -> tomatoes

	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil)) // tomatoes -> tomato

	var stored domain.Food
	require.NoError(t, db.First(&stored, "id = ?", alias.ID).Error)
	require.NotNil(t, stored.CanonicalFoodID)
	assert.Equal(t, keep.ID, *stored.CanonicalFoodID, "an alias of the merged food must point at the kept food directly")

	found := &domain.Food{Name: "Tomatos", Slug: "tomatos"}
	require.NoError(t, repo.FindOrCreate(found))
	assert.Equal(t, keep.ID, found.ID)
}

func TestFoodRepository_CanonicalIDs_MapsOnlyAliases(t *testing.T) {
	repo, keep, merge := seedFoodAlias(t)

	got, err := repo.CanonicalIDs([]uuid.UUID{keep.ID, merge.ID, uuid.New()})

	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]uuid.UUID{merge.ID: keep.ID}, got)
}

func TestFoodRepository_ByID_ResolvesAlias(t *testing.T) {
	repo, keep, merge := seedFoodAlias(t)

	got, err := repo.ByID(merge.ID)

	require.NoError(t, err)
	assert.Equal(t, keep.ID, got.ID, "ByID on an alias must return the canonical food")
}

func TestFoodRepository_DeleteConversion_Missing_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	err := repo.DeleteConversion(uuid.New(), uuid.New())

	require.ErrorIs(t, err, sentinels.ErrNotFound)
}

// seedFoodAlias creates "tomato" and "tomatoes" and merges tomatoes into tomato, so merge is an alias of keep.
func seedFoodAlias(t *testing.T) (repo domain.FoodRepository, keep, merge *domain.Food) {
	t.Helper()
	db := openPrivateTestDB(t)
	repo = repositories.NewFoodRepository(db)
	keep = &domain.Food{Name: "Tomato", Slug: "tomato"}
	merge = &domain.Food{Name: "Tomatoes", Slug: "tomatoes"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)
	require.NoError(t, repo.Merge(keep.ID, merge.ID, nil))
	return repo, keep, merge
}

func TestFoodRepository_ByIDs_ResolvesAliasesLikeByID(t *testing.T) {
	repo, keep, merge := seedFoodAlias(t)

	got, err := repo.ByIDs([]uuid.UUID{merge.ID})

	require.NoError(t, err)
	require.Contains(t, got, merge.ID)
	assert.Equal(t, keep.ID, got[merge.ID].ID, "an alias ID maps to its canonical food, loaded on demand")
}

func TestFoodRepository_Merge_ListHoldsBothFoods_CombinesItems(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewFoodRepository(db)

	keep := &domain.Food{Name: "Tomato", Slug: "tomato"}
	merge := &domain.Food{Name: "Tomatoes", Slug: "tomatoes"}
	g := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	ml := &domain.Unit{Name: "millilitre", Slug: "ml", BaseFactor: 1}
	for _, v := range []any{keep, merge, g, ml} {
		require.NoError(t, db.Create(v).Error)
	}
	sameUnitList, otherUnitList, mergeOnlyList := seedShoppingList(t, db), seedShoppingList(t, db), seedShoppingList(t, db)

	kept := &domain.ShoppingItem{ShoppingListID: sameUnitList, FoodID: &keep.ID, UnitID: &g.ID, Amount: new(200.0), Text: "tomato", IsBought: true}
	summed := &domain.ShoppingItem{ShoppingListID: sameUnitList, FoodID: &merge.ID, UnitID: &g.ID, Amount: new(300.0), Text: "tomatoes"}
	otherKept := &domain.ShoppingItem{ShoppingListID: otherUnitList, FoodID: &keep.ID, UnitID: &g.ID, Amount: new(200.0), Text: "tomato"}
	unlinked := &domain.ShoppingItem{ShoppingListID: otherUnitList, FoodID: &merge.ID, UnitID: &ml.ID, Amount: new(50.0), Text: "tomato juice"}
	moved := &domain.ShoppingItem{ShoppingListID: mergeOnlyList, FoodID: &merge.ID, Text: "tomatoes"}
	for _, item := range []*domain.ShoppingItem{kept, summed, otherKept, unlinked, moved} {
		require.NoError(t, db.Create(item).Error)
	}

	sumSameUnit := func(target, incoming *domain.ShoppingItem) (bool, error) {
		if *target.UnitID != *incoming.UnitID {
			return false, nil
		}
		target.Amount = new(*target.Amount + *incoming.Amount)
		target.IsBought, target.Text = false, incoming.Text
		return true, nil
	}
	require.NoError(t, repo.Merge(keep.ID, merge.ID, sumSameUnit), "a list holding both foods must merge")

	var keptStored, unlinkedStored, movedStored domain.ShoppingItem
	require.NoError(t, db.First(&keptStored, "id = ?", kept.ID).Error)
	assert.InDelta(t, 500.0, *keptStored.Amount, 1e-9, "a combined amount is stored on the kept item")
	assert.False(t, keptStored.IsBought, "the combined bought state is stored")
	assert.Equal(t, "tomatoes", keptStored.Text)
	assert.ErrorIs(t, db.First(&domain.ShoppingItem{}, "id = ?", summed.ID).Error, gorm.ErrRecordNotFound)

	require.NoError(t, db.First(&unlinkedStored, "id = ?", unlinked.ID).Error)
	assert.Nil(t, unlinkedStored.FoodID, "an item that cannot be combined loses its food link")
	assert.Equal(t, "tomato juice", unlinkedStored.Text)

	require.NoError(t, db.First(&movedStored, "id = ?", moved.ID).Error)
	assert.Equal(t, keep.ID, *movedStored.FoodID)
}

func TestFlattenAliases_RepointsLegacyChains(t *testing.T) {
	db := openPrivateTestDB(t)

	root := &domain.Food{Name: "Tomato", Slug: "tomato"}
	mid := &domain.Food{Name: "Tomatoes", Slug: "tomatoes"}
	leaf := &domain.Food{Name: "Tomatos", Slug: "tomatos"}
	for _, f := range []*domain.Food{root, mid, leaf} {
		require.NoError(t, db.Create(f).Error)
	}
	// What the old Merge left behind: tomatos -> tomatoes -> tomato.
	require.NoError(t, db.Model(mid).Update("canonical_food_id", root.ID).Error)
	require.NoError(t, db.Model(leaf).Update("canonical_food_id", mid.ID).Error)

	require.NoError(t, database.FlattenAliases(db))

	var leafStored, midStored domain.Food
	require.NoError(t, db.First(&leafStored, "id = ?", leaf.ID).Error)
	assert.Equal(t, root.ID, *leafStored.CanonicalFoodID, "a chained alias must point at the root")
	require.NoError(t, db.First(&midStored, "id = ?", mid.ID).Error)
	assert.Equal(t, root.ID, *midStored.CanonicalFoodID)

	got, err := repositories.NewFoodRepository(db).CanonicalIDs([]uuid.UUID{leaf.ID})
	require.NoError(t, err)
	assert.Equal(t, root.ID, got[leaf.ID])
}
