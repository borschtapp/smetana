package repositories

import (
	"errors"

	"borscht.app/smetana/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PublisherRepository struct {
	db *gorm.DB
}

func NewPublisherRepository(db *gorm.DB) *PublisherRepository {
	return &PublisherRepository{db: db}
}

func (r *PublisherRepository) List(offset, limit int) ([]domain.Publisher, int64, error) {
	var total int64
	if err := r.db.Model(&domain.Publisher{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var publishers []domain.Publisher
	if err := r.db.Model(&domain.Publisher{}).Offset(offset).Limit(limit).Find(&publishers).Error; err != nil {
		return nil, 0, err
	}
	return publishers, total, nil
}

func (r *PublisherRepository) FindOrCreate(pub *domain.Publisher) error {
	if err := r.find(pub); err == nil {
		return nil
	}

	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&pub).Error; err != nil {
		return err
	}

	if pub.ID == uuid.Nil { // fallback for conflict scenario
		return r.find(pub)
	}

	return nil
}

func (r *PublisherRepository) find(pub *domain.Publisher) error {
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
