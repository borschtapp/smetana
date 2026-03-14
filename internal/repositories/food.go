package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type FoodRepository struct {
	db *gorm.DB
}

func NewFoodRepository(db *gorm.DB) domain.FoodRepository {
	return &FoodRepository{db: db}
}

func (r *FoodRepository) FindOrCreate(food *domain.Food) error {
	if food.Slug != "" {
		if err := r.db.First(food, "slug = ?", food.Slug).Error; err == nil {
			return nil
		}
	}

	if err := r.db.First(&food, "lower(name) = lower(?)", food.Name).Error; err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(food).Error; err != nil {
		return err
	}

	if food.ID == uuid.Nil { // fallback for conflict scenario
		return r.db.First(&food, "name = ?", food.Name).Error
	}

	return nil
}

func (r *FoodRepository) Update(food *domain.Food) error {
	return r.db.Model(food).Select("name", "image_path", "default_unit_id").Updates(food).Error
}

func (r *FoodRepository) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return r.db.Model(&domain.Food{ID: foodID}).Association("Taxonomies").Append(taxonomy)
}
