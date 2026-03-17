package repositories

import (
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type publisherRepository struct {
	db *gorm.DB
}

func NewPublisherRepository(db *gorm.DB) domain.PublisherRepository {
	return &publisherRepository{db: db}
}

func (r *publisherRepository) Search(opts types.SearchOptions) ([]domain.Publisher, int64, error) {
	var publishers []domain.Publisher

	q := r.db.Model(&domain.Publisher{})

	if opts.SearchQuery != "" {
		q = q.Where("name LIKE ?", "%"+opts.SearchQuery+"%")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	} else if total == 0 {
		return nil, 0, nil
	}

	q = q.Select("publishers.*")

	if len(opts.Preload) != 0 {
		if slices.Contains(opts.Preload, "feeds") {
			q = q.Preload("Feeds")
		}

		if slices.Contains(opts.Preload, "images") {
			q = q.Preload("Images")
		}

		if slices.Contains(opts.Preload, "recipes:5") {
			q = q.Preload("Recipes", func(db *gorm.DB) *gorm.DB {
				return db.Order("created DESC").Limit(5)
			})
		}

		if slices.Contains(opts.Preload, "recipes.images") {
			q = q.Preload("Recipes.Images")
		}

		if slices.Contains(opts.Preload, "total_recipes") {
			q = q.Select(`publishers.*, (
					SELECT COUNT(*) FROM recipes
					WHERE recipes.publisher_id = publishers.id
				) AS total_recipes`)
		}
	}

	q = q.Offset(opts.Offset).Limit(opts.Limit)
	q = q.Order(clause.OrderByColumn{
		Column: clause.Column{Table: "publishers", Name: opts.Sort},
		Desc:   strings.EqualFold(opts.Order, "DESC"),
	})

	if err := q.Find(&publishers).Error; err != nil {
		return nil, 0, err
	}
	return publishers, total, nil
}

func (r *publisherRepository) FindOrCreate(pub *domain.Publisher) error {
	if err := r.find(pub); err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(&pub).Error; err != nil {
		return err
	}

	if pub.ID == uuid.Nil { // fallback for conflict scenario
		return r.find(pub)
	}

	return nil
}

func (r *publisherRepository) find(pub *domain.Publisher) error {
	if pub.ID != uuid.Nil {
		return nil
	}
	if len(pub.Url) > 0 {
		var existing domain.Publisher
		if err := r.db.First(&existing, "url = ?", pub.Url).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	if len(pub.Name) > 0 {
		var existing domain.Publisher
		if err := r.db.First(&existing, "name = ?", pub.Name).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	return errors.New("publisher not found")
}
