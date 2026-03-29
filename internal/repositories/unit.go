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

func (r *unitRepository) ByID(id uuid.UUID) (*domain.Unit, error) {
	var unit domain.Unit
	if err := r.db.First(&unit, id).Error; err != nil {
		return nil, mapErr(err)
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
		return mapErr(result.Error)
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return mapErr(r.db.First(unit, "slug = ?", unit.Slug).Error)
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
		return nil, 0, mapErr(err)
	}

	var units []domain.Unit
	err := db.Offset(offset).Limit(limit).Find(&units).Error
	return units, total, mapErr(err)
}

func (r *unitRepository) Update(unit *domain.Unit) error {
	return mapErr(r.db.Model(unit).Select("name").Updates(unit).Error)
}

func (r *unitRepository) ByBase(baseUnitID uuid.UUID, imperial bool) ([]domain.Unit, error) {
	var units []domain.Unit
	err := r.db.Where("(id = ? OR base_unit_id = ?) AND imperial = ?", baseUnitID, baseUnitID, imperial).Find(&units).Error
	return units, mapErr(err)
}

func (r *unitRepository) AddTaxonomy(unitID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return mapErr(r.db.Model(&domain.Unit{ID: unitID}).Association("Taxonomies").Append(taxonomy))
}
