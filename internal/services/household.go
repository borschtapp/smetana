package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type HouseholdService struct {
	repo     domain.HouseholdRepository
	userRepo domain.UserRepository
}

func NewHouseholdService(repo domain.HouseholdRepository, userRepo domain.UserRepository) domain.HouseholdService {
	return &HouseholdService{repo: repo, userRepo: userRepo}
}

func (s *HouseholdService) ByID(id uuid.UUID, requesterHouseholdID uuid.UUID) (*domain.Household, error) {
	if id != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	return s.repo.ByID(id)
}

func (s *HouseholdService) Update(household *domain.Household, requesterHouseholdID uuid.UUID) error {
	if household.ID != requesterHouseholdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Update(household)
}

func (s *HouseholdService) Members(householdID uuid.UUID, requesterHouseholdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	if householdID != requesterHouseholdID {
		return nil, 0, sentinels.ErrForbidden
	}
	return s.repo.Members(householdID, offset, limit)
}

// AddMember looks up the user by email and assigns them to the household.
func (s *HouseholdService) AddMember(householdID uuid.UUID, requesterHouseholdID uuid.UUID, targetEmail string) (*domain.User, error) {
	if householdID != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	target, err := s.userRepo.ByEmail(targetEmail)
	if err != nil {
		return nil, err
	}
	if target.HouseholdID == householdID {
		return target, nil
	}
	target.HouseholdID = householdID
	if err := s.userRepo.Update(target); err != nil {
		return nil, err
	}
	return target, nil
}

// RemoveMember verifies the user belongs to the household, then moves them to a new solo household.
func (s *HouseholdService) RemoveMember(householdID uuid.UUID, requesterHouseholdID uuid.UUID, targetUserID uuid.UUID) error {
	if householdID != requesterHouseholdID {
		return sentinels.ErrForbidden
	}
	target, err := s.userRepo.ByID(targetUserID)
	if err != nil {
		return err
	}
	if target.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	newHousehold := &domain.Household{Name: target.Name + "'s Household"}
	if err := s.repo.Create(newHousehold); err != nil {
		return err
	}
	target.HouseholdID = newHousehold.ID
	return s.userRepo.Update(target)
}
