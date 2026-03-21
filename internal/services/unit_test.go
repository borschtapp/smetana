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
	units map[uuid.UUID]*domain.Unit
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

var (
	idG   = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	idKg  = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	idMg  = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	idOz  = uuid.MustParse("00000000-0000-0000-0000-000000000004")
	idMl  = uuid.MustParse("00000000-0000-0000-0000-000000000005")
	idL   = uuid.MustParse("00000000-0000-0000-0000-000000000006")
	idTsp = uuid.MustParse("00000000-0000-0000-0000-000000000007")
)

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

func TestUnitService_Convert_SameUnit_ReturnsAmountUnchanged(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.Convert(42, idKg, idKg)

	require.NoError(t, err)
	assert.InDelta(t, 42.0, result, 1e-9, "same-unit conversion must be identity")
}

func TestUnitService_Convert_MetricToMetric_ScalesCorrectly(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.Convert(1, idKg, idG)

	require.NoError(t, err)
	assert.InDelta(t, 1000.0, result, 1e-6, "1 kg must equal 1000 g")
}

func TestUnitService_Convert_MetricToImperial_ScalesCorrectly(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	result, err := svc.Convert(100, idG, idOz)

	require.NoError(t, err)
	assert.InDelta(t, 3.572, result, 0.001, "100 g must convert to ~3.572 oz")
}

func TestUnitService_Convert_IncompatibleDimensions_ReturnsNotFound(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	_, err := svc.Convert(1, idG, idMl)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "mass and volume are incompatible dimensions")
}

func TestUnitService_Convert_UnknownFromUnit_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	_, err := svc.Convert(1, uuid.New(), idG)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "unknown source unit must return not found")
}

func TestUnitService_Convert_UnknownToUnit_ReturnsNotFound(t *testing.T) {
	svc := services.NewUnitService(newFakeUnitRepo(massFixtures()...))

	_, err := svc.Convert(1, idG, uuid.New())

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "unknown target unit must return not found")
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

	units, total, err := svc.Search("", ptr(true), 0, 20)

	require.NoError(t, err)
	assert.Equal(t, int64(2), total, "fixture has exactly 2 imperial units (oz, tsp)")
	for _, u := range units {
		assert.True(t, u.Imperial, "all returned units must be imperial")
	}
}

func TestUnitService_Search_MetricFilter_ReturnsOnlyMetricUnits(t *testing.T) {
	all := append(massFixtures(), volumeFixtures()...)
	svc := services.NewUnitService(newFakeUnitRepo(all...))

	units, _, err := svc.Search("", ptr(false), 0, 20)

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
