package repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type authorRepository struct {
	db *gorm.DB
}

func NewAuthorRepository(db *gorm.DB) domain.AuthorRepository {
	return &authorRepository{db: db}
}

func (r *authorRepository) FindOrCreate(author *domain.Author) error {
	if err := r.findAuthor(author); err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(&author).Error; err != nil {
		return err
	}

	if author.ID == uuid.Nil { // fallback for conflict scenario
		return r.findAuthor(author)
	}

	return nil
}

func (r *authorRepository) findAuthor(author *domain.Author) error {
	if author.ID != uuid.Nil {
		return nil
	}
	if len(author.Url) > 0 {
		var existing domain.Author
		if err := r.db.First(&existing, "url = ?", author.Url).Error; err == nil {
			*author = existing
			return nil
		}
	}
	if len(author.Name) > 0 {
		var existing domain.Author
		if err := r.db.First(&existing, "name = ?", author.Name).Error; err == nil {
			*author = existing
			return nil
		}
	}
	return errors.New("author not found")
}
