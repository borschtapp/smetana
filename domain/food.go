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

// FoodUnitUsage is the number of times a (food, unit) pair occurs across recipe ingredients
type FoodUnitUsage struct {
	FoodID  uuid.UUID
	UnitID  uuid.UUID
	Count   int
	Updated time.Time
}

type FoodRepository interface {
	ByID(id uuid.UUID) (*Food, error)
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Food, error)
	FindOrCreate(food *Food) error
	Search(query string, offset, limit int) ([]Food, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	// Detach clears the canonical pointer set by Merge, so the food stops resolving
	// as an alias. It does not restore rows Merge reassigned to the kept food.
	Detach(id uuid.UUID) error
	AddTaxonomy(foodID uuid.UUID, taxonomy *Taxonomy) error
	Update(food *Food) error

	// DefaultUnitUsageSince returns (food, unit) usage counts across recipe ingredients
	// for canonical foods with no default unit that changed after since. A food's
	// per-unit counts always partition its total usage, so callers can sum Count per
	// FoodID instead of running a separate total-count query.
	DefaultUnitUsageSince(since time.Time) ([]FoodUnitUsage, error)
	// BySlugsOrNames batch-resolves canonical foods matching any of the given slugs or
	// any of the given (already-lowercased) names.
	BySlugsOrNames(slugs, names []string) ([]Food, error)
	// MergeCandidates returns the work set (canonical foods updated after since) and the
	// full canonical catalog to compare it against for near-duplicate detection.
	MergeCandidates(since time.Time) (workSet, catalog []Food, err error)

	CreatePrice(price *FoodPrice) error
	ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]FoodPrice, int64, error)
	LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*FoodPrice, error)
	DeletePrice(householdID, id uuid.UUID) error

	CreateConversion(c *FoodUnitConversion) error
	ListConversions(foodID uuid.UUID) ([]FoodUnitConversion, error)
	UpdateConversion(c *FoodUnitConversion) error
	DeleteConversion(foodID, id uuid.UUID) error
	ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]FoodUnitConversion, error)
}

type FoodService interface {
	ByID(id uuid.UUID) (*Food, error)
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Food, error)
	FindOrCreate(ctx context.Context, food *Food) error
	Search(query string, offset, limit int) ([]Food, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	Detach(id uuid.UUID) error
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
