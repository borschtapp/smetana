package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeIngredient struct {
	ID       uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	RecipeID uuid.UUID  `gorm:"type:char(36);index" json:"-"`
	Amount   *float64   `json:"amount"`
	UnitID   *uuid.UUID `gorm:"type:char(36);index" json:"unit_id,omitempty"`
	FoodID   *uuid.UUID `gorm:"type:char(36);index" json:"food_id,omitempty"`
	Kind     string     `gorm:"default:'main'" json:"kind"` // "main", "secondary", "essential"
	Note     *string    `json:"note,omitempty"`
	RawText  string     `json:"raw_text,omitempty"` // original unparsed value
	Updated  time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created  time.Time  `gorm:"autoCreateTime" json:"-"`

	Recipe *Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit   *Unit   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"unit,omitempty"`
	Food   *Food   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"food,omitempty"`
}

func (ri *RecipeIngredient) BeforeCreate(tx *gorm.DB) error {
	if ri.ID == uuid.Nil {
		var err error
		ri.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
