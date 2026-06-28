package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FoodUnitConversion struct {
	ID           uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	FoodID       uuid.UUID `gorm:"type:char(36);uniqueIndex:idx_food_unit,priority:1" json:"food_id"`
	UnitID       uuid.UUID `gorm:"type:char(36);uniqueIndex:idx_food_unit,priority:2" json:"unit_id"` // source, e.g. piece
	TargetAmount float64   `json:"target_amount" validate:"required,gt=0"`                            // e.g. 200
	TargetUnitID uuid.UUID `gorm:"type:char(36);index" json:"target_unit_id"`                         // e.g. gram
	Updated      time.Time `gorm:"autoUpdateTime" json:"-"`
	Created      time.Time `gorm:"autoCreateTime" json:"-"`

	Food       *Food `gorm:"foreignKey:FoodID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit       *Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"unit,omitempty"`
	TargetUnit *Unit `gorm:"foreignKey:TargetUnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"target_unit,omitempty"`
}

func (f *FoodUnitConversion) BeforeCreate(_ *gorm.DB) error {
	if f.ID == uuid.Nil {
		var err error
		f.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
