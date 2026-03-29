package repositories

import (
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type schedulerRepository struct {
	db *gorm.DB
}

func NewSchedulerRepository(db *gorm.DB) domain.SchedulerRepository {
	return &schedulerRepository{db: db}
}

func (r *schedulerRepository) CreateLog(log *domain.SchedulerLog) error {
	return mapErr(r.db.Create(log).Error)
}

func (r *schedulerRepository) UpdateLog(log *domain.SchedulerLog) error {
	return mapErr(r.db.Model(log).Updates(log).Error)
}
