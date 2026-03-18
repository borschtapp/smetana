package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type foodRepository struct {
	db *gorm.DB
}

func NewFoodRepository(db *gorm.DB) domain.FoodRepository {
	return &foodRepository{db: db}
}

func (r *foodRepository) FindOrCreate(food *domain.Food) error {
	if food.Slug != "" {
		if err := r.db.First(food, "slug = ?", food.Slug).Error; err == nil {
			return nil
		}
	}

	if err := r.db.First(&food, "lower(name) = lower(?)", food.Name).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(food)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.db.First(food, "lower(name) = lower(?)", food.Name).Error
	}

	return nil
}

func (r *foodRepository) Update(food *domain.Food) error {
	return r.db.Model(food).Select("name", "image_path", "default_unit_id").Updates(food).Error
}

func (r *foodRepository) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return r.db.Model(&domain.Food{ID: foodID}).Association("Taxonomies").Append(taxonomy)
}
