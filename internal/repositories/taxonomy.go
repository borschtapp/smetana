package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type taxonomyRepository struct {
	db *gorm.DB
}

func NewTaxonomyRepository(db *gorm.DB) domain.TaxonomyRepository {
	return &taxonomyRepository{db: db}
}

func (r *taxonomyRepository) List(taxonomyType string, offset, limit int) ([]domain.Taxonomy, int64, error) {
	query := r.db.Model(&domain.Taxonomy{})

	if taxonomyType != "" {
		query = query.Where("type = ?", taxonomyType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var taxonomies []domain.Taxonomy
	if err := query.Offset(offset).Limit(limit).Find(&taxonomies).Error; err != nil {
		return nil, 0, err
	}
	return taxonomies, total, nil
}

func (r *taxonomyRepository) FindOrCreate(taxonomy *domain.Taxonomy) error {
	if err := r.db.First(taxonomy, "lower(slug) = lower(?)", taxonomy.Slug).Error; err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(taxonomy).Error; err != nil {
		return err
	}

	if taxonomy.ID == uuid.Nil { // fallback for conflict scenario
		return r.db.First(taxonomy, "lower(slug) = lower(?)", taxonomy.Slug).Error
	}

	return nil
}
