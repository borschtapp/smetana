package services

import (
	"errors"
	"strings"
	"time"

	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) ById(id uuid.UUID) (*domain.User, error) {
	return s.repo.ById(id)
}

func (s *UserService) ByEmail(email string) (*domain.User, error) {
	return s.repo.ByEmail(email)
}

func (s *UserService) Update(user *domain.User) error {
	return s.repo.Update(user)
}

func (s *UserService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

// Create provisions a personal household then persists the user in a single transaction.
func (s *UserService) Create(user *domain.User) error {
	user.Household = &domain.Household{Name: user.Name + "'s Household"}
	return s.repo.Create(user)
}

// FindOrRegisterOIDCUser finds a user by email (with Household preloaded) or creates one via JIT provisioning.
func (s *UserService) FindOrRegisterOIDCUser(email, name string) (*domain.User, error) {
	user, err := s.repo.ByEmailWithHousehold(email)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, domain.ErrRecordNotFound) {
		return nil, err
	}

	newUser := domain.User{
		ID:      uuid.New(),
		Email:   email,
		Name:    name,
		Created: time.Now(),
	}
	if newUser.Name == "" {
		newUser.Name = strings.Split(email, "@")[0]
	}

	if err := s.Create(&newUser); err != nil {
		return nil, err
	}
	return &newUser, nil
}
