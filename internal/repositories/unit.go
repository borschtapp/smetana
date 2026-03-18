package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type unitRepository struct {
	db *gorm.DB
}

func NewUnitRepository(db *gorm.DB) domain.UnitRepository {
	return &unitRepository{db: db}
}

func (r *unitRepository) FindOrCreate(unit *domain.Unit) error {
	if unit.Slug == "" {
		unit.Slug = utils.CreateTag(unit.Name)
	}

	if err := r.db.First(unit, "slug = ?", unit.Slug).Error; err == nil {
		return nil
	}

	if err := r.db.First(unit, "lower(name) = lower(?)", unit.Name).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(unit)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.db.First(unit, "slug = ?", unit.Slug).Error
	}

	return nil
}

func (r *unitRepository) Update(unit *domain.Unit) error {
	return r.db.Model(unit).Select("name").Updates(unit).Error
}

func (r *unitRepository) AddTaxonomy(unitID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return r.db.Model(&domain.Unit{ID: unitID}).Association("Taxonomies").Append(taxonomy)
}
