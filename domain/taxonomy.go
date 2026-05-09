package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/types"
)

const (
	TaxonomyTypeDiet     = "diet"
	TaxonomyTypeCategory = "category"
	TaxonomyTypeCuisine  = "cuisine"
	TaxonomyTypeKeyword  = "keyword"
)

type Taxonomy struct {
	ID          uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	Type        string     `gorm:"index" json:"type" validate:"required,oneof=diet category cuisine keyword"` // diet, category, cuisine, keyword
	Slug        string     `gorm:"uniqueIndex" json:"slug" validate:"required,min=1,max=255"`
	Label       string     `json:"label" validate:"required,min=1,max=255"`
	ParentID    *uuid.UUID `gorm:"type:char(36);index" json:"parent_id,omitempty"`
	CanonicalID *uuid.UUID `gorm:"type:char(36);index" json:"canonical_id,omitempty"` // For merging/aliases
	Updated     time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time  `gorm:"autoCreateTime" json:"-"`

	TotalRecipes *int64    `gorm:"->;-:migration" json:"total_recipes,omitempty"`
	Parent       *Taxonomy `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"parent,omitempty"`
	Canonical    *Taxonomy `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"canonical,omitempty"`

	Recipes []*Recipe `gorm:"many2many:recipe_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Foods   []*Food   `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Units   []*Unit   `gorm:"many2many:unit_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (t *Taxonomy) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		var err error
		t.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type TaxonomyRepository interface {
	Search(taxonomyType string, householdID uuid.UUID, opts types.SearchOptions) ([]Taxonomy, int64, error)
	FindOrCreate(taxonomy *Taxonomy) error
}

type TaxonomyService interface {
	Search(taxonomyType string, householdID uuid.UUID, opts types.SearchOptions) ([]Taxonomy, int64, error)
	FindOrCreate(taxonomy *Taxonomy) error
}
