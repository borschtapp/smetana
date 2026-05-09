package repositories

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type householdRepository struct {
	db *gorm.DB
}

func NewHouseholdRepository(db *gorm.DB) domain.HouseholdRepository {
	return &householdRepository{db: db}
}

func (r *householdRepository) ByID(id uuid.UUID) (*domain.Household, error) {
	var household domain.Household
	if err := r.db.First(&household, id).Error; err != nil {
		return nil, fmt.Errorf("household by id %s: %w", id, mapErr(err))
	}
	return &household, nil
}

func (r *householdRepository) ByIDWithPreload(id uuid.UUID, opts types.PreloadOptions) (*domain.Household, error) {
	q := r.db.Model(&domain.Household{})

	if opts.Has("members") {
		q = q.Preload("Members")
	}

	// "invites" is populated by the service layer

	var household domain.Household
	if err := q.First(&household, id).Error; err != nil {
		return nil, fmt.Errorf("household by id %s with preload: %w", id, mapErr(err))
	}
	return &household, nil
}

func (r *householdRepository) Create(household *domain.Household) error {
	if err := r.db.Model(household).Omit(clause.Associations).Create(household).Error; err != nil {
		return fmt.Errorf("create household: %w", mapErr(err))
	}
	return nil
}

func (r *householdRepository) Update(household *domain.Household) error {
	if err := r.db.Model(household).Updates(household).Error; err != nil {
		return fmt.Errorf("update household %s: %w", household.ID, mapErr(err))
	}
	return nil
}

func (r *householdRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.Household{}, id).Error; err != nil {
		return fmt.Errorf("delete household %s: %w", id, mapErr(err))
	}
	return nil
}

func (r *householdRepository) FirstOtherMember(householdID uuid.UUID, excludeUserID uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.Where("household_id = ? AND id != ?", householdID, excludeUserID).
		Order("created ASC").
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("first other member of household %s excluding %s: %w", householdID, excludeUserID, mapErr(err))
	}
	return &user, nil
}

func (r *householdRepository) MoveUserToNewHousehold(user *domain.User, currency string) (*domain.Household, error) {
	var newHousehold *domain.Household
	err := r.db.Transaction(func(tx *gorm.DB) error {
		newHousehold = &domain.Household{Name: user.Name + "'s Household", OwnerID: user.ID, Currency: currency}
		if err := tx.Omit(clause.Associations).Create(newHousehold).Error; err != nil {
			return err
		}
		return tx.Model(user).Update("household_id", newHousehold.ID).Error
	})
	if err != nil {
		return nil, fmt.Errorf("move user to new household transaction: %w", mapErr(err))
	}

	user.HouseholdID = newHousehold.ID
	return newHousehold, nil
}

func (r *householdRepository) Members(householdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	query := r.db.Where("household_id = ?", householdID)

	var total int64
	if err := query.Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("members count for household %s: %w", householdID, mapErr(err))
	}

	var members []domain.User
	if err := query.Offset(offset).Limit(limit).Find(&members).Error; err != nil {
		return nil, 0, fmt.Errorf("members find for household %s: %w", householdID, mapErr(err))
	}
	return members, total, nil
}
