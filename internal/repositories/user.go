package repositories

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) ByID(id uuid.UUID) (*domain.User, error) {
	var user domain.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, fmt.Errorf("user by id %s: %w", id, mapErr(err))
	}
	return &user, nil
}

func (r *userRepository) ByEmail(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where(&domain.User{Email: email}).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user by email %s: %w", email, mapErr(err))
	}
	return &user, nil
}

func (r *userRepository) ByEmailWithHousehold(email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.Where(&domain.User{Email: email}).Preload("Household").First(&user).Error; err != nil {
		return nil, fmt.Errorf("user by email %s with household: %w", email, mapErr(err))
	}
	return &user, nil
}

func (r *userRepository) Create(user *domain.User) error {
	return mapErr(r.db.Transaction(func(tx *gorm.DB) error {
		if user.HouseholdID == uuid.Nil {
			_ = user.BeforeCreate(tx) // to ensure user's ID is not empty
			user.Household = &domain.Household{OwnerID: user.ID, Name: user.Name + "'s Household"}
			if err := tx.Create(user.Household).Error; err != nil {
				return mapErr(err)
			}
			user.HouseholdID = user.Household.ID
		}

		return mapErr(tx.Create(user).Error)
	}))
}

func (r *userRepository) Update(user *domain.User) error {
	if err := r.db.Model(user).Updates(user).Error; err != nil {
		return fmt.Errorf("update user %s: %w", user.ID, mapErr(err))
	}
	return nil
}

func (r *userRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.User{}, id).Error; err != nil {
		return fmt.Errorf("delete user %s: %w", id, mapErr(err))
	}
	return nil
}

func (r *userRepository) FindTokensByUser(userID uuid.UUID, tokenType string) ([]domain.UserToken, error) {
	var tokens []domain.UserToken
	if err := r.db.Where(&domain.UserToken{UserID: userID, Type: tokenType}).Find(&tokens).Error; err != nil {
		return nil, fmt.Errorf("find tokens by user %s type %s: %w", userID, tokenType, mapErr(err))
	}
	return tokens, nil
}

func (r *userRepository) FindTokensByHousehold(householdID uuid.UUID, tokenType string) ([]domain.UserToken, error) {
	var tokens []domain.UserToken
	err := r.db.Joins("JOIN users ON users.id = user_tokens.user_id").
		Where("users.household_id = ? AND user_tokens.type = ?", householdID, tokenType).
		Find(&tokens).Error
	if err != nil {
		return nil, fmt.Errorf("find tokens by household %s type %s: %w", householdID, tokenType, mapErr(err))
	}
	return tokens, nil
}

func (r *userRepository) FindToken(tokenStr string, tokenType string) (*domain.UserToken, error) {
	var userToken domain.UserToken
	if err := r.db.Preload("User").Where(&domain.UserToken{Token: tokenStr, Type: tokenType}).First(&userToken).Error; err != nil {
		return nil, fmt.Errorf("find token type %s: %w", tokenType, mapErr(err))
	}
	return &userToken, nil
}

func (r *userRepository) CreateToken(token *domain.UserToken) error {
	if err := r.db.Create(token).Error; err != nil {
		return fmt.Errorf("create token: %w", mapErr(err))
	}
	return nil
}

func (r *userRepository) DeleteToken(tokenStr string) (bool, error) {
	result := r.db.Unscoped().Where(&domain.UserToken{Token: tokenStr}).Delete(&domain.UserToken{})
	if result.Error != nil {
		return false, fmt.Errorf("delete token: %w", mapErr(result.Error))
	}
	return result.RowsAffected == 1, nil
}
