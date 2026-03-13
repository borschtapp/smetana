package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type ShoppingListRepository struct {
	db *gorm.DB
}

func NewShoppingListRepository(db *gorm.DB) domain.ShoppingListRepository {
	return &ShoppingListRepository{db: db}
}

func (r *ShoppingListRepository) ByID(id uuid.UUID) (*domain.ShoppingList, error) {
	var list domain.ShoppingList
	if err := r.db.First(&list, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &list, nil
}

func (r *ShoppingListRepository) ListByHousehold(householdID uuid.UUID) ([]domain.ShoppingList, error) {
	var lists []domain.ShoppingList
	if err := r.db.Where("household_id = ?", householdID).Find(&lists).Error; err != nil {
		return nil, err
	}
	return lists, nil
}

func (r *ShoppingListRepository) DefaultForHousehold(householdID uuid.UUID) (*domain.ShoppingList, error) {
	var list domain.ShoppingList
	err := r.db.Where("household_id = ? AND is_default = ?", householdID, true).First(&list).Error
	if err != nil {
		return nil, mapErr(err) // ErrNotFound if absent — service interprets as "not yet created"
	}
	return &list, nil
}

func (r *ShoppingListRepository) CreateList(list *domain.ShoppingList) error {
	return r.db.Create(list).Error
}

func (r *ShoppingListRepository) DeleteList(id uuid.UUID) error {
	return r.db.Delete(&domain.ShoppingList{}, id).Error
}

func (r *ShoppingListRepository) ItemByID(id uuid.UUID) (*domain.ShoppingItem, error) {
	var item domain.ShoppingItem
	if err := r.db.Preload("Unit").First(&item, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &item, nil
}

func (r *ShoppingListRepository) ListItems(listID uuid.UUID, offset, limit int) ([]domain.ShoppingItem, int64, error) {
	query := r.db.Preload("Unit").Where("shopping_list_id = ?", listID)

	var total int64
	if err := query.Model(&domain.ShoppingItem{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []domain.ShoppingItem
	if err := query.Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *ShoppingListRepository) CreateItem(item *domain.ShoppingItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return err
	}
	if item.UnitID != nil {
		return r.db.Preload("Unit").First(item, item.ID).Error
	}
	return nil
}

func (r *ShoppingListRepository) UpdateItem(item *domain.ShoppingItem) error {
	return r.db.Model(item).Select("product", "quantity", "unit_id", "is_bought").Updates(item).Error
}

func (r *ShoppingListRepository) DeleteItem(id uuid.UUID) error {
	return r.db.Delete(&domain.ShoppingItem{}, id).Error
}
