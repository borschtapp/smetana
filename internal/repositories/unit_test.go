package repositories_test

import (
	"testing"

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

func TestUnitRepository_BySlug_ResolvesAlias(t *testing.T) {
	repo, keep, _ := seedUnitAlias(t)

	got, err := repo.BySlug("pcs")
	require.NoError(t, err)
	assert.Equal(t, keep.ID, got.ID, "BySlug on an alias must resolve to the canonical unit")
}

func TestUnitRepository_BySlug_Missing_ReturnsNotFoundWithoutCreating(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	_, err := repo.BySlug("does-not-exist")
	require.ErrorIs(t, err, sentinels.ErrNotFound)

	var count int64
	require.NoError(t, db.Model(&domain.Unit{}).Where("slug = ?", "does-not-exist").Count(&count).Error)
	assert.Zero(t, count)
}

func TestUnitRepository_ByIDs_ReturnsFoundUnitsByID(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	gram := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	cup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	require.NoError(t, db.Create(gram).Error)
	require.NoError(t, db.Create(cup).Error)

	got, err := repo.ByIDs([]uuid.UUID{gram.ID, gram.ID, uuid.New()})

	require.NoError(t, err)
	require.Len(t, got, 1, "duplicates collapse and unknown IDs are left out")
	assert.Equal(t, "g", got[gram.ID].Slug)
}

func TestUnitRepository_Merge_RepointsFoodConversions(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "piece", Slug: "pc"}
	merge := &domain.Unit{Name: "pcs", Slug: "pcs"}
	gram := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	cup := &domain.Unit{Name: "cup", Slug: "cup", BaseFactor: 250}
	for _, u := range []*domain.Unit{keep, merge, gram, cup} {
		require.NoError(t, db.Create(u).Error)
	}

	foods := map[string]*domain.Food{}
	for _, name := range []string{"apple", "flour", "egg", "onion"} {
		foods[name] = &domain.Food{Name: name, Slug: name}
		require.NoError(t, db.Create(foods[name]).Error)
	}
	conversions := []*domain.FoodUnitConversion{
		{FoodID: foods["apple"].ID, UnitID: merge.ID, TargetAmount: 150, TargetUnitID: gram.ID}, // source repointed
		{FoodID: foods["flour"].ID, UnitID: cup.ID, TargetAmount: 2, TargetUnitID: merge.ID},    // target repointed
		{FoodID: foods["egg"].ID, UnitID: keep.ID, TargetAmount: 1, TargetUnitID: merge.ID},     // becomes pc -> pc: dropped
		{FoodID: foods["onion"].ID, UnitID: keep.ID, TargetAmount: 100, TargetUnitID: gram.ID},  // kept unit's conversion wins
		{FoodID: foods["onion"].ID, UnitID: merge.ID, TargetAmount: 90, TargetUnitID: gram.ID},  // duplicate after merge: dropped
	}
	for _, c := range conversions {
		require.NoError(t, db.Create(c).Error)
	}

	require.NoError(t, repo.Merge(keep.ID, merge.ID))

	var remaining []domain.FoodUnitConversion
	require.NoError(t, db.Find(&remaining).Error)
	got := make(map[uuid.UUID][]domain.FoodUnitConversion)
	for _, c := range remaining {
		assert.NotEqual(t, merge.ID, c.UnitID, "no conversion may still start at the merged unit")
		assert.NotEqual(t, merge.ID, c.TargetUnitID, "no conversion may still target the merged unit")
		got[c.FoodID] = append(got[c.FoodID], c)
	}

	require.Len(t, got[foods["apple"].ID], 1)
	assert.Equal(t, keep.ID, got[foods["apple"].ID][0].UnitID)
	require.Len(t, got[foods["flour"].ID], 1)
	assert.Equal(t, keep.ID, got[foods["flour"].ID][0].TargetUnitID)
	assert.Empty(t, got[foods["egg"].ID], "a conversion between the merged units must be dropped")
	require.Len(t, got[foods["onion"].ID], 1)
	assert.Equal(t, 100.0, got[foods["onion"].ID][0].TargetAmount, "the kept unit's conversion must win")
}

func TestUnitRepository_Merge_RepointsExistingAliases(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	keep := &domain.Unit{Name: "pc", Slug: "pc"}
	merge := &domain.Unit{Name: "piece", Slug: "piece"}
	alias := &domain.Unit{Name: "pcs", Slug: "pcs"}
	for _, u := range []*domain.Unit{keep, merge, alias} {
		require.NoError(t, db.Create(u).Error)
	}
	require.NoError(t, repo.Merge(merge.ID, alias.ID)) // pcs -> piece

	require.NoError(t, repo.Merge(keep.ID, merge.ID)) // piece -> pc

	got, err := repo.BySlug("pcs")
	require.NoError(t, err)
	assert.Equal(t, keep.ID, got.ID, "an alias of the merged unit must resolve to the kept unit, not to another alias")
}

func TestUnitRepository_CanonicalIDs_MapsOnlyAliases(t *testing.T) {
	repo, keep, merge := seedUnitAlias(t)

	got, err := repo.CanonicalIDs([]uuid.UUID{keep.ID, merge.ID, uuid.New()})

	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]uuid.UUID{merge.ID: keep.ID}, got)
}

func TestUnitRepository_ByID_ResolvesAlias(t *testing.T) {
	repo, keep, merge := seedUnitAlias(t)

	got, err := repo.ByID(merge.ID)

	require.NoError(t, err)
	assert.Equal(t, keep.ID, got.ID, "ByID on an alias must return the canonical unit")
}

func TestUnitRepository_Update_ChangesNameOnly(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	piece := &domain.Unit{Name: "piece", Slug: domain.PieceUnitSlug}
	require.NoError(t, db.Create(piece).Error)

	require.NoError(t, repo.Update(&domain.Unit{ID: piece.ID, Name: "Stück", Slug: "stueck"}))

	got, err := repo.BySlug(domain.PieceUnitSlug)
	require.NoError(t, err, "unitless amounts find the piece unit by its slug, so Update must never change it")
	assert.Equal(t, piece.ID, got.ID)
	assert.Equal(t, "Stück", got.Name)
}

// seedUnitAlias creates "pc" and "pcs" and merges pcs into pc, so pcs is an alias of keep.
func seedUnitAlias(t *testing.T) (repo domain.UnitRepository, keep, merge *domain.Unit) {
	t.Helper()
	db := openPrivateTestDB(t)
	repo = repositories.NewUnitRepository(db)
	keep = &domain.Unit{Name: "piece", Slug: domain.PieceUnitSlug}
	merge = &domain.Unit{Name: "pcs", Slug: "pcs"}
	require.NoError(t, db.Create(keep).Error)
	require.NoError(t, db.Create(merge).Error)
	require.NoError(t, repo.Merge(keep.ID, merge.ID))
	return repo, keep, merge
}

func TestUnitRepository_Merge_PieceUnit_CannotBeMergedAway(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	piece := &domain.Unit{Name: "piece", Slug: domain.PieceUnitSlug}
	gram := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	require.NoError(t, db.Create(piece).Error)
	require.NoError(t, db.Create(gram).Error)

	require.ErrorIs(t, repo.Merge(gram.ID, piece.ID), sentinels.ErrForbidden, "the piece unit must not become an alias")
}

func TestUnitRepository_Merge_IntoPieceUnit_OnlyCountUnits(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUnitRepository(db)

	piece := &domain.Unit{Name: "piece", Slug: domain.PieceUnitSlug}
	pcs := &domain.Unit{Name: "pcs", Slug: "pcs"}
	gram := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	for _, u := range []*domain.Unit{piece, pcs, gram} {
		require.NoError(t, db.Create(u).Error)
	}

	err := repo.Merge(piece.ID, gram.ID)
	var se *sentinels.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 422, se.Status, "merging a measurable unit would make the piece unit convertible")

	require.NoError(t, repo.Merge(piece.ID, pcs.ID), "a duplicate count unit can be merged into the piece unit")
	got, err := repo.BySlug(domain.PieceUnitSlug)
	require.NoError(t, err)
	assert.False(t, got.Convertible(), "the piece unit must stay a count unit")
}

func TestUnitRepository_ByIDs_ResolvesAliasesLikeByID(t *testing.T) {
	repo, keep, merge := seedUnitAlias(t)

	got, err := repo.ByIDs([]uuid.UUID{merge.ID, keep.ID, uuid.New()})

	require.NoError(t, err)
	require.Len(t, got, 2, "unknown IDs are left out")
	assert.Equal(t, keep.ID, got[merge.ID].ID, "an alias ID maps to its canonical unit")
	assert.Equal(t, keep.ID, got[keep.ID].ID)
}
