package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type CollectionRepository struct {
	db *gorm.DB
}

func NewCollectionRepository(db *gorm.DB) *CollectionRepository {
	return &CollectionRepository{db: db}
}

func (r *CollectionRepository) ByID(id uuid.UUID) (*domain.Collection, error) {
	var collection domain.Collection
	if err := r.db.First(&collection, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &collection, nil
}

func (r *CollectionRepository) ByIdWithRecipes(id uuid.UUID) (*domain.Collection, error) {
	var collection domain.Collection
	if err := r.db.Preload("Recipes").First(&collection, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &collection, nil
}

func (r *CollectionRepository) List(householdID uuid.UUID, offset, limit int) ([]domain.Collection, int64, error) {
	query := r.db.Where("household_id = ?", householdID)

	var total int64
	if err := query.Model(&domain.Collection{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var collections []domain.Collection
	if err := query.Offset(offset).Limit(limit).Find(&collections).Error; err != nil {
		return nil, 0, err
	}
	return collections, total, nil
}

func (r *CollectionRepository) Create(collection *domain.Collection) error {
	return r.db.Create(collection).Error
}

func (r *CollectionRepository) Update(collection *domain.Collection) error {
	return r.db.Model(collection).Select("name", "description").Updates(collection).Error
}

func (r *CollectionRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Collection{}, id).Error
}

func (r *CollectionRepository) AddRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	return r.db.Model(collection).Association("Recipes").Append(&domain.Recipe{ID: recipeID})
}

func (r *CollectionRepository) RemoveRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	return r.db.Model(collection).Association("Recipes").Delete(&domain.Recipe{ID: recipeID})
}
