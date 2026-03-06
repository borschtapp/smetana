package services

import (
	"borscht.app/smetana/domain"
)

type UnitService struct {
	repo domain.UnitRepository
}

func NewUnitService(repo domain.UnitRepository) domain.UnitService {
	return &UnitService{repo: repo}
}

func (s *UnitService) FindOrCreate(unit *domain.Unit) error {
	return s.repo.FindOrCreate(unit)
}
