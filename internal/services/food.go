package services

import (
	"borscht.app/smetana/domain"
)

type FoodService struct {
	repo domain.FoodRepository
}

func NewFoodService(repo domain.FoodRepository) domain.FoodService {
	return &FoodService{repo: repo}
}

func (s *FoodService) FindOrCreate(food *domain.Food) error {
	return s.repo.FindOrCreate(food)
}
