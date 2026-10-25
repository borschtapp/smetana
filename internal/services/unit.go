package services

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"github.com/google/uuid"
)

type unitService struct {
	repo domain.UnitRepository
}

func NewUnitService(repo domain.UnitRepository) domain.UnitService {
	return &unitService{repo: repo}
}

func (s *unitService) ByID(id uuid.UUID) (*domain.Unit, error) {
	unit, err := s.repo.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("by id: %w", err)
	}
	return unit, nil
}

func (s *unitService) Update(unit *domain.Unit) error {
	if err := s.repo.Update(unit); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

func (s *unitService) FindOrCreate(unit *domain.Unit) error {
	if err := s.repo.FindOrCreate(unit); err != nil {
		return fmt.Errorf("find or create: %w", err)
	}
	return nil
}

func (s *unitService) Merge(keepID, mergeID uuid.UUID) error {
	if keepID == mergeID {
		return sentinels.BadRequest("cannot merge a unit into itself")
	}
	// A merged-away ID stands for the unit it was merged into; if both resolve to one unit, nothing is left to do.
	if err := s.Canonicalize(&keepID, &mergeID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	if keepID == mergeID {
		return nil
	}
	if err := s.repo.Merge(keepID, mergeID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	return nil
}

func (s *unitService) Search(query string, imperial *bool, offset, limit int) ([]domain.Unit, int64, error) {
	units, total, err := s.repo.Search(query, imperial, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	return units, total, nil
}

// scaleUnits converts amount between two loaded units along their shared base-unit chain.
func scaleUnits(amount float64, from, to *domain.Unit) (float64, error) {
	if from.ID == to.ID {
		return amount, nil
	}
	if from.BaseID() != to.BaseID() {
		return 0, sentinels.Unprocessable("units do not share a common base unit")
	}

	fromFactor, err := from.ToBaseFactor()
	if err != nil {
		return 0, sentinels.Unprocessable("from unit is missing a conversion factor")
	}
	toFactor, err := to.ToBaseFactor()
	if err != nil {
		return 0, sentinels.Unprocessable("to unit is missing a conversion factor")
	}

	return amount * fromFactor / toFactor, nil
}

func (s *unitService) BestUnit(amount float64, fromUnitID uuid.UUID, imperial bool) (*domain.Unit, error) {
	from, err := s.repo.ByID(fromUnitID)
	if err != nil {
		return nil, fmt.Errorf("best unit (fetch from-unit): %w", err)
	}

	candidates, err := s.repo.ByBase(from.BaseID(), imperial)
	if err != nil {
		return nil, fmt.Errorf("best unit (fetch candidates): %w", err)
	}
	if len(candidates) == 0 {
		return nil, sentinels.Unprocessable("no units found with the same base unit")
	}

	fromFactor, err := from.ToBaseFactor()
	if err != nil {
		return nil, sentinels.Unprocessable("from unit is missing a conversion factor")
	}

	amountInBase := amount * fromFactor
	best := &candidates[0]
	f0, err := candidates[0].ToBaseFactor()
	if err != nil {
		return nil, sentinels.Unprocessable("f0 unit is missing a conversion factor")
	}

	bestScore := math.Abs(math.Log10(amountInBase / f0))
	for i := 1; i < len(candidates); i++ {
		f, err := candidates[i].ToBaseFactor()
		if err != nil {
			continue
		}
		score := math.Abs(math.Log10(amountInBase / f))
		if score < bestScore {
			bestScore = score
			best = &candidates[i]
		}
	}
	return best, nil
}

func (s *unitService) Canonicalize(unitIDs ...*uuid.UUID) error {
	if err := canonicalize(s.repo.CanonicalIDs, unitIDs); err != nil {
		return fmt.Errorf("canonicalize units: %w", err)
	}
	return nil
}

func (s *unitService) NewConverter() domain.UnitConverter {
	return &unitConverter{
		repo:    s.repo,
		units:   make(map[uuid.UUID]*domain.Unit),
		pending: make(map[uuid.UUID]struct{}),
	}
}

// unitConverter caches units for one request, so a batch of conversions needs one query, not one per conversion.
type unitConverter struct {
	repo     domain.UnitRepository
	units    map[uuid.UUID]*domain.Unit // nil: known to be missing
	pending  map[uuid.UUID]struct{}     // queued for the next lookup
	pieceID  uuid.UUID                  // uuid.Nil until resolved
	pieceErr error                      // cached piece unit lookup failure
}

func (c *unitConverter) Add(units ...*domain.Unit) {
	for _, u := range units {
		if u == nil {
			continue
		}
		c.units[u.ID] = u
		delete(c.pending, u.ID)
		if u.IsPiece() {
			c.pieceID = u.ID
		}
	}
}

func (c *unitConverter) Expect(unitIDs ...uuid.UUID) {
	for _, id := range unitIDs {
		if _, seen := c.units[id]; !seen {
			c.pending[id] = struct{}{}
		}
	}
}

func (c *unitConverter) Convert(amount float64, fromUnit, toUnit *uuid.UUID, conversions []domain.FoodUnitConversion) (float64, error) {
	if fromUnit == nil && toUnit == nil {
		return amount, nil
	}
	fromUnitID, err := c.unitOrPiece(fromUnit)
	if err != nil {
		return 0, err
	}
	toUnitID, err := c.unitOrPiece(toUnit)
	if err != nil {
		return 0, err
	}

	if fromUnitID == toUnitID {
		return amount, nil
	}

	// Fetch the units this conversion can touch, together with anything queued.
	c.Expect(fromUnitID, toUnitID)
	for _, conv := range conversions {
		c.Expect(conv.UnitID, conv.TargetUnitID)
	}
	if err := c.load(); err != nil {
		return 0, err
	}
	from, to := c.units[fromUnitID], c.units[toUnitID]
	if from == nil || to == nil {
		return 0, fmt.Errorf("convert (fetch units): %w", sentinels.ErrNotFound)
	}

	if finalAmount, err := scaleUnits(amount, from, to); err == nil {
		return finalAmount, nil
	}

	for _, conv := range conversions {
		src, dst := c.units[conv.UnitID], c.units[conv.TargetUnitID]
		if src == nil || dst == nil {
			continue
		}

		if bridgeAmount, err := scaleUnits(amount, from, src); err == nil {
			if finalAmount, err := scaleUnits(bridgeAmount*conv.TargetAmount, dst, to); err == nil {
				return finalAmount, nil
			}
		}

		if bridgeAmount, err := scaleUnits(amount, from, dst); err == nil {
			if finalAmount, err := scaleUnits(bridgeAmount/conv.TargetAmount, src, to); err == nil {
				return finalAmount, nil
			}
		}
	}

	return 0, sentinels.Unprocessable("units do not share a common base unit and no matching conversion bridge was found")
}

// load fetches all pending units in one query.
func (c *unitConverter) load() error {
	if len(c.pending) == 0 {
		return nil
	}
	found, err := c.repo.ByIDs(slices.Collect(maps.Keys(c.pending)))
	if err != nil {
		return fmt.Errorf("convert (fetch units): %w", err)
	}
	for id := range c.pending {
		c.units[id] = found[id]
	}
	clear(c.pending)
	return nil
}

// unitOrPiece returns the unit ID, or the piece unit's ID for a unitless amount.
func (c *unitConverter) unitOrPiece(unitID *uuid.UUID) (uuid.UUID, error) {
	if unitID != nil {
		return *unitID, nil
	}
	if c.pieceErr != nil {
		return uuid.Nil, c.pieceErr
	}
	if c.pieceID == uuid.Nil {
		piece, err := c.repo.BySlug(domain.PieceUnitSlug)
		if err != nil {
			c.pieceErr = fmt.Errorf("resolve piece unit: %w", err)
			return uuid.Nil, c.pieceErr
		}
		c.Add(piece)
		c.pieceID = piece.ID // after a legacy merge, BySlug may resolve to a unit with another slug
	}
	return c.pieceID, nil
}

// conversionUnitIDs lists the source and target units of the given food conversions.
func conversionUnitIDs(conversions map[uuid.UUID][]domain.FoodUnitConversion) []uuid.UUID {
	var ids []uuid.UUID
	for _, list := range conversions {
		for _, conv := range list {
			ids = append(ids, conv.UnitID, conv.TargetUnitID)
		}
	}
	return ids
}
