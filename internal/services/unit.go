package services

import (
	"math"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type unitService struct {
	repo domain.UnitRepository
}

func NewUnitService(repo domain.UnitRepository) domain.UnitService {
	return &unitService{repo: repo}
}

func (s *unitService) FindOrCreate(unit *domain.Unit) error {
	return s.repo.FindOrCreate(unit)
}

func (s *unitService) Search(query string, imperial *bool, offset, limit int) ([]domain.Unit, int64, error) {
	return s.repo.Search(query, imperial, offset, limit)
}

func (s *unitService) Convert(amount float64, fromUnitID, toUnitID uuid.UUID) (float64, error) {
	if fromUnitID == toUnitID {
		return amount, nil
	}

	from, err := s.repo.ByID(fromUnitID)
	if err != nil {
		return 0, err
	}
	to, err := s.repo.ByID(toUnitID)
	if err != nil {
		return 0, err
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
		return nil, err
	}

	candidates, err := s.repo.ByBase(from.BaseID(), imperial)
	if err != nil {
		return nil, err
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
