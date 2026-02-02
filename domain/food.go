package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Food struct {
	ID            uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	Name          string     `gorm:"uniqueIndex:idx_food_name,sort:desc" json:"name"`
	Icon          *string    `json:"icon,omitempty"`
	DefaultUnitID *uuid.UUID `gorm:"type:char(36);index" json:"default_unit_id,omitempty"`
	Updated       time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time  `gorm:"autoCreateTime" json:"-"`

	DefaultUnit *Unit       `gorm:"foreignKey:DefaultUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"default_unit,omitempty"`
	Taxonomies  []*Taxonomy `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
}

// TableName overrides the table name used by Food to `food`
func (f Food) TableName() string {
	return "food"
}

func (f *Food) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		var err error
		f.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
