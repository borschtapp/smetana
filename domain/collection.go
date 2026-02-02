package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Collection struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index" json:"household_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time `gorm:"autoCreateTime" json:"-"`

	Household *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"household,omitempty"`
	Recipes   []*Recipe  `gorm:"many2many:collection_recipes;" json:"recipes,omitempty"`
}

func (c *Collection) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		var err error
		c.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
