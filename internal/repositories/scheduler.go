package repositories

import (
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type SchedulerRepository struct {
	db *gorm.DB
}

func NewSchedulerRepository(db *gorm.DB) domain.SchedulerRepository {
	return &SchedulerRepository{db: db}
}

func (r *SchedulerRepository) CreateLog(log *domain.SchedulerLog) error {
	return r.db.Create(log).Error
}

func (r *SchedulerRepository) UpdateLog(log *domain.SchedulerLog) error {
	return r.db.Model(log).Updates(log).Error
}
