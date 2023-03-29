package user

import (
	"borscht.app/smetana/model"
	"borscht.app/smetana/utils"
)

type Service interface {
	FindUser(ID uint) (*model.User, error)
	FindUserByEmail(email string) (*model.User, error)
	CreateUser(user *model.User) (*model.User, error)
	UpdateUser(user *model.User) (*model.User, error)
	DeleteUser(ID uint) error
}

type service struct {
	repository Repository
}

func NewService(r Repository) Service {
	return &service{
		repository: r,
	}
}

func (s *service) FindUserByEmail(email string) (*model.User, error) {
	return s.repository.FindByEmail(email)
}

func (s *service) FindUser(ID uint) (*model.User, error) {
	return s.repository.Find(ID)
}

func (s *service) CreateUser(user *model.User) (*model.User, error) {
	hash, err := utils.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}

	user.Password = hash
	return s.repository.Create(user)
}

func (s *service) UpdateUser(user *model.User) (*model.User, error) {
	return s.repository.Update(user)
}

func (s *service) DeleteUser(ID uint) error {
	return s.repository.Delete(ID)
}
