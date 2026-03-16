package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShoppingList struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index" json:"-"`
	Name        string    `json:"name"`
	IsDefault   bool      `gorm:"default:false;uniqueIndex:idx_household_default,where:is_default = true" json:"is_default"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time `gorm:"autoCreateTime" json:"-"`

	Household *Household      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Items     []*ShoppingItem `gorm:"foreignKey:ShoppingListID" json:"items,omitempty"`
}

func (s *ShoppingList) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		var err error
		s.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type ShoppingItem struct {
	ID             uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	ShoppingListID uuid.UUID  `gorm:"type:char(36);index" json:"-"`
	Amount         *float64   `json:"amount,omitempty"`
	Text           string     `json:"text"` // raw user input
	UnitID         *uuid.UUID `gorm:"type:char(36);index" json:"unit_id,omitempty"`
	FoodID         *uuid.UUID `gorm:"type:char(36);index" json:"food_id,omitempty"`
	IsBought       bool       `gorm:"default:false" json:"is_bought"`
	Updated        time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created        time.Time  `gorm:"autoCreateTime" json:"-"`

	ShoppingList *ShoppingList `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit         *Unit         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"unit,omitempty"`
	Food         *Food         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"food,omitempty"`
}

func (s *ShoppingItem) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		var err error
		s.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type ShoppingListRepository interface {
	ByID(id uuid.UUID) (*ShoppingList, error)
	ListByHousehold(householdID uuid.UUID, offset, limit int) ([]ShoppingList, int64, error)
	DefaultForHousehold(householdID uuid.UUID) (*ShoppingList, error) // ErrNotFound if absent
	CreateList(list *ShoppingList) error
	DeleteList(id uuid.UUID) error

	ListItems(listID uuid.UUID, offset, limit int) ([]ShoppingItem, int64, error)
	ItemByID(id uuid.UUID) (*ShoppingItem, error)
	CreateItems(items []*ShoppingItem) error
	UpdateItem(item *ShoppingItem) error
	DeleteItem(id uuid.UUID) error
}

type ShoppingListService interface {
	Lists(householdID uuid.UUID, offset, limit int) ([]ShoppingList, int64, error)
	GetList(listID uuid.UUID, householdID uuid.UUID) (*ShoppingList, error)
	CreateList(list *ShoppingList, householdID uuid.UUID) error
	DeleteList(listID uuid.UUID, householdID uuid.UUID) error

	Items(listID uuid.UUID, householdID uuid.UUID, offset, limit int) ([]ShoppingItem, int64, error)
	AddItems(items []*ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error
	UpdateItem(item *ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error
	DeleteItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID) error
}
