package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type equipmentRepository struct {
	db *gorm.DB
}

func NewEquipmentRepository(db *gorm.DB) domain.EquipmentRepository {
	return &equipmentRepository{db: db}
}

func (r *equipmentRepository) FindOrCreate(equipment *domain.Equipment) error {
	if equipment.Slug != "" {
		if err := r.db.First(equipment, "slug = ?", equipment.Slug).Error; err == nil {
			return nil
		}
	}

	if err := r.db.First(&equipment, "lower(name) = lower(?)", equipment.Name).Error; err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(equipment).Error; err != nil {
		return err
	}

	if equipment.ID == uuid.Nil { // fallback for conflict scenario
		return r.db.First(&equipment, "lower(name) = lower(?)", equipment.Name).Error
	}

	return nil
}
