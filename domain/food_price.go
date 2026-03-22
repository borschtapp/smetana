package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FoodPrice is a single price observation for a food, scoped to a household.
// Price is expressed as: Price <Currency> per Amount <Unit>.
// Example: 4.99 EUR per 1 kg of chicken breast.
type FoodPrice struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index:idx_food_price_lookup" json:"household_id"`
	FoodID      uuid.UUID `gorm:"type:char(36);index:idx_food_price_lookup" json:"food_id"`
	UnitID      uuid.UUID `gorm:"type:char(36)" json:"unit_id"`
	Price       float64   `gorm:"not null" json:"price"`
	Amount      float64   `gorm:"not null;default:1" json:"amount"`
	Created     time.Time `gorm:"index:idx_food_price_lookup,sort:desc;not null;autoCreateTime" json:"created"`

	Food *Food `gorm:"foreignKey:FoodID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"food,omitempty"`
	Unit *Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" json:"unit,omitempty"`
}

func (gp *FoodPrice) BeforeCreate(_ *gorm.DB) error {
	if gp.ID == uuid.Nil {
		var err error
		gp.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return nil
}
