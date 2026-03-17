package services

import (
	"context"

	"borscht.app/smetana/domain"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type foodService struct {
	repo         domain.FoodRepository
	imageService domain.ImageService
}

func NewFoodService(repo domain.FoodRepository, imageService domain.ImageService) domain.FoodService {
	return &foodService{repo: repo, imageService: imageService}
}

func (s *foodService) FindOrCreate(ctx context.Context, food *domain.Food) error {
	if err := s.repo.FindOrCreate(food); err != nil {
		return err
	}

	if food != nil && food.ImagePath == nil && len(food.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, food.Images[0], "food", food.ID, "")
		if err != nil {
			log.Warnw("unable to process food image, skipping", "food_id", food.ID, "image", food.Images[0], "error", err)
		}
		food.ImagePath = path
	}
	return nil
}

func (s *foodService) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	return s.repo.AddTaxonomy(foodID, taxonomy)
}

func (s *foodService) Update(food *domain.Food) error {
	return s.repo.Update(food)
}
