package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type householdService struct {
	repo         domain.HouseholdRepository
	userRepo     domain.UserRepository
	emailService domain.EmailService
}

func NewHouseholdService(repo domain.HouseholdRepository, userRepo domain.UserRepository, emailService domain.EmailService) domain.HouseholdService {
	return &householdService{repo: repo, userRepo: userRepo, emailService: emailService}
}

func (s *householdService) ByID(id uuid.UUID, requesterHouseholdID uuid.UUID, opts types.PreloadOptions) (*domain.Household, error) {
	if id != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	household, err := s.repo.ByIDWithPreload(id, opts)
	if err != nil {
		return nil, err
	}

	if opts.Has("invites") {
		household.Invites, err = s.userRepo.FindTokensByHousehold(id, domain.TokenTypeHouseholdInvite)
		if err != nil {
			return nil, err
		}
	}
	return household, nil
}

func (s *householdService) Update(id uuid.UUID, requesterID uuid.UUID, name string) (*domain.Household, error) {
	household, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}

	if household.OwnerID != requesterID {
		return nil, sentinels.ErrForbidden
	}

	household.Name = name
	if err := s.repo.Update(household); err != nil {
		return nil, err
	}

	return household, nil
}

func (s *householdService) Members(householdID uuid.UUID, requesterHouseholdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	if householdID != requesterHouseholdID {
		return nil, 0, sentinels.ErrForbidden
	}
	return s.repo.Members(householdID, offset, limit)
}

func (s *householdService) CreateInvite(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID, email string) (*domain.UserToken, error) {
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

	if email != "" && s.emailService != nil {
		if err := s.emailService.SendHouseholdInvite(email, code); err != nil {
			return token, fmt.Errorf("invite created but failed to send email: %w", err)
		}
	}
	return token, nil
}

func (s *householdService) ListInvites(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID) ([]domain.UserToken, error) {
	if householdID != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}
	return s.userRepo.FindTokensByUser(requesterID, domain.TokenTypeHouseholdInvite)
}

func (s *householdService) JoinByInvite(joiningUserID uuid.UUID, code string) (*domain.User, error) {
	if len(code) != 8 {
		return nil, sentinels.BadRequest("invalid invite code format")
	}
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return nil, err
	}
	if time.Now().After(token.Expires) {
		return nil, sentinels.ErrNotFound
	}

	joiningUser, err := s.userRepo.ByID(joiningUserID)
	if err != nil {
		return nil, err
	}

	oldHouseholdID := joiningUser.HouseholdID
	joiningUser.HouseholdID = token.User.HouseholdID
	if err := s.userRepo.Update(joiningUser); err != nil {
		return nil, err
	}
	if _, err = s.userRepo.DeleteToken(code); err != nil {
		return nil, err
	}
	return joiningUser, s.deleteIfEmpty(oldHouseholdID)
}

func (s *householdService) InviteInfo(code string) (*domain.InviteInfo, error) {
	if len(code) != 8 {
		return nil, sentinels.BadRequest("invalid invite code format")
	}
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return nil, err
	}
	if time.Now().After(token.Expires) {
		return nil, sentinels.ErrNotFound
	}

	household, err := s.repo.ByID(token.User.HouseholdID)
	if err != nil {
		return nil, err
	}

	return &domain.InviteInfo{
		HouseholdName: household.Name,
		InviterName:   token.User.Name,
	}, nil
}

func (s *householdService) RevokeInvite(code string) error {
	if len(code) != 8 {
		return sentinels.BadRequest("invalid invite code format")
	}

	_, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return err
	}
	_, err = s.userRepo.DeleteToken(code)
	return err
}

// RemoveMember removes targetUserID from householdID.
// Only the household owner or the target user themselves may do this.
// Returns the updated target user with their new personal HouseholdID.
func (s *householdService) RemoveMember(householdID uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID, targetUserID uuid.UUID) (*domain.User, error) {
	if householdID != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}

	household, err := s.repo.ByID(householdID)
	if err != nil {
		return nil, err
	}

	if requesterID != targetUserID && household.OwnerID != requesterID {
		return nil, sentinels.ErrForbidden
	}

	target, err := s.userRepo.ByID(targetUserID)
	if err != nil {
		return nil, err
	}
	if target.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}

	// Transfer ownership if the owner is being removed.
	if household.OwnerID == targetUserID {
		next, err := s.repo.FirstOtherMember(householdID, targetUserID)
		if err != nil {
			return nil, err
		}
		if next != nil {
			household.OwnerID = next.ID
			if err := s.repo.Update(household); err != nil {
				return nil, err
			}
		}
	}

	// Move the removed user to a new solo household.
	newHousehold := &domain.Household{Name: target.Name + "'s Household", OwnerID: target.ID}
	if err := s.repo.Create(newHousehold); err != nil {
		return nil, err
	}
	target.HouseholdID = newHousehold.ID
	if err := s.userRepo.Update(target); err != nil {
		return nil, err
	}

	return target, s.deleteIfEmpty(householdID)
}

func (s *householdService) deleteIfEmpty(householdID uuid.UUID) error {
	_, total, err := s.repo.Members(householdID, 0, 1)
	if err != nil {
		return err
	}
	if total == 0 {
		return s.repo.Delete(householdID)
	}
	return nil
}
