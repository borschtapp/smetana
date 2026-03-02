package services

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type HouseholdService struct {
	repo domain.HouseholdRepository
}

func NewHouseholdService(repo domain.HouseholdRepository) *HouseholdService {
	return &HouseholdService{repo: repo}
}

func (s *HouseholdService) ById(id uuid.UUID) (*domain.Household, error) {
	return s.repo.ById(id)
}

func (s *HouseholdService) Create(household *domain.Household) error {
	return s.repo.Create(household)
}

func (s *HouseholdService) Update(household *domain.Household) error {
	return s.repo.Update(household)
}

func (s *HouseholdService) Members(householdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	return s.repo.Members(householdID, offset, limit)
}
