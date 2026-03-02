package repositories

import (
	"borscht.app/smetana/domain"
	"gorm.io/gorm"
)

type TaxonomyRepository struct {
	db *gorm.DB
}

func NewTaxonomyRepository(db *gorm.DB) *TaxonomyRepository {
	return &TaxonomyRepository{db: db}
}

func (r *TaxonomyRepository) List(taxonomyType string, offset, limit int) ([]domain.Taxonomy, int64, error) {
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
