package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/storage"
)

type Food struct {
	ID            uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Slug          string        `gorm:"uniqueIndex:idx_food_slug,sort:desc" json:"slug"`
	Name          string        `json:"name"`
	Icon          *storage.Path `json:"icon,omitempty"`
	DefaultUnitID *uuid.UUID    `gorm:"type:char(36);index" json:"default_unit_id,omitempty"`
	Updated       time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time     `gorm:"autoCreateTime" json:"-"`

	RemoteIcon *string `gorm:"-" json:"-"` // transient: remote icon URL from import, not persisted

	DefaultUnit *Unit       `gorm:"foreignKey:DefaultUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"default_unit,omitempty"`
	Taxonomies  []*Taxonomy `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
}

func (f Food) TableName() string {
	return "food"
}

func (f *Food) BeforeCreate(_ *gorm.DB) error {
	if f.ID == uuid.Nil {
		var err error
		f.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type FoodRepository interface {
	FindOrCreate(food *Food) error
	AddTaxonomy(foodID uuid.UUID, taxonomy *Taxonomy) error
	Update(food *Food) error
}

type FoodService interface {
	FindOrCreate(food *Food) error
}
