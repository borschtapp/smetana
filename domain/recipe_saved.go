package domain

import (
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
)

// RecipeSavedUser is a slim projection used in Recipe.SavedBy
type RecipeSavedUser struct {
	RecipeID  uuid.UUID     `gorm:"column:recipe_id" json:"-"`
	ID        uuid.UUID     `gorm:"column:user_id" json:"id"`
	Name      string        `json:"name"`
	ImagePath *storage.Path `json:"image_url,omitempty"`
}

type RecipeSaved struct {
	UserID      uuid.UUID `gorm:"type:char(36);primaryKey" json:"user_id"`
	RecipeID    uuid.UUID `gorm:"type:char(36);primaryKey" json:"recipe_id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index" json:"-"`
	Created     time.Time `gorm:"autoCreateTime" json:"-"`

	User      *User      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Recipe    *Recipe    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Household *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (rs RecipeSaved) TableName() string {
	return "recipes_saved"
}
