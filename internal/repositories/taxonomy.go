package repositories

import (
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
		return nil, 0, mapErr(err)
	}

	var taxonomies []domain.Taxonomy
	if err := query.Offset(offset).Limit(limit).Find(&taxonomies).Error; err != nil {
		return nil, 0, mapErr(err)
	}
	return taxonomies, total, nil
}

func (r *taxonomyRepository) FindOrCreate(taxonomy *domain.Taxonomy) error {
	if err := r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(taxonomy)
	if result.Error != nil {
		return mapErr(result.Error)
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		return mapErr(r.db.First(taxonomy, "slug = ?", taxonomy.Slug).Error)
	}

	return nil
}
