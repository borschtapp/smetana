package repositories

import (
	"errors"

	"borscht.app/smetana/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type HouseholdRepository struct {
	db *gorm.DB
}

func NewHouseholdRepository(db *gorm.DB) *HouseholdRepository {
	return &HouseholdRepository{db: db}
}

func (r *HouseholdRepository) ById(id uuid.UUID) (*domain.Household, error) {
	var household domain.Household
	if err := r.db.First(&household, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &household, nil
}

func (r *HouseholdRepository) Create(household *domain.Household) error {
	return r.db.Model(household).Create(household).Error
}

func (r *HouseholdRepository) Update(household *domain.Household) error {
	return r.db.Model(household).Updates(household).Error
}

func (r *HouseholdRepository) Members(householdID uuid.UUID, offset, limit int) ([]domain.User, int64, error) {
	query := r.db.Where("household_id = ?", householdID)

	var total int64
	if err := query.Model(&domain.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var members []domain.User
	if err := query.Offset(offset).Limit(limit).Find(&members).Error; err != nil {
		return nil, 0, err
	}
	return members, total, nil
}
