package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PieceUnitSlug is the slug of the seeded piece unit, the implicit unit of unitless amounts.
const PieceUnitSlug = "pc"

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

// IsPiece reports whether this is the seeded piece unit, the implicit unit of unitless amounts.
func (u *Unit) IsPiece() bool {
	return u.Slug == PieceUnitSlug && u.CanonicalUnitID == nil
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

type UnitRepository interface {
	FindOrCreate(unit *Unit) error
	ByID(id uuid.UUID) (*Unit, error)                              // resolves aliases to their canonical unit
	ByIDs(ids []uuid.UUID) (map[uuid.UUID]*Unit, error)            // like ByID, keyed by the requested ID
	CanonicalIDs(ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) // maps each alias among ids to its canonical unit
	BySlug(slug string) (*Unit, error)                             // resolves aliases to their canonical unit
	ByBase(baseUnitID uuid.UUID, imperial bool) ([]Unit, error)
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	AddTaxonomy(unitID uuid.UUID, taxonomy *Taxonomy) error
	Update(unit *Unit) error
}

// UnitConverter converts amounts and caches the units it loads. Use one per request; it is not safe for concurrent use.
type UnitConverter interface {
	// Add registers already-loaded units so they are never fetched. Nil entries are skipped.
	Add(units ...*Unit)
	// Expect queues units for the next lookup, so a batch costs one query, or none if nothing converts.
	Expect(unitIDs ...uuid.UUID)
	// Convert scales an amount between units, bridging dimensions via food conversions. A nil unit means pieces.
	Convert(amount float64, fromUnitID, toUnitID *uuid.UUID, conversions []FoodUnitConversion) (float64, error)
}

type UnitService interface {
	ByID(id uuid.UUID) (*Unit, error) // resolves aliases to their canonical unit
	// Canonicalize replaces alias unit IDs in place with their canonical IDs. Nil pointers are skipped.
	Canonicalize(unitIDs ...*uuid.UUID) error
	FindOrCreate(unit *Unit) error
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	Merge(keepID, mergeID uuid.UUID) error
	Update(unit *Unit) error
	// NewConverter returns a request-scoped UnitConverter.
	NewConverter() UnitConverter
	// BestUnit finds the most human-readable unit in the target system for the given amount.
	BestUnit(amount float64, fromUnitID uuid.UUID, imperial bool) (*Unit, error)
}
