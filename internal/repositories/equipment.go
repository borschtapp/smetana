package repositories

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type equipmentRepository struct {
	db *gorm.DB
}

func NewEquipmentRepository(db *gorm.DB) domain.EquipmentRepository {
	return &equipmentRepository{db: db}
}

func (r *equipmentRepository) Search(query string, offset, limit int) ([]domain.Equipment, int64, error) {
	q := r.db.Model(&domain.Equipment{})
	if query != "" {
		q = q.Where("name LIKE ? OR slug LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var equipment []domain.Equipment
	if err := q.Offset(offset).Limit(limit).Find(&equipment).Error; err != nil {
		return nil, 0, err
	}
	return equipment, total, nil
}

func (r *equipmentRepository) FindOrCreate(equipment *domain.Equipment) error {
	if equipment.Slug == "" {
		equipment.Slug = utils.CreateTag(equipment.Name)
	}

	if err := r.db.First(equipment, "slug = ?", equipment.Slug).Error; err == nil {
		return nil
	}

	if err := r.db.First(equipment, "lower(name) = lower(?)", equipment.Name).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(equipment)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.db.First(equipment, "slug = ?", equipment.Slug).Error
	}

	return nil
}
