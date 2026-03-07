package repositories

import (
	"slices"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type CollectionRepository struct {
	db *gorm.DB
}

func NewCollectionRepository(db *gorm.DB) domain.CollectionRepository {
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

func (r *CollectionRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Collection, int64, error) {
	var collections []domain.Collection

	q := r.db.Model(&domain.Collection{}).
		Where("household_id = ?", householdID)

	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ? OR description LIKE ?", "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	} else if total == 0 {
		return collections, 0, nil
	}

	if len(opts.Preload) != 0 {
		if slices.Contains(opts.Preload, "recipes:5") {
			q = q.Preload("Recipes", func(db *gorm.DB) *gorm.DB {
				return db.Order("created DESC").Limit(5)
			})
		}

		if slices.Contains(opts.Preload, "recipes.images") {
			q = q.Preload("Recipes.Images")
		}

		if slices.Contains(opts.Preload, "total_recipes") {
			q = q.Select(`collections.*, (
					SELECT COUNT(*) FROM collection_recipes
					WHERE collection_recipes.collection_id = collections.id
				) AS total_recipes`)
		}
	}

	q = q.Offset(opts.Offset).Limit(opts.Limit)
	q = q.Order(clause.OrderByColumn{
		Column: clause.Column{Table: "collections", Name: opts.Sort},
		Desc:   strings.EqualFold(opts.Order, "DESC"),
	})

	if err := q.Find(&collections).Error; err != nil {
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
