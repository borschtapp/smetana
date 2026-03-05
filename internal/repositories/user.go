package repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) ByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ByEmail(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where(&domain.User{Email: email}).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ByEmailWithHousehold(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where(&domain.User{Email: email}).Preload("Household").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) Create(user *domain.User) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user.Household).Error; err != nil {
			return err
		}
		user.HouseholdID = user.Household.ID
		return tx.Create(user).Error
	})
}

func (r *UserRepository) Update(user *domain.User) error {
	return r.db.Model(user).Updates(user).Error
}

func (r *UserRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.User{}, id).Error
}

func (r *UserRepository) FindToken(tokenStr string, tokenType string) (*domain.UserToken, error) {
	var userToken domain.UserToken
	err := r.db.Joins("User").Where(&domain.UserToken{Token: tokenStr, Type: tokenType}).First(&userToken).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &userToken, nil
}

func (r *UserRepository) CreateToken(token *domain.UserToken) error {
	return r.db.Create(token).Error
}

func (r *UserRepository) DeleteToken(tokenStr string) error {
	return r.db.Unscoped().Where(&domain.UserToken{Token: tokenStr}).Delete(&domain.UserToken{}).Error
}
