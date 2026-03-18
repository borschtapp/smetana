package repositories

import (
	"errors"

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
	if err := r.find(author); err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(author)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return r.find(author)
	}

	return nil
}

func (r *authorRepository) find(author *domain.Author) error {
	if author.Url != nil && len(*author.Url) > 0 {
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
