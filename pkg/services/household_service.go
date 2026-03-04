package services

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type HouseholdService struct {
	repo     domain.HouseholdRepository
	userRepo domain.UserRepository
}

func NewHouseholdService(repo domain.HouseholdRepository, userRepo domain.UserRepository) *HouseholdService {
	return &HouseholdService{repo: repo, userRepo: userRepo}
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

// AddMember looks up the user by email and assigns them to the household.
func (s *HouseholdService) AddMember(householdID uuid.UUID, targetEmail string) (*domain.User, error) {
	target, err := s.userRepo.ByEmail(targetEmail)
	if err != nil {
		return nil, err
	}
	target.HouseholdID = householdID
	if err := s.userRepo.Update(target); err != nil {
		return nil, err
	}
	return target, nil
}

// RemoveMember verifies the user belongs to the household, then moves them to a new solo household.
func (s *HouseholdService) RemoveMember(householdID uuid.UUID, targetUserID uuid.UUID) error {
	target, err := s.userRepo.ById(targetUserID)
	if err != nil {
		return err
	}
	if target.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	newHousehold := &domain.Household{Name: target.Name + "'s Household"}
	if err := s.repo.Create(newHousehold); err != nil {
		return err
	}
	target.HouseholdID = newHousehold.ID
	return s.userRepo.Update(target)
}
