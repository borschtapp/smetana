package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Unit struct {
	ID         uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	Slug       string     `gorm:"uniqueIndex:idx_unit_slug,sort:desc" json:"slug" validate:"required,min=1,max=255"`
	Name       string     `json:"name" validate:"required,min=1,max=255"`
	Imperial   bool       `json:"imperial"`
	BaseUnitID *uuid.UUID `gorm:"type:char(36);index" json:"base_unit_id,omitempty"`
	// BaseFactor is the multiplier to convert amounts to the base unit.
	BaseFactor      float64    `gorm:"default:0" json:"base_factor,omitempty"`
	CanonicalUnitID *uuid.UUID `gorm:"type:char(36);index" json:"canonical_unit_id,omitempty"`
	Updated         time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created         time.Time  `gorm:"autoCreateTime" json:"-"`

	CanonicalUnit *Unit       `gorm:"foreignKey:CanonicalUnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	BaseUnit      *Unit       `gorm:"foreignKey:BaseUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"base_unit,omitempty"`
	Taxonomies    []*Taxonomy `gorm:"many2many:unit_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
}

func (u *Unit) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		var err error
		u.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

// BaseID returns the root base-unit ID: own ID for base units, BaseUnitID for derived units.
func (u *Unit) BaseID() uuid.UUID {
	if u.BaseUnitID != nil {
		return *u.BaseUnitID
	}
	return u.ID
}

// Convertible reports whether this unit can be converted to other units.
func (u *Unit) Convertible() bool {
	return u.BaseFactor != 0
}

// ToBaseFactor returns the multiplier relative to the base unit.
// Base units return 1.0; derived units return BaseFactor or error if BaseFactor is unset.
func (u *Unit) ToBaseFactor() (float64, error) {
	if !u.Convertible() {
		return 0, errors.New("unit is not convertible")
	}
	if u.BaseUnitID == nil {
		return 1.0, nil
	}
	return u.BaseFactor, nil
}

// FoodUnitPair identifies a food/unit combination observed in a recipe ingredient.
type FoodUnitPair struct {
	FoodID uuid.UUID
	UnitID uuid.UUID
}

type UnitRepository interface {
	FindOrCreate(unit *Unit) error
	ByID(id uuid.UUID) (*Unit, error)
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Unit, error)
	ByBase(baseUnitID uuid.UUID, imperial bool) ([]Unit, error)
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	// Detach clears the canonical pointer set by Merge, so the unit stops resolving
	// as an alias. It does not restore rows Merge reassigned to the kept unit.
	Detach(id uuid.UUID) error
	AddTaxonomy(unitID uuid.UUID, taxonomy *Taxonomy) error
	Update(unit *Unit) error

	// NonConvertibleUsageSince returns distinct (food, unit) pairs, for canonical foods
	// updated after since, where the unit is not convertible (see Unit.Convertible) and
	// no FoodUnitConversion row exists yet to bridge the dimension mismatch.
	NonConvertibleUsageSince(since time.Time) ([]FoodUnitPair, error)
	// MergeCandidates returns the work set (canonical units updated after since) and the
	// full canonical catalog to compare it against for near-duplicate detection.
	MergeCandidates(since time.Time) (workSet, catalog []Unit, err error)
}

type UnitService interface {
	ByID(id uuid.UUID) (*Unit, error)
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Unit, error)
	FindOrCreate(unit *Unit) error
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	Detach(id uuid.UUID) error
	Update(unit *Unit) error
	// Convert scales amount from one unit to another using their shared base-unit chain.
	Convert(amount float64, fromUnitID, toUnitID uuid.UUID) (float64, error)
	// ConvertWithConversions scales an amount across different dimensions using food-specific unit conversions.
	ConvertWithConversions(amount float64, fromUnitID, toUnitID uuid.UUID, conversions []FoodUnitConversion) (float64, error)
	// BestUnit finds the most human-readable unit in the target system for the given amount.
	BestUnit(amount float64, fromUnitID uuid.UUID, imperial bool) (*Unit, error)
}
