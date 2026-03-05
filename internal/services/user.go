package services

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) ByID(id uuid.UUID, requesterID uuid.UUID) (*domain.User, error) {
	if id != requesterID {
		return nil, domain.ErrForbidden
	}
	return s.repo.ByID(id)
}

func (s *UserService) ByEmail(email string) (*domain.User, error) {
	return s.repo.ByEmail(email)
}

func (s *UserService) ByEmailWithHousehold(email string) (*domain.User, error) {
	return s.repo.ByEmailWithHousehold(email)
}

func (s *UserService) Update(user *domain.User, requesterID uuid.UUID) error {
	if user.ID != requesterID {
		return domain.ErrForbidden
	}
	return s.repo.Update(user)
}

func (s *UserService) Delete(id uuid.UUID, requesterID uuid.UUID) error {
	if id != requesterID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(id)
}

// Create provisions a personal household then persists the user in a single transaction.
func (s *UserService) Create(user *domain.User) error {
	user.Household = &domain.Household{Name: user.Name + "'s Household"}
	return s.repo.Create(user)
}

// FindRefreshToken retrieves a refresh token with its associated user.
func (s *UserService) FindRefreshToken(tokenStr string) (*domain.UserToken, error) {
	return s.repo.FindToken(tokenStr, "refresh")
}

// CreateRefreshToken persists a new refresh token for a user.
func (s *UserService) CreateRefreshToken(token *domain.UserToken) error {
	return s.repo.CreateToken(token)
}

// DeleteRefreshToken permanently removes a refresh token.
func (s *UserService) DeleteRefreshToken(tokenStr string) error {
	return s.repo.DeleteToken(tokenStr)
}
