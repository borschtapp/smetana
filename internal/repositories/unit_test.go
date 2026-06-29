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
)

func TestUnitRepository_Merge_SetsCanonicalUnitID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "kilogram", Slug: "kilogram", BaseFactor: 1}
	merge := &domain.Unit{Name: "kilo", Slug: "kilo", BaseFactor: 1}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var alias domain.Unit
	require.NoError(t, db.First(&alias, "id = ?", merge.ID).Error)
	require.NotNil(t, alias.CanonicalUnitID, "merged unit must have canonical_unit_id set")
	assert.Equal(t, keep.ID, *alias.CanonicalUnitID)
}

func TestUnitRepository_Merge_ReparentsDerivedUnits(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	// keep is a base unit; merge is a synonym base unit with a derived unit pointing to it
	keep := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	merge := &domain.Unit{Name: "gramme", Slug: "gramme", BaseFactor: 1}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	derived := &domain.Unit{Name: "milligram", Slug: "mg", BaseFactor: 0.001, BaseUnitID: &merge.ID}
	require.NoError(t, db.Create(derived).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var updated domain.Unit
	require.NoError(t, db.First(&updated, "id = ?", derived.ID).Error)
	require.NotNil(t, updated.BaseUnitID, "derived unit must still have a base unit after merge")
	assert.Equal(t, keep.ID, *updated.BaseUnitID, "derived unit must be re-parented to the kept unit")
}

func TestUnitRepository_Merge_FindOrCreate_ResolvesAlias(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "kilogram", Slug: "kilogram", BaseFactor: 1}
	merge := &domain.Unit{Name: "kg", Slug: "kg", BaseFactor: 1}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)
	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	lookup := &domain.Unit{Name: "kg"}
	require.NoError(t, repo.FindOrCreate(lookup))

	assert.Equal(t, keep.ID, lookup.ID, "FindOrCreate on the old name must resolve to the canonical unit")
}

func TestUnitRepository_Merge_Search_ExcludesAliases(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "kilogram", Slug: "kilogram", BaseFactor: 1}
	alias := &domain.Unit{Name: "kilo", Slug: "kilo", BaseFactor: 1}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, repo.Merge(keep.ID, alias.ID))

	units, total, err := repo.Search("kilo", nil, 0, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total, "alias must not appear in search results")
	assert.Equal(t, keep.ID, units[0].ID)
}

func TestUnitRepository_Merge_KeepIsAlias_ReturnsNotFound(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	canonical := &domain.Unit{Name: "litre", Slug: "litre", BaseFactor: 1}
	alias := &domain.Unit{Name: "liter", Slug: "liter", BaseFactor: 1}
	other := &domain.Unit{Name: "milliliter", Slug: "ml", BaseFactor: 0.001}
	require.NoError(t, db.Create(canonical).Error)
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, repo.Merge(canonical.ID, alias.ID))

	err := repo.Merge(alias.ID, other.ID)
	require.ErrorIs(t, err, sentinels.ErrNotFound, "alias cannot be used as the keep target")
}

func TestUnitRepository_Merge_InheritsBaseUnitFromMerged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	base := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	require.NoError(t, db.Create(base).Error)

	keep := &domain.Unit{Name: "kilogram", Slug: "kilogram"}
	merge := &domain.Unit{Name: "kilo", Slug: "kilo", BaseUnitID: &base.ID, BaseFactor: 1000}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Unit
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	require.NotNil(t, result.BaseUnitID, "keep unit must inherit base_unit_id when it had none")
	assert.Equal(t, base.ID, *result.BaseUnitID)
	assert.Equal(t, 1000.0, result.BaseFactor, "keep unit must inherit base_factor when it was zero")
}

func TestUnitRepository_Merge_InheritsImperialFromMerged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "fl oz", Slug: "fl-oz", BaseFactor: 1, Imperial: false}
	merge := &domain.Unit{Name: "fluid ounce", Slug: "fluid-ounce", BaseFactor: 1, Imperial: true}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Unit
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	assert.True(t, result.Imperial, "keep unit must become imperial when merged unit was imperial")
}

func TestUnitRepository_Merge_DoesNotOverwriteExistingBaseUnit(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	base1 := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}
	base2 := &domain.Unit{Name: "litre", Slug: "litre", BaseFactor: 1}
	require.NoError(t, db.Create(base1).Error)
	require.NoError(t, db.Create(base2).Error)

	keep := &domain.Unit{Name: "kilogram", Slug: "kilogram", BaseUnitID: &base1.ID, BaseFactor: 1000}
	merge := &domain.Unit{Name: "kilo", Slug: "kilo", BaseUnitID: &base2.ID, BaseFactor: 500}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var result domain.Unit
	require.NoError(t, db.First(&result, "id = ?", keep.ID).Error)
	assert.Equal(t, base1.ID, *result.BaseUnitID, "keep unit must retain its own base_unit_id when both have one")
	assert.Equal(t, 1000.0, result.BaseFactor, "keep unit must retain its own base_factor when both have one")
}

func TestUnitRepository_Detach_ClearsAlias(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Slug: "gram", Name: "gram"}
	alias := &domain.Unit{Slug: "grams", Name: "grams"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(alias).Error)

	require.NoError(t, repo.Merge(keep.ID, alias.ID))
	require.NoError(t, repo.Detach(alias.ID))

	var detached domain.Unit
	require.NoError(t, db.First(&detached, "id = ?", alias.ID).Error)
	assert.Nil(t, detached.CanonicalUnitID)
}

func TestUnitRepository_NonConvertibleUsageSince_ExcludesConvertibleAndAlreadyBridged(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)
	since := time.Now().Add(-time.Hour)

	recipe := &domain.Recipe{Name: new("Test Recipe")}
	require.NoError(t, db.Create(recipe).Error)

	piece := &domain.Unit{Name: "piece", Slug: "piece", BaseFactor: 0} // non-convertible
	gram := &domain.Unit{Name: "gram", Slug: "gram", BaseFactor: 1}    // convertible
	require.NoError(t, db.Create(piece).Error)
	require.NoError(t, db.Create(gram).Error)

	needsConversion := &domain.Food{Name: "Egg", Slug: "egg"}
	convertibleUnit := &domain.Food{Name: "Rice", Slug: "rice"}
	alreadyBridged := &domain.Food{Name: "Bunch", Slug: "bunch"}
	require.NoError(t, db.Create(needsConversion).Error)
	require.NoError(t, db.Create(convertibleUnit).Error)
	require.NoError(t, db.Create(alreadyBridged).Error)

	require.NoError(t, db.Create(&domain.RecipeIngredient{
		RecipeID: recipe.ID, FoodID: &needsConversion.ID, UnitID: &piece.ID, RawText: "egg",
	}).Error)
	require.NoError(t, db.Create(&domain.RecipeIngredient{
		RecipeID: recipe.ID, FoodID: &convertibleUnit.ID, UnitID: &gram.ID, RawText: "rice",
	}).Error)
	require.NoError(t, db.Create(&domain.RecipeIngredient{
		RecipeID: recipe.ID, FoodID: &alreadyBridged.ID, UnitID: &piece.ID, RawText: "bunch",
	}).Error)
	require.NoError(t, db.Create(&domain.FoodUnitConversion{
		FoodID: alreadyBridged.ID, UnitID: piece.ID, TargetAmount: 100, TargetUnitID: gram.ID,
	}).Error)

	pairs, err := repo.NonConvertibleUsageSince(since)
	require.NoError(t, err)

	require.Len(t, pairs, 1, "only the food with a non-convertible unit and no existing conversion must be returned")
	assert.Equal(t, domain.FoodUnitPair{FoodID: needsConversion.ID, UnitID: piece.ID}, pairs[0])
}

func TestUnitRepository_MergeCandidates_FiltersAliasesAndSince(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	stale := &domain.Unit{Name: "Stale", Slug: "stale", BaseFactor: 1}
	fresh := &domain.Unit{Name: "Fresh", Slug: "fresh", BaseFactor: 1}
	require.NoError(t, db.Create(stale).Error)
	require.NoError(t, db.Create(fresh).Error)

	since := stale.Updated.Add(time.Millisecond)
	require.NoError(t, db.Model(fresh).UpdateColumn("updated", since.Add(time.Millisecond)).Error)

	alias := &domain.Unit{Name: "Alias", Slug: "alias", BaseFactor: 1}
	require.NoError(t, db.Create(alias).Error)
	require.NoError(t, repo.Merge(fresh.ID, alias.ID))

	workSet, catalog, err := repo.MergeCandidates(since)
	require.NoError(t, err)

	workIDs := make([]uuid.UUID, len(workSet))
	for i, u := range workSet {
		workIDs[i] = u.ID
	}
	assert.Contains(t, workIDs, fresh.ID)
	assert.NotContains(t, workIDs, stale.ID, "work set excludes rows updated before since")
	assert.NotContains(t, workIDs, alias.ID, "work set excludes aliases")

	catalogIDs := make([]uuid.UUID, len(catalog))
	for i, u := range catalog {
		catalogIDs[i] = u.ID
	}
	assert.Contains(t, catalogIDs, stale.ID, "catalog includes canonical rows regardless of since")
	assert.NotContains(t, catalogIDs, alias.ID, "catalog excludes aliases")
}
