package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
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

func (s *HouseholdService) CreateInvite(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID) (*domain.UserToken, error) {
	if householdID != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	code := utils.GenerateInviteCode()
	if code == "" {
		return nil, fmt.Errorf("failed to generate invite code")
	}
	token := &domain.UserToken{
		UserID:  requesterID,
		Type:    domain.TokenTypeHouseholdInvite,
		Token:   code,
		Expires: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := s.userRepo.CreateToken(token); err != nil {
		return nil, err
	}
	return token, nil
}

func (s *HouseholdService) ListInvites(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID) ([]domain.UserToken, error) {
	if householdID != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	return s.userRepo.FindTokensByUser(requesterID, domain.TokenTypeHouseholdInvite)
}

func (s *HouseholdService) RevokeInvite(householdID uuid.UUID, requesterHouseholdID uuid.UUID, code string) error {
	if householdID != requesterHouseholdID {
		return sentinels.ErrForbidden
	}
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return err
	}
	if token.User.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	_, err = s.userRepo.DeleteToken(code)
	return err
}

func (s *HouseholdService) JoinByInvite(joiningUserID uuid.UUID, code string) error {
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return err
	}
	if time.Now().After(token.Expires) {
		return sentinels.ErrRecordNotFound
	}
	joiningUser, err := s.userRepo.ByID(joiningUserID)
	if err != nil {
		return err
	}
	joiningUser.HouseholdID = token.User.HouseholdID
	if err := s.userRepo.Update(joiningUser); err != nil {
		return err
	}
	_, err = s.userRepo.DeleteToken(code)
	return err
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
