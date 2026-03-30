package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Unit struct {
	ID         uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	Slug       string     `gorm:"uniqueIndex:idx_unit_slug,sort:desc" json:"slug"`
	Name       string     `json:"name"`
	Imperial   bool       `json:"imperial"`
	BaseUnitID *uuid.UUID `gorm:"type:char(36);index" json:"base_unit_id,omitempty"`
	// BaseFactor is the multiplier to convert amounts to the base unit.
	BaseFactor float64   `gorm:"default:0" json:"base_factor,omitempty"`
	Updated    time.Time `gorm:"autoUpdateTime" json:"-"`
	Created    time.Time `gorm:"autoCreateTime" json:"-"`

	BaseUnit   *Unit       `gorm:"foreignKey:BaseUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"base_unit,omitempty"`
	Taxonomies []*Taxonomy `gorm:"many2many:unit_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
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

type UnitRepository interface {
	FindOrCreate(unit *Unit) error
	ByID(id uuid.UUID) (*Unit, error)
	ByBase(baseUnitID uuid.UUID, imperial bool) ([]Unit, error)
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	AddTaxonomy(unitID uuid.UUID, taxonomy *Taxonomy) error
	Update(unit *Unit) error
}

type UnitService interface {
	FindOrCreate(unit *Unit) error
	Search(query string, imperial *bool, offset, limit int) ([]Unit, int64, error)
	// Convert scales amount from one unit to another using their shared base-unit chain.
	Convert(amount float64, fromUnitID, toUnitID uuid.UUID) (float64, error)
	// BestUnit finds the most human-readable unit in the target system for the given amount.
	BestUnit(amount float64, fromUnitID uuid.UUID, imperial bool) (*Unit, error)
}
