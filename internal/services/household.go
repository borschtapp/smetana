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
		return nil, fmt.Errorf("by id with preload: %w", err)
	}

	if opts.Has("invites") {
		household.Invites, err = s.userRepo.FindTokensByHousehold(id, domain.TokenTypeHouseholdInvite)
		if err != nil {
			return nil, fmt.Errorf("by id (fetch invites): %w", err)
		}
	}
	return household, nil
}

func (s *householdService) Update(id uuid.UUID, requesterID uuid.UUID, requesterHouseholdID uuid.UUID, name string, currency *string) (*domain.Household, error) {
	if id != requesterHouseholdID {
		return nil, sentinels.ErrForbidden
	}

	household, err := s.repo.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("update (fetch existing): %w", err)
	}

	if household.OwnerID != requesterID {
		return nil, sentinels.ErrForbidden
	}

	household.Name = name
	if currency != nil {
		household.Currency = *currency
	}
	if err := s.repo.Update(household); err != nil {
		return nil, fmt.Errorf("update (persist): %w", err)
	}

	return household, nil
}

func (s *householdService) Members(householdID uuid.UUID, requesterHouseholdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	if householdID != requesterHouseholdID {
		return nil, 0, sentinels.ErrForbidden
	}
	members, total, err := s.repo.Members(householdID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("members: %w", err)
	}
	return members, total, nil
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
		return nil, fmt.Errorf("create invite (persist token): %w", err)
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
	invites, err := s.userRepo.FindTokensByHousehold(householdID, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	return invites, nil
}

func (s *householdService) JoinByInvite(joiningUserID uuid.UUID, code string) (*domain.User, error) {
	if len(code) != 8 {
		return nil, sentinels.BadRequest("invalid invite code format")
	}
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return nil, fmt.Errorf("join by invite (fetch token): %w", err)
	}
	if time.Now().After(token.Expires) {
		return nil, sentinels.Unauthorized("invite code has expired")
	}

	joiningUser, err := s.userRepo.ByID(joiningUserID)
	if err != nil {
		return nil, fmt.Errorf("join by invite (fetch user): %w", err)
	}

	oldHouseholdID := joiningUser.HouseholdID
	joiningUser.HouseholdID = token.User.HouseholdID
	if err := s.userRepo.Update(joiningUser); err != nil {
		return nil, fmt.Errorf("join by invite (update user household): %w", err)
	}
	if _, err = s.userRepo.DeleteToken(code); err != nil {
		return nil, fmt.Errorf("join by invite (consume token): %w", err)
	}
	return joiningUser, s.deleteIfEmpty(oldHouseholdID)
}

func (s *householdService) InviteInfo(code string) (*domain.InviteInfo, error) {
	if len(code) != 8 {
		return nil, sentinels.BadRequest("invalid invite code format")
	}
	token, err := s.userRepo.FindToken(code, domain.TokenTypeHouseholdInvite)
	if err != nil {
		return nil, fmt.Errorf("invite info (fetch token): %w", err)
	}
	if time.Now().After(token.Expires) {
		return nil, sentinels.Unauthorized("invite code has expired")
	}

	household, err := s.repo.ByID(token.User.HouseholdID)
	if err != nil {
		return nil, fmt.Errorf("invite info (fetch household): %w", err)
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
		return fmt.Errorf("revoke invite (fetch token): %w", err)
	}
	if _, err = s.userRepo.DeleteToken(code); err != nil {
		return fmt.Errorf("revoke invite (consume token): %w", err)
	}
	return nil
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
		return nil, fmt.Errorf("remove member (fetch household): %w", err)
	}

	if requesterID != targetUserID && household.OwnerID != requesterID {
		return nil, sentinels.ErrForbidden
	}

	target, err := s.userRepo.ByID(targetUserID)
	if err != nil {
		return nil, fmt.Errorf("remove member (fetch target user): %w", err)
	}
	if target.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}

	// Transfer ownership if the owner is being removed.
	if household.OwnerID == targetUserID {
		next, err := s.repo.FirstOtherMember(householdID, targetUserID)
		if err != nil {
			return nil, fmt.Errorf("remove member (find next owner): %w", err)
		}
		if next != nil {
			household.OwnerID = next.ID
			if err := s.repo.Update(household); err != nil {
				return nil, fmt.Errorf("remove member (persist owner update): %w", err)
			}
		}
	}

	if _, err := s.repo.MoveUserToNewHousehold(target, household.Currency); err != nil {
		return nil, fmt.Errorf("remove member (move to new household): %w", err)
	}

	return target, s.deleteIfEmpty(householdID)
}

func (s *householdService) deleteIfEmpty(householdID uuid.UUID) error {
	_, total, err := s.repo.Members(householdID, 0, 1)
	if err != nil {
		return fmt.Errorf("delete if empty (check members): %w", err)
	}
	if total == 0 {
		if err := s.repo.Delete(householdID); err != nil {
			return fmt.Errorf("delete if empty (persist delete): %w", err)
		}
	}
	return nil
}
