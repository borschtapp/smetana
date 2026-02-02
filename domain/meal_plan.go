package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MealPlan struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID  `gorm:"type:char(36);index" json:"household_id"`
	Date        time.Time  `gorm:"index" json:"date"`
	MealType    string     `json:"meal_type"` // breakfast, lunch, dinner
	RecipeID    *uuid.UUID `gorm:"type:char(36)" json:"recipe_id,omitempty"`
	Servings    *int       `json:"servings,omitempty"`
	Note        *string    `json:"note,omitempty"`
	Updated     time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time  `gorm:"autoCreateTime" json:"-"`

	Household *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Recipe    *Recipe    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"recipe,omitempty"`
}

func (mp *MealPlan) BeforeCreate(tx *gorm.DB) error {
	if mp.ID == uuid.Nil {
		var err error
		mp.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
