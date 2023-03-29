package user

import (
	"errors"

	"gorm.io/gorm"

	"borscht.app/smetana/model"
)

type Repository interface {
	Find(ID uint) (*model.User, error)
	FindByEmail(email string) (*model.User, error)
	Create(user *model.User) (*model.User, error)
	Update(user *model.User) (*model.User, error)
	Delete(ID uint) error
}

type repository struct {
	db *gorm.DB
}

func NewRepo(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Find(ID uint) (*model.User, error) {
	var user model.User

	if err := r.db.Take(&user, ID).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *repository) FindByEmail(email string) (*model.User, error) {
	var user model.User

	if err := r.db.Where(&model.User{Email: email}).Find(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}

func (r *repository) Create(user *model.User) (*model.User, error) {
	if err := r.db.Create(&user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *repository) Update(user *model.User) (*model.User, error) {
	if err := r.db.Save(&user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func (r *repository) Delete(ID uint) error {
	if result := r.db.Delete(&model.User{}, ID); result.Error != nil {
		return result.Error
	}

	return nil
}
