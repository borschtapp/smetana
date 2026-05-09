package repositories

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type collectionRepository struct {
	db *gorm.DB
}

func NewCollectionRepository(db *gorm.DB) domain.CollectionRepository {
	return &collectionRepository{db: db}
}

func (r *collectionRepository) ByID(id uuid.UUID) (*domain.Collection, error) {
	var collection domain.Collection
	if err := r.db.First(&collection, id).Error; err != nil {
		return nil, fmt.Errorf("collection by id %s: %w", id, mapErr(err))
	}
	return &collection, nil
}

func (r *collectionRepository) ByIdWithRecipes(id uuid.UUID) (*domain.Collection, error) {
	var collection domain.Collection
	if err := r.db.Preload("Recipes").First(&collection, id).Error; err != nil {
		return nil, fmt.Errorf("collection by id %s with recipes: %w", id, mapErr(err))
	}
	return &collection, nil
}

func (r *collectionRepository) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Collection, int64, error) {
	var collections []domain.Collection

	q := r.db.Model(&domain.Collection{}).
		Where("household_id = ?", householdID)

	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ? OR description LIKE ?", "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count collections: %w", mapErr(err))
	} else if total == 0 {
		return collections, 0, nil
	}

	q = q.Select("collections.*")

	if opts.Has("total_recipes") {
		q = q.Select(`collections.*, (
				SELECT COUNT(*) FROM collection_recipes
				WHERE collection_recipes.collection_id = collections.id
			) AS total_recipes`)
	}

	q = q.Offset(opts.Offset).Limit(opts.Limit)
	q = q.Order(clause.OrderByColumn{
		Column: clause.Column{Table: "collections", Name: opts.Sort},
		Desc:   strings.EqualFold(opts.Order, "DESC"),
	})

	if err := q.Find(&collections).Error; err != nil {
		return nil, 0, fmt.Errorf("search find collections: %w", mapErr(err))
	}

	if opts.Has("last3_recipes") {
		for i := range collections {
			if err := r.db.Select("recipes.*").
				Joins("JOIN collection_recipes ON collection_recipes.recipe_id = recipes.id").
				Where("collection_recipes.collection_id = ?", collections[i].ID).
				Order("recipes.created DESC").
				Limit(3).
				Find(&collections[i].Recipes).Error; err != nil {
				return nil, 0, fmt.Errorf("find last 3 recipes for collection %s: %w", collections[i].ID, mapErr(err))
			}
		}
	}

	return collections, total, nil
}

func (r *collectionRepository) Create(collection *domain.Collection) error {
	if err := r.db.Create(collection).Error; err != nil {
		return fmt.Errorf("create collection: %w", mapErr(err))
	}
	return nil
}

func (r *collectionRepository) Update(collection *domain.Collection) error {
	if err := r.db.Model(collection).Select("name", "description").Updates(collection).Error; err != nil {
		return fmt.Errorf("update collection %s: %w", collection.ID, mapErr(err))
	}
	return nil
}

func (r *collectionRepository) Delete(id uuid.UUID) error {
	if err := r.db.Delete(&domain.Collection{}, id).Error; err != nil {
		return fmt.Errorf("delete collection %s: %w", id, mapErr(err))
	}
	return nil
}

func (r *collectionRepository) AddRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	if err := r.db.Model(collection).Association("Recipes").Append(&domain.Recipe{ID: recipeID}); err != nil {
		return fmt.Errorf("add recipe %s to collection %s: %w", recipeID, collection.ID, mapErr(err))
	}
	return nil
}

func (r *collectionRepository) RemoveRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	if err := r.db.Model(collection).Association("Recipes").Delete(&domain.Recipe{ID: recipeID}); err != nil {
		return fmt.Errorf("remove recipe %s from collection %s: %w", recipeID, collection.ID, mapErr(err))
	}
	return nil
}
