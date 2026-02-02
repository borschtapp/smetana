package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShoppingList struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID  `gorm:"type:char(36);index" json:"household_id"`
	Product     string     `json:"product"`
	Quantity    *float64   `json:"quantity,omitempty"`
	UnitID      *uuid.UUID `gorm:"type:char(36);index" json:"unit_id,omitempty"`
	IsBought    bool       `gorm:"default:false" json:"is_bought"`
	Updated     time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time  `gorm:"autoCreateTime" json:"-"`

	Household *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit      *Unit      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"unit,omitempty"`
}

func (s *ShoppingList) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		var err error
		s.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
