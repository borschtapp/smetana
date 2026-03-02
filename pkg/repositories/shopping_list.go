package repositories

import (
	"errors"

	"borscht.app/smetana/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShoppingListRepository struct {
	db *gorm.DB
}

func NewShoppingListRepository(db *gorm.DB) *ShoppingListRepository {
	return &ShoppingListRepository{db: db}
}

func (r *ShoppingListRepository) ById(id uuid.UUID) (*domain.ShoppingList, error) {
	var item domain.ShoppingList
	if err := r.db.Preload("Unit").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *ShoppingListRepository) List(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	query := r.db.Preload("Unit").Where("household_id = ?", householdID)

	var total int64
	if err := query.Model(&domain.ShoppingList{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.ShoppingList
	if err := query.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ShoppingListRepository) Create(item *domain.ShoppingList) error {
	return r.db.Create(item).Error
}

func (r *ShoppingListRepository) Update(item *domain.ShoppingList) error {
	return r.db.Model(item).Updates(item).Error
}

func (r *ShoppingListRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.ShoppingList{}, id).Error
}
