package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type UnitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) domain.UnitRepository {
	return &UnitRepository{db: db}
}

func (r *UnitRepository) FindOrCreate(unit *domain.Unit) error {
	if unit.Slug != "" {
		if err := r.db.First(unit, "slug = ?", unit.Slug).Error; err == nil {
			return nil
		}
	}

	if err := r.db.First(unit, "lower(name) = lower(?)", unit.Name).Error; err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(unit).Error; err != nil {
		return err
	}

	if unit.ID == uuid.Nil { // fallback for conflict scenario
		return r.db.First(&unit, "lower(name) = lower(?)", unit.Name).Error
	}

	return nil
}
