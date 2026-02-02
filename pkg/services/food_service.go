package services

import (
	"github.com/google/uuid"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
)

type FoodService struct{}

func NewFoodService() *FoodService {
	return &FoodService{}
}

func (s *FoodService) FindOrCreateFood(food *domain.Food) error {
	if err := database.DB.First(&food, "name = ?", food.Name).Error; err == nil {
		return nil
	}

	if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(food).Error; err != nil {
		return err
	}

	if food.ID == uuid.Nil { // fallback for conflict scenario
		return database.DB.First(&food, "name = ?", food.Name).Error
	}

	return nil
}
