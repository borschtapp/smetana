package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type userService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) domain.UserService {
	return &userService{repo: repo}
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
	return s.repo.Delete(id)
}
