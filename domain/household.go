package domain

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Household struct {
	ID   uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Name string    `json:"name"`

	Members      []*User         `gorm:"foreignKey:HouseholdID" json:"members,omitempty"`
	Collections  []*Collection   `gorm:"foreignKey:HouseholdID" json:"collections,omitempty"`
	ShoppingList []*ShoppingList `gorm:"foreignKey:HouseholdID" json:"shopping_lists,omitempty"`
}

func (h *Household) BeforeCreate(tx *gorm.DB) error {
	if h.ID == uuid.Nil {
		var err error
		h.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
