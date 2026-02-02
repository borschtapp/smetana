package services

import (
	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
)

type UnitService struct{}

func NewUnitService() *UnitService {
	return &UnitService{}
}

func (s *UnitService) FindOrCreateUnit(unit *domain.Unit) error {
	if err := database.DB.First(&unit, "name = ?", unit.Name).Error; err == nil {
		return nil
	}

	if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(unit).Error; err != nil {
		return err
	}

	if unit.ID == uuid.Nil { // fallback for conflict scenario
		return database.DB.First(&unit, "name = ?", unit.Name).Error
	}

	return nil
}
