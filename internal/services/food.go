package services

import (
	"borscht.app/smetana/domain"
)

type foodService struct {
	repo domain.FoodRepository
}

func NewFoodService(repo domain.FoodRepository) domain.FoodService {
	return &foodService{repo: repo}
}

func (s *foodService) FindOrCreate(food *domain.Food) error {
	return s.repo.FindOrCreate(food)
}
