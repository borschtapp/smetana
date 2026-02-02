package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Taxonomy struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	Type        string     `gorm:"index" json:"type"` // diet, category, cuisine, keyword
	Slug        string     `gorm:"uniqueIndex" json:"slug"`
	Label       string     `json:"label"`
	ParentID    *uuid.UUID `gorm:"type:char(36);index" json:"parent_id,omitempty"`
	CanonicalID *uuid.UUID `gorm:"type:char(36);index" json:"canonical_id,omitempty"` // For merging/aliases
	Updated     time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time  `gorm:"autoCreateTime" json:"-"`

	Parent    *Taxonomy `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"parent,omitempty"`
	Canonical *Taxonomy `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"canonical,omitempty"`

	Recipes []*Recipe `gorm:"many2many:recipe_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"recipes,omitempty"`
	Foods   []*Food   `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"foods,omitempty"`
	Units   []*Unit   `gorm:"many2many:unit_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"units,omitempty"`
}

func (t *Taxonomy) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		var err error
		t.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
