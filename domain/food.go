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
	ID              uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Slug            string        `gorm:"uniqueIndex:idx_food_slug,sort:desc" json:"slug" validate:"required,min=1,max=255"`
	Name            string        `json:"name" validate:"required,min=1,max=255"`
	Description     *string       `json:"description,omitempty" validate:"omitempty,max=1000"`
	ImagePath       *storage.Path `json:"image_url,omitempty"`
	DefaultUnitID   *uuid.UUID    `gorm:"type:char(36);index" json:"default_unit_id,omitempty"`
	Pantry          bool          `json:"pantry"` // Whether this food is always available (e.g. salt, oil) and should be excluded from shopping lists
	CanonicalFoodID *uuid.UUID    `gorm:"type:char(36);index" json:"canonical_food_id,omitempty"`
	Updated         time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created         time.Time     `gorm:"autoCreateTime" json:"-"`

	CanonicalFood *Food       `gorm:"foreignKey:CanonicalFoodID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	DefaultUnit   *Unit       `gorm:"foreignKey:DefaultUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"default_unit,omitempty"`
	Taxonomies    []*Taxonomy `gorm:"many2many:food_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
	Images        []*Image    `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
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

// ShoppingItemCombiner folds incoming into target. On true, target is saved and incoming deleted; on false both stay.
type ShoppingItemCombiner func(target, incoming *ShoppingItem) (bool, error)

type FoodRepository interface {
	ByID(id uuid.UUID) (*Food, error)                              // resolves aliases to their canonical food
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Food, error)            // like ByID, keyed by the requested ID
	CanonicalIDs(ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) // maps each alias among ids to its canonical food
	FindOrCreate(food *Food) error
	Search(query string, offset, limit int) ([]Food, int64, error)
	// Merge makes mergeID an alias of keepID. Where a list holds both foods, combine decides; nil never combines.
	Merge(keepID, mergeID uuid.UUID, combine ShoppingItemCombiner) error
	AddTaxonomy(foodID uuid.UUID, taxonomy *Taxonomy) error
	Update(food *Food) error

	CreatePrice(price *FoodPrice) error
	ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]FoodPrice, int64, error)
	LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*FoodPrice, error)
	DeletePrice(householdID, id uuid.UUID) error

	CreateConversion(c *FoodUnitConversion) error
	ListConversions(foodID uuid.UUID) ([]FoodUnitConversion, error)
	ConversionByID(foodID, id uuid.UUID) (*FoodUnitConversion, error)
	UpdateConversion(c *FoodUnitConversion) error
	DeleteConversion(foodID, id uuid.UUID) error
	ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]FoodUnitConversion, error)
}

type FoodService interface {
	ByID(id uuid.UUID) (*Food, error) // resolves aliases to their canonical food
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Food, error)
	// Canonicalize replaces alias food IDs in place with their canonical IDs. Nil pointers are skipped.
	Canonicalize(foodIDs ...*uuid.UUID) error
	FindOrCreate(ctx context.Context, food *Food) error
	Search(query string, offset, limit int) ([]Food, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	AddTaxonomy(foodID uuid.UUID, taxonomy *Taxonomy) error
	Update(food *Food) error

	RecordPrice(householdID uuid.UUID, price *FoodPrice) error
	ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]FoodPrice, int64, error)
	LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*FoodPrice, error)
	DeletePrice(householdID, id uuid.UUID) error

	CreateConversion(c *FoodUnitConversion) error
	ListConversions(foodID uuid.UUID) ([]FoodUnitConversion, error)
	UpdateConversion(c *FoodUnitConversion) error
	DeleteConversion(foodID, id uuid.UUID) error
	ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]FoodUnitConversion, error)
}
