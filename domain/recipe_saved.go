package domain

import (
	"time"

	"github.com/google/uuid"
)

type RecipeSaved struct {
	UserID      uuid.UUID `gorm:"type:char(36);primaryKey" json:"user_id"`
	RecipeID    uuid.UUID `gorm:"type:char(36);primaryKey" json:"recipe_id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index" json:"-"`
	IsFavorite  bool      `json:"is_favorite"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time `gorm:"autoCreateTime" json:"-"`

	User      *User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Recipe    *Recipe    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Household *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (rs RecipeSaved) TableName() string {
	return "recipes_saved"
}
