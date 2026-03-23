package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Food struct {
	ID            uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Slug          string        `gorm:"uniqueIndex:idx_food_slug,sort:desc" json:"slug"`
	Name          string        `json:"name"`
	Description   *string       `json:"description,omitempty"`
	ImagePath     *storage.Path `json:"image_url,omitempty"`
	DefaultUnitID *uuid.UUID    `gorm:"type:char(36);index" json:"default_unit_id,omitempty"`
	Pantry        bool          `json:"pantry"` // Whether this food is always available (e.g. salt, oil) and should be excluded from shopping lists
	Updated       time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time     `gorm:"autoCreateTime" json:"-"`

	DefaultUnit *Unit       `gorm:"foreignKey:DefaultUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"default_unit,omitempty"`
	Taxonomies  []*Taxonomy `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
	Images      []*Image    `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
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

	CreatePrice(price *FoodPrice) error
	ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]FoodPrice, int64, error)
	LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*FoodPrice, error)
	DeletePrice(householdID, id uuid.UUID) error
}

type FoodService interface {
	FindOrCreate(ctx context.Context, food *Food) error
	AddTaxonomy(foodID uuid.UUID, taxonomy *Taxonomy) error
	Update(food *Food) error

	RecordPrice(householdID uuid.UUID, price *FoodPrice) error
	ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]FoodPrice, int64, error)
	LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*FoodPrice, error)
	DeletePrice(householdID, id uuid.UUID) error
}
