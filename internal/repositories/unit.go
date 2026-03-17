package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type unitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) domain.UnitRepository {
	return &unitRepository{db: db}
}

func (r *unitRepository) FindOrCreate(unit *domain.Unit) error {
	if unit.Slug != "" {
		if err := r.db.First(unit, "slug = ?", unit.Slug).Error; err == nil {
			return nil
		}
	}

	if err := r.db.First(unit, "lower(name) = lower(?)", unit.Name).Error; err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(unit).Error; err != nil {
		return err
	}

	if unit.ID == uuid.Nil { // fallback for conflict scenario
		return r.db.First(&unit, "lower(name) = lower(?)", unit.Name).Error
	}

	return nil
}

func (r *unitRepository) Update(unit *domain.Unit) error {
	return r.db.Model(unit).Select("name").Updates(unit).Error
}

func (r *unitRepository) AddTaxonomy(unitID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return r.db.Model(&domain.Unit{ID: unitID}).Association("Taxonomies").Append(taxonomy)
}
