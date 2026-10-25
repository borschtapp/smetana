package services_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/services"
)

type fakeUnitRepo struct {
	domain.UnitRepository
	units       map[uuid.UUID]*domain.Unit
	byIDsCalls  int
	bySlugCalls int
	merged      [][2]uuid.UUID // keep, merge pairs passed to Merge
}

func newFakeUnitRepo(units ...*domain.Unit) *fakeUnitRepo {
	r := &fakeUnitRepo{units: make(map[uuid.UUID]*domain.Unit)}
	for _, u := range units {
		r.units[u.ID] = u
	}
	return r
}

func (r *fakeUnitRepo) ByID(id uuid.UUID) (*domain.Unit, error) {
	if u, ok := r.units[id]; ok {
		return u, nil
	}
	return nil, sentinels.ErrUnitConversion
}

func (r *fakeUnitRepo) ByBase(baseUnitID uuid.UUID, imperial bool) ([]domain.Unit, error) {
	var out []domain.Unit
	for _, u := range r.units {
		base := u.ID
		if u.BaseUnitID != nil {
			base = *u.BaseUnitID
		}
		if base == baseUnitID && u.Imperial == imperial {
			out = append(out, *u)
		}
	}
	return out, nil
}

func (r *fakeUnitRepo) Search(query string, imperial *bool, offset, limit int) ([]domain.Unit, int64, error) {
	var out []domain.Unit
	for _, u := range r.units {
		if imperial != nil && u.Imperial != *imperial {
			continue
		}
		out = append(out, *u)
	}
	total := int64(len(out))
	if offset >= len(out) {
		return nil, total, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], total, nil
}

func (r *fakeUnitRepo) FindOrCreate(unit *domain.Unit) error {
	if unit.ID == uuid.Nil {
		unit.ID, _ = uuid.NewV7()
	}
	r.units[unit.ID] = unit
	return nil
}

func (r *fakeUnitRepo) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Unit, error) {
	r.byIDsCalls++
	out := make(map[uuid.UUID]*domain.Unit, len(ids))
	for _, id := range ids {
		if u, ok := r.units[id]; ok {
			if u.CanonicalUnitID != nil {
				u = r.units[*u.CanonicalUnitID]
			}
			out[id] = u
		}
	}
	return out, nil
}

func (r *fakeUnitRepo) CanonicalIDs(ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	out := make(map[uuid.UUID]uuid.UUID)
	for _, id := range ids {
		if u, ok := r.units[id]; ok && u.CanonicalUnitID != nil {
			out[id] = *u.CanonicalUnitID
		}
	}
	return out, nil
}

func (r *fakeUnitRepo) BySlug(slug string) (*domain.Unit, error) {
	r.bySlugCalls++
	for _, u := range r.units {
		if u.Slug == slug {
			return u, nil
		}
	}
	return nil, sentinels.ErrNotFound
}

func (r *fakeUnitRepo) Merge(keepID, mergeID uuid.UUID) error {
	r.merged = append(r.merged, [2]uuid.UUID{keepID, mergeID})
	return nil
}
func (r *fakeUnitRepo) Update(_ *domain.Unit) error { return nil }

var (
	idG   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	idKg  = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	idMg  = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	idOz  = uuid.MustParse("00000000-0000-0000-0000-000000000004")
	idMl  = uuid.MustParse("00000000-0000-0000-0000-000000000005")
	idL   = uuid.MustParse("00000000-0000-0000-0000-000000000006")
	idTsp = uuid.MustParse("00000000-0000-0000-0000-000000000007")
	idPc  = uuid.MustParse("00000000-0000-0000-0000-000000000008")
)

func pieceFixture() *domain.Unit {
	return &domain.Unit{ID: idPc, Slug: domain.PieceUnitSlug, Name: "piece"}
}

func massFixtures() []*domain.Unit {
	return []*domain.Unit{
		{ID: idG, Slug: "g", Name: "gram", BaseFactor: 1, Imperial: false},
		{ID: idKg, Slug: "kg", Name: "kilogram", BaseUnitID: &idG, BaseFactor: 1000, Imperial: false},
		{ID: idMg, Slug: "mg", Name: "milligram", BaseUnitID: &idG, BaseFactor: 0.001, Imperial: false},
		{ID: idOz, Slug: "oz", Name: "ounce", BaseUnitID: &idG, BaseFactor: 28, Imperial: true},
	}
}

func volumeFixtures() []*domain.Unit {
	return []*domain.Unit{
		{ID: idMl, Slug: "ml", Name: "milliliter", BaseFactor: 1, Imperial: false},
		{ID: idL, Slug: "l", Name: "liter", BaseUnitID: &idMl, BaseFactor: 1000, Imperial: false},
		{ID: idTsp, Slug: "tsp", Name: "teaspoon", BaseUnitID: &idMl, BaseFactor: 5, Imperial: true},
	}
}

func TestUnitConverter_Convert_SameUnit_ReturnsAmountUnchanged(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.NewConverter().Convert(42, &idKg, &idKg, nil)

	require.NoError(t, err)
	assert.InDelta(t, 42.0, result, 1e-9, "same-unit conversion must be identity")
}

func TestUnitConverter_Convert_MetricToMetric_ScalesCorrectly(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.NewConverter().Convert(1, &idKg, &idG, nil)

	require.NoError(t, err)
	assert.InDelta(t, 1000.0, result, 1e-6, "1 kg must equal 1000 g")
}

func TestUnitConverter_Convert_MetricToImperial_ScalesCorrectly(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.NewConverter().Convert(100, &idG, &idOz, nil)

	require.NoError(t, err)
	assert.InDelta(t, 3.572, result, 0.001, "100 g must convert to ~3.572 oz")
}

func TestUnitConverter_Convert_IncompatibleDimensions_ReturnsUnprocessable(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	_, err := svc.NewConverter().Convert(1, &idG, &idMl, nil)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "mass and volume are incompatible dimensions")
}

func TestUnitConverter_Convert_UnknownFromUnit_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))
	unknown := uuid.New()

	_, err := svc.NewConverter().Convert(1, &unknown, &idG, nil)

	require.ErrorIs(t, err, sentinels.ErrNotFound, "an unknown unit is a lookup failure, not an incompatible conversion")
}

func TestUnitConverter_Convert_UnknownToUnit_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))
	unknown := uuid.New()

	_, err := svc.NewConverter().Convert(1, &idG, &unknown, nil)

	require.ErrorIs(t, err, sentinels.ErrNotFound, "an unknown unit is a lookup failure, not an incompatible conversion")
}

func TestUnitService_BestUnit_LargeAmount_PrefersKilogram(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	best, err := svc.BestUnit(5000, idG, false)

	require.NoError(t, err)
	assert.Equal(t, "kg", best.Slug, "5000 g is most readable as 5 kg")
}

func TestUnitService_BestUnit_SmallAmount_PrefersMilligram(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	best, err := svc.BestUnit(0.005, idG, false)

	require.NoError(t, err)
	assert.Equal(t, "mg", best.Slug, "0.005 g is most readable as 5 mg")
}

func TestUnitService_BestUnit_ImperialFlag_ReturnsImperialUnit(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	best, err := svc.BestUnit(100, idG, true)

	require.NoError(t, err)
	assert.Equal(t, "oz", best.Slug, "imperial best unit for ~100 g must be oz")
}

func TestUnitService_BestUnit_FromDerivedUnit_ResolvesViaBase(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	best, err := svc.BestUnit(2, idKg, false)

	require.NoError(t, err)
	assert.Equal(t, "kg", best.Slug, "2 kg expressed from kg input must stay kg")
}

func TestUnitService_BestUnit_NoCandidates_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	_, err := svc.BestUnit(1, idMl, true)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "no candidates must return not found")
}

func TestUnitService_BestUnit_UnknownUnit_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	_, err := svc.BestUnit(1, uuid.New(), false)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "unknown source unit must return not found")
}

func TestUnitService_Search_ImperialFilter_ReturnsOnlyImperialUnits(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	units, total, err := svc.Search("", new(true), 0, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "fixture has exactly 2 imperial units (oz, tsp)")
	for _, u := range units {
		assert.True(t, u.Imperial, "all returned units must be imperial")
	}
}

func TestUnitService_Search_MetricFilter_ReturnsOnlyMetricUnits(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	units, _, err := svc.Search("", new(false), 0, 20)

	require.NoError(t, err)
	for _, u := range units {
		assert.False(t, u.Imperial, "all returned units must be metric")
	}
}

func TestUnitService_Search_NoFilter_ReturnsAllUnits(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	_, total, err := svc.Search("", nil, 0, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(len(all)), total, "unfiltered search must return all units")
}

func TestUnitService_Merge_SameID_ReturnsBadRequest(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo())
	id := uuid.New()

	err := svc.Merge(id, id)

	var se *sentinels.Error
	require.ErrorAs(t, err, &se, "same-ID merge must return a typed sentinel error")
	assert.Equal(t, 400, se.Status)
}

func TestUnitConverter_Convert_FromSideBridge(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	// 1 cup = 250ml. Say we want to convert 500ml to oz.
	// Wait, we don't have cup in volumeFixtures, we have tsp and ml.
	// We want to convert 100 tsp to oz (volume to mass).
	// We bridge from ml to g. Say 1 ml = 2 g (so 1 tsp = 5 ml = 10 g).
	// Target is oz.
	// Bridge definition: Unit=ml, TargetAmount=2, TargetUnit=g
	// fromUnit=tsp, toUnit=oz
	conversions := []domain.FoodUnitConversion{
		{
			UnitID:       idMl,
			TargetAmount: 2.0,
			TargetUnitID: idG,
		},
	}

	// 100 tsp = 500 ml
	// 500 ml -> bridge -> 1000 g
	// 1000 g -> oz (~35.714)
	res, err := svc.NewConverter().Convert(100, &idTsp, &idOz, conversions)

	require.NoError(t, err)
	assert.InDelta(t, 1000.0/28.0, res, 1e-6)
}

func TestUnitConverter_Convert_ToSideBridge(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	// Converting 10 oz to tsp.
	// Bridge: 1 ml = 2 g.
	// Convert 10 oz -> g -> bridge to ml -> tsp
	// 10 oz = 280 g
	// 280 g / 2 = 140 ml
	// 140 ml / 5 = 28 tsp
	conversions := []domain.FoodUnitConversion{
		{
			UnitID:       idMl,
			TargetAmount: 2.0,
			TargetUnitID: idG,
		},
	}

	res, err := svc.NewConverter().Convert(10, &idOz, &idTsp, conversions)

	require.NoError(t, err)
	assert.InDelta(t, 28.0, res, 1e-6)
}

func TestUnitConverter_Convert_NoBridge_ReturnsError(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	// Unrelated bridge
	conversions := []domain.FoodUnitConversion{
		{
			UnitID:       uuid.New(),
			TargetAmount: 1,
			TargetUnitID: uuid.New(),
		},
	}

	_, err := svc.NewConverter().Convert(10, &idOz, &idTsp, conversions)

	require.Error(t, err)
	var se *sentinels.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, 422, se.Status)
}

func TestUnitConverter_Convert_MissingUnit_CountsAsPiece(t *testing.T) {
	all := append(massFixtures(), pieceFixture())
	svc := services.NewUnitService(newFakeUnitRepo(all...))
	conversions := []domain.FoodUnitConversion{{UnitID: idPc, TargetAmount: 200, TargetUnitID: idG}}

	toMass, err := svc.NewConverter().Convert(2, nil, &idKg, conversions)
	require.NoError(t, err)
	assert.InDelta(t, 0.4, toMass, 1e-9, "2 pieces x 200 g = 0.4 kg")

	fromMass, err := svc.NewConverter().Convert(600, &idG, nil, conversions)
	require.NoError(t, err)
	assert.InDelta(t, 3.0, fromMass, 1e-9, "600 g / 200 g = 3 pieces")
}

func TestUnitConverter_Convert_BothMissing_ReturnsAmount(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo()) // no piece unit: must not be looked up

	res, err := svc.NewConverter().Convert(7, nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, 7.0, res)
}

func TestUnitConverter_Convert_LoadsUnitsInOneQuery(t *testing.T) {
	repo := newFakeUnitRepo(append(massFixtures(), volumeFixtures()...)...)
	svc := services.NewUnitService(repo)
	conversions := []domain.FoodUnitConversion{
		{UnitID: uuid.New(), TargetAmount: 1, TargetUnitID: uuid.New()}, // unrelated, tried first
		{UnitID: idMl, TargetAmount: 2.0, TargetUnitID: idG},
	}

	res, err := svc.NewConverter().Convert(100, &idTsp, &idOz, conversions)

	require.NoError(t, err)
	assert.InDelta(t, 1000.0/28.0, res, 1e-6)
	assert.Equal(t, 1, repo.byIDsCalls, "all units must be loaded in a single batch")
}

func TestUnitConverter_Convert_ReusesLoadedUnits(t *testing.T) {
	repo := newFakeUnitRepo(append(massFixtures(), pieceFixture())...)
	converter := services.NewUnitService(repo).NewConverter()
	conversions := []domain.FoodUnitConversion{{UnitID: idPc, TargetAmount: 200, TargetUnitID: idG}}

	for range 3 {
		res, err := converter.Convert(2, nil, &idKg, conversions)
		require.NoError(t, err)
		assert.InDelta(t, 0.4, res, 1e-9)
	}

	assert.Equal(t, 1, repo.byIDsCalls, "units must be loaded once per converter")
	assert.Equal(t, 1, repo.bySlugCalls, "the piece unit must be resolved once per converter")
}

func TestUnitConverter_Convert_MissingPieceUnit_LookedUpOnce(t *testing.T) {
	repo := newFakeUnitRepo(massFixtures()...) // no piece unit
	converter := services.NewUnitService(repo).NewConverter()

	for range 3 {
		_, err := converter.Convert(2, nil, &idKg, nil)
		require.ErrorIs(t, err, sentinels.ErrNotFound)
	}

	assert.Equal(t, 1, repo.bySlugCalls, "a missing piece unit is looked up once")
}

func TestUnitConverter_Expect_FetchesWithFirstLookupOnly(t *testing.T) {
	repo := newFakeUnitRepo(massFixtures()...)
	converter := services.NewUnitService(repo).NewConverter()
	converter.Expect(idKg, idMg, idOz, idG)

	_, err := converter.Convert(5, &idG, &idG, nil)
	require.NoError(t, err)
	assert.Zero(t, repo.byIDsCalls, "expected units must not be fetched while no conversion needs a lookup")

	kg, err := converter.Convert(1, &idKg, &idG, nil)
	require.NoError(t, err)
	oz, err := converter.Convert(1, &idOz, &idMg, nil)
	require.NoError(t, err)

	assert.InDelta(t, 1000.0, kg, 1e-9)
	assert.InDelta(t, 28000.0, oz, 1e-9)
	assert.Equal(t, 1, repo.byIDsCalls, "all expected units must arrive with the first lookup")
}

func TestUnitConverter_Add_PreloadedUnitsNeedNoLookup(t *testing.T) {
	mass := massFixtures()
	repo := newFakeUnitRepo(append(mass, pieceFixture())...)
	converter := services.NewUnitService(repo).NewConverter()
	converter.Add(nil, mass[0], mass[1], pieceFixture()) // g, kg, pc

	res, err := converter.Convert(2, nil, &idKg, []domain.FoodUnitConversion{{UnitID: idPc, TargetAmount: 200, TargetUnitID: idG}})

	require.NoError(t, err)
	assert.InDelta(t, 0.4, res, 1e-9)
	assert.Zero(t, repo.byIDsCalls, "units handed over with Add must not be fetched")
	assert.Zero(t, repo.bySlugCalls, "a handed-over piece unit must be recognised")
}

func TestUnitService_Canonicalize_RewritesAliasesInPlace(t *testing.T) {
	idAlias := uuid.New()
	repo := newFakeUnitRepo(append(massFixtures(), &domain.Unit{ID: idAlias, Slug: "gramme", CanonicalUnitID: &idG})...)
	svc := services.NewUnitService(repo)
	alias, canonical := idAlias, idKg

	require.NoError(t, svc.Canonicalize(&alias, nil, &canonical))

	assert.Equal(t, idG, alias, "an alias must be rewritten to its canonical unit")
	assert.Equal(t, idKg, canonical, "a canonical unit must be left as is")
}

func TestUnitService_Merge_AliasIDs_MergeTheirCanonicalUnits(t *testing.T) {
	idGramme, idKilo := uuid.New(), uuid.New()
	repo := newFakeUnitRepo(append(massFixtures(),
		&domain.Unit{ID: idGramme, Slug: "gramme", CanonicalUnitID: &idG},
		&domain.Unit{ID: idKilo, Slug: "kilo", CanonicalUnitID: &idKg})...)
	svc := services.NewUnitService(repo)

	require.NoError(t, svc.Merge(idGramme, idKilo))
	assert.Equal(t, [][2]uuid.UUID{{idG, idKg}}, repo.merged, "alias IDs must merge the units they stand for")

	require.NoError(t, svc.Merge(idG, idGramme), "an alias of the kept unit is already merged")
	assert.Len(t, repo.merged, 1, "nothing is left to merge")
}
