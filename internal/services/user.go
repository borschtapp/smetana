package services

import (
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
	return s.repo.ByID(id)
}

// Update fetches the user, applies the non-nil patches, and persists the result.
func (s *userService) Update(id uuid.UUID, requesterID uuid.UUID, name, email, currentPassword, newPassword *string) (*domain.User, error) {
	if id != requesterID {
		return nil, sentinels.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		user.Password = hash
	}
	if name != nil {
		user.Name = *name
	}
	return user, s.repo.Update(user)
}

func (s *userService) Delete(id uuid.UUID, requesterID uuid.UUID) error {
	if id != requesterID {
		return sentinels.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	householdID := user.HouseholdID
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	_, total, err := s.householdRepo.Members(householdID, 0, 1)
	if err != nil {
		return err
	}
	if total == 0 {
		return s.householdRepo.Delete(householdID)
	}
	return nil
}
