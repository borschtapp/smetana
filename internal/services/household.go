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
	oldHouseholdID := joiningUser.HouseholdID
	joiningUser.HouseholdID = token.User.HouseholdID
	if err := s.userRepo.Update(joiningUser); err != nil {
		return err
	}
	if _, err = s.userRepo.DeleteToken(code); err != nil {
		return err
	}
	return s.deleteIfEmpty(oldHouseholdID)
}

// RemoveMember removes targetUserID from householdID.
// Only the household owner or the target user themselves may do this.
func (s *HouseholdService) RemoveMember(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID, targetUserID uuid.UUID) error {
	if householdID != requesterHouseholdID {
		return sentinels.ErrForbidden
	}

	household, err := s.repo.ByID(householdID)
	if err != nil {
		return err
	}

	if requesterID != targetUserID && household.OwnerID != requesterID {
		return sentinels.ErrForbidden
	}

	target, err := s.userRepo.ByID(targetUserID)
	if err != nil {
		return err
	}
	if target.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	// Transfer ownership if the owner is being removed.
	if household.OwnerID == targetUserID {
		next, err := s.repo.FirstOtherMember(householdID, targetUserID)
		if err != nil {
			return err
		}
		if next != nil {
			household.OwnerID = next.ID
			if err := s.repo.Update(household); err != nil {
				return err
			}
		}
	}

	// Move the removed user to a new solo household.
	newHousehold := &domain.Household{Name: target.Name + "'s Household", OwnerID: target.ID}
	if err := s.repo.Create(newHousehold); err != nil {
		return err
	}
	target.HouseholdID = newHousehold.ID
	if err := s.userRepo.Update(target); err != nil {
		return err
	}

	return s.deleteIfEmpty(householdID)
}

func (s *HouseholdService) deleteIfEmpty(householdID uuid.UUID) error {
	_, total, err := s.repo.Members(householdID, 0, 1)
	if err != nil {
		return err
	}
	if total == 0 {
		return s.repo.Delete(householdID)
	}
	return nil
}
