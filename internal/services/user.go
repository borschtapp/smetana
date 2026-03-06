package services

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) domain.UserService {
	return &UserService{repo: repo}
}

func (s *UserService) ByID(id uuid.UUID, requesterID uuid.UUID) (*domain.User, error) {
	if id != requesterID {
		return nil, domain.ErrForbidden
	}
	return s.repo.ByID(id)
}

// Update fetches the user, applies the non-nil patches, and persists the result.
func (s *UserService) Update(id uuid.UUID, requesterID uuid.UUID, name, email *string) (*domain.User, error) {
	if id != requesterID {
		return nil, domain.ErrForbidden
	}
	user, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		user.Name = *name
	}
	if email != nil {
		user.Email = *email
	}
	return user, s.repo.Update(user)
}

func (s *UserService) Delete(id uuid.UUID, requesterID uuid.UUID) error {
	if id != requesterID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(id)
}
