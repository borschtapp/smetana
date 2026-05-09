package repositories

import (
	"fmt"

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

func (r *unitRepository) ByID(id uuid.UUID) (*domain.Unit, error) {
	var unit domain.Unit
	if err := r.db.First(&unit, id).Error; err != nil {
		return nil, fmt.Errorf("unit by id %s: %w", id, mapErr(err))
	}
	return &unit, nil
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
		return fmt.Errorf("create unit: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return fmt.Errorf("find unit after conflict: %w", mapErr(r.db.First(unit, "slug = ?", unit.Slug).Error))
	}

	return nil
}

func (r *unitRepository) Search(query string, imperial *bool, offset, limit int) ([]domain.Unit, int64, error) {
	db := r.db.Model(&domain.Unit{})
	if query != "" {
		db = db.Where("name LIKE ? OR slug LIKE ?", "%"+query+"%", "%"+query+"%")
	}
	if imperial != nil {
		db = db.Where("imperial = ?", *imperial)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count units: %w", mapErr(err))
	}

	var units []domain.Unit
	if err := db.Offset(offset).Limit(limit).Find(&units).Error; err != nil {
		return nil, 0, fmt.Errorf("search find units: %w", mapErr(err))
	}
	return units, total, nil
}

func (r *unitRepository) Update(unit *domain.Unit) error {
	if err := r.db.Model(unit).Select("name").Updates(unit).Error; err != nil {
		return fmt.Errorf("update unit %s: %w", unit.ID, mapErr(err))
	}
	return nil
}

func (r *unitRepository) ByBase(baseUnitID uuid.UUID, imperial bool) ([]domain.Unit, error) {
	var units []domain.Unit
	if err := r.db.Where("(id = ? OR base_unit_id = ?) AND imperial = ?", baseUnitID, baseUnitID, imperial).Find(&units).Error; err != nil {
		return nil, fmt.Errorf("find units by base %s: %w", baseUnitID, mapErr(err))
	}
	return units, nil
}

func (r *unitRepository) AddTaxonomy(unitID uuid.UUID, taxonomy *domain.Taxonomy) error {
	if err := r.db.Model(&domain.Unit{ID: unitID}).Association("Taxonomies").Append(taxonomy); err != nil {
		return fmt.Errorf("add taxonomy %s to unit %s: %w", taxonomy.ID, unitID, mapErr(err))
	}
	return nil
}
