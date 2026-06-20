package services

import (
	"fmt"
	"math"

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

func (s *unitService) Convert(amount float64, fromUnitID, toUnitID uuid.UUID) (float64, error) {
	if fromUnitID == toUnitID {
		return amount, nil
	}

	from, err := s.repo.ByID(fromUnitID)
	if err != nil {
		return 0, fmt.Errorf("convert (fetch from-unit): %w", err)
	}
	to, err := s.repo.ByID(toUnitID)
	if err != nil {
		return 0, fmt.Errorf("convert (fetch to-unit): %w", err)
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
