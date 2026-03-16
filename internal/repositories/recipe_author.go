package repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type recipeAuthorRepository struct {
	db *gorm.DB
}

func NewRecipeAuthorRepository(db *gorm.DB) domain.RecipeAuthorRepository {
	return &recipeAuthorRepository{db: db}
}

func (r *recipeAuthorRepository) FindOrCreate(author *domain.RecipeAuthor) error {
	if err := r.findAuthor(author); err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&author).Error; err != nil {
		return err
	}

	if author.ID == uuid.Nil { // fallback for conflict scenario
		return r.findAuthor(author)
	}

	return nil
}

func (r *recipeAuthorRepository) findAuthor(author *domain.RecipeAuthor) error {
	if author.ID != uuid.Nil {
		return nil
	}
	if len(author.Url) > 0 {
		var existing domain.RecipeAuthor
		if err := r.db.First(&existing, "url = ?", author.Url).Error; err == nil {
			*author = existing
			return nil
		}
	}
	if len(author.Name) > 0 {
		var existing domain.RecipeAuthor
		if err := r.db.First(&existing, "name = ?", author.Name).Error; err == nil {
			*author = existing
			return nil
		}
	}
	return errors.New("author not found")
}
