package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Unit struct {
	ID      uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Name    string    `gorm:"uniqueIndex:idx_unit_name,sort:desc" json:"name"`
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	Taxonomies []*Taxonomy `gorm:"many2many:unit_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
}

func (u *Unit) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		var err error
		u.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
