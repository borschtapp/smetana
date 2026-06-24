package services

import (
	"context"
	"fmt"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
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

func (s *foodService) ByID(id uuid.UUID) (*domain.Food, error) {
	food, err := s.repo.ByID(id)
	if err != nil {
		return nil, fmt.Errorf("by id: %w", err)
	}
	return food, nil
}

func (s *foodService) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Food, error) {
	foods, err := s.repo.ByIDs(ids)
	if err != nil {
		return nil, fmt.Errorf("by ids: %w", err)
	}
	return foods, nil
}

func (s *foodService) FindOrCreate(ctx context.Context, food *domain.Food) error {
	if err := s.repo.FindOrCreate(food); err != nil {
		return fmt.Errorf("find or create: %w", err)
	}

	if food != nil && food.ImagePath == nil && len(food.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, food.Images[0], "food", food.ID, "")
		if err != nil {
			log.Warnw("unable to process food image, skipping", "food_id", food.ID, "image", food.Images[0], "error", err.Error())
		}
		food.ImagePath = path
	}
	return nil
}

func (s *foodService) Search(query string, offset, limit int) ([]domain.Food, int64, error) {
	foods, total, err := s.repo.Search(query, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	return foods, total, nil
}

func (s *foodService) Merge(keepID, mergeID uuid.UUID) error {
	if keepID == mergeID {
		return sentinels.BadRequest("cannot merge a food into itself")
	}
	if err := s.repo.Merge(keepID, mergeID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	return nil
}

func (s *foodService) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	if err := s.repo.AddTaxonomy(foodID, taxonomy); err != nil {
		return fmt.Errorf("add taxonomy: %w", err)
	}
	return nil
}

func (s *foodService) Update(food *domain.Food) error {
	if err := s.repo.Update(food); err != nil {
		return fmt.Errorf("update: %w", err)
	}
	return nil
}

func (s *foodService) RecordPrice(householdID uuid.UUID, price *domain.FoodPrice) error {
	price.HouseholdID = householdID
	if err := s.repo.CreatePrice(price); err != nil {
		return fmt.Errorf("record price: %w", err)
	}
	return nil
}

func (s *foodService) LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
	prices, err := s.repo.LatestPrices(householdID, foodIDs)
	if err != nil {
		return nil, fmt.Errorf("latest prices: %w", err)
	}
	return prices, nil
}

func (s *foodService) ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]domain.FoodPrice, int64, error) {
	prices, total, err := s.repo.ListPrices(householdID, foodID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list prices: %w", err)
	}
	return prices, total, nil
}

func (s *foodService) DeletePrice(householdID, id uuid.UUID) error {
	if err := s.repo.DeletePrice(householdID, id); err != nil {
		return fmt.Errorf("delete price: %w", err)
	}
	return nil
}
