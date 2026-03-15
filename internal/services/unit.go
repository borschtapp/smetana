package services

import (
	"borscht.app/smetana/domain"
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
