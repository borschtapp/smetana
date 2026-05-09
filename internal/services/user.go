package services

import (
	"fmt"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type userService struct {
	repo          domain.UserRepository
	householdRepo domain.HouseholdRepository
}

func NewUserService(repo domain.UserRepository, householdRepo domain.HouseholdRepository) domain.UserService {
	return &userService{repo: repo, householdRepo: householdRepo}
}

func (s *userService) ByID(id uuid.UUID, requesterID uuid.UUID) (*domain.User, error) {
	if id != requesterID {
		return nil, sentinels.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("by id: %w", err)
	}
	return user, nil
}

// Update fetches the user, applies the non-nil patches, and persists the result.
func (s *userService) Update(id uuid.UUID, requesterID uuid.UUID, name, email, currentPassword, newPassword *string) (*domain.User, error) {
	if id != requesterID {
		return nil, sentinels.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("update (fetch user): %w", err)
	}
	if email != nil || newPassword != nil {
		if currentPassword == nil || !utils.ValidatePassword(user.Password, *currentPassword) {
			return nil, sentinels.ErrUnauthorized
		}
	}
	if email != nil {
		user.Email = *email
	}
	if newPassword != nil {
		hash, err := utils.HashPassword(*newPassword)
		if err != nil {
			return nil, fmt.Errorf("update (hash password): %w", err)
		}
		user.Password = hash
	}
	if name != nil {
		user.Name = *name
	}
	if err := s.repo.Update(user); err != nil {
		return nil, fmt.Errorf("update (persist): %w", err)
	}
	return user, nil
}

func (s *userService) Delete(id uuid.UUID, requesterID uuid.UUID) error {
	if id != requesterID {
		return sentinels.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return fmt.Errorf("delete (fetch user): %w", err)
	}
	householdID := user.HouseholdID
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("delete (persist): %w", err)
	}
	_, total, err := s.householdRepo.Members(householdID, 0, 1)
	if err != nil {
		return fmt.Errorf("delete (check remaining members): %w", err)
	}
	if total == 0 {
		if err := s.householdRepo.Delete(householdID); err != nil {
			return fmt.Errorf("delete (cleanup empty household): %w", err)
		}
	}
	return nil
}
