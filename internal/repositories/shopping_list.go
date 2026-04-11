package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type shoppingListRepository struct {
	db *gorm.DB
}

func NewShoppingListRepository(db *gorm.DB) domain.ShoppingListRepository {
	return &shoppingListRepository{db: db}
}

func (r *shoppingListRepository) ByID(id uuid.UUID) (*domain.ShoppingList, error) {
	var list domain.ShoppingList
	if err := r.db.First(&list, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &list, nil
}

func (r *shoppingListRepository) ListByHousehold(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	query := r.db.Where("household_id = ?", householdID)

	var total int64
	if err := query.Model(&domain.ShoppingList{}).Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
	}

	var lists []domain.ShoppingList
	if err := query.Offset(offset).Limit(limit).Find(&lists).Error; err != nil {
		return nil, 0, mapErr(err)
	}
	return lists, total, nil
}

func (r *shoppingListRepository) DefaultForHousehold(householdID uuid.UUID) (*domain.ShoppingList, error) {
	var list domain.ShoppingList
	err := r.db.Where("household_id = ? AND is_default = ?", householdID, true).First(&list).Error
	if err != nil {
		return nil, mapErr(err) // ErrNotFound if absent — service interprets as "not yet created"
	}
	return &list, nil
}

func (r *shoppingListRepository) CreateList(list *domain.ShoppingList) error {
	return mapErr(r.db.Create(list).Error)
}

func (r *shoppingListRepository) DeleteList(id uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.ShoppingList{}, id).Error)
}

func (r *shoppingListRepository) ItemByID(id uuid.UUID) (*domain.ShoppingItem, error) {
	var item domain.ShoppingItem
	if err := r.db.Preload("Unit").Preload("Food").First(&item, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &item, nil
}

func (r *shoppingListRepository) ListItems(listID uuid.UUID, offset, limit int) ([]domain.ShoppingItem, int64, error) {
	query := r.db.Preload("Unit").Preload("Food").
		Where("shopping_list_id = ?", listID).
		Order("is_bought ASC, created DESC")

	var total int64
	if err := query.Model(&domain.ShoppingItem{}).Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
	}

	var items []domain.ShoppingItem
	if err := query.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, mapErr(err)
	}
	return items, total, nil
}

func (r *shoppingListRepository) CreateItems(items []*domain.ShoppingItem) error {
	if err := r.db.Create(&items).Error; err != nil {
		return mapErr(err)
	}

	for _, item := range items {
		if item.UnitID != nil || item.FoodID != nil {
			if err := r.db.Preload("Unit").Preload("Food").First(item, item.ID).Error; err != nil {
				return mapErr(err)
			}
		}
	}
	return nil
}

func (r *shoppingListRepository) UpdateItem(item *domain.ShoppingItem) error {
	return mapErr(r.db.Model(item).Select("amount", "text", "is_bought", "unit_id", "food_id").Updates(item).Error)
}

func (r *shoppingListRepository) DeleteItem(id uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.ShoppingItem{}, id).Error)
}
