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
	unitService  domain.UnitService
}

func NewFoodService(repo domain.FoodRepository, imageService domain.ImageService, unitService domain.UnitService) domain.FoodService {
	return &foodService{repo: repo, imageService: imageService, unitService: unitService}
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

func (s *foodService) Detach(id uuid.UUID) error {
	if err := s.repo.Detach(id); err != nil {
		return fmt.Errorf("detach: %w", err)
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

	food, err := s.repo.ByID(price.FoodID)
	if err != nil {
		return fmt.Errorf("record price: %w", err)
	}

	if err := s.repo.CreatePrice(price); err != nil {
		return fmt.Errorf("record price: %w", err)
	}

	// When the food has no reference unit yet, adopt the unit of this first price
	// observation as its default so later cost and conversion math has a baseline.
	if food.DefaultUnitID == nil {
		food.DefaultUnitID = new(price.UnitID)
		if err := s.repo.Update(food); err != nil {
			return fmt.Errorf("record price: set default unit: %w", err)
		}
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

func (s *foodService) validateConversion(c *domain.FoodUnitConversion) error {
	if c.UnitID == c.TargetUnitID {
		return sentinels.BadRequest("source and target units cannot be the same")
	}
	if _, err := s.unitService.ByID(c.UnitID); err != nil {
		return sentinels.BadRequest("invalid source unit")
	}
	targetUnit, err := s.unitService.ByID(c.TargetUnitID)
	if err != nil {
		return sentinels.BadRequest("invalid target unit")
	}
	if !targetUnit.Convertible() {
		return sentinels.BadRequest("target unit must be a convertible unit (e.g. grams, ml)")
	}
	return nil
}

func (s *foodService) CreateConversion(c *domain.FoodUnitConversion) error {
	if err := s.validateConversion(c); err != nil {
		return err
	}
	if err := s.repo.CreateConversion(c); err != nil {
		return fmt.Errorf("create conversion: %w", err)
	}
	return nil
}

func (s *foodService) ListConversions(foodID uuid.UUID) ([]domain.FoodUnitConversion, error) {
	return s.repo.ListConversions(foodID)
}

func (s *foodService) UpdateConversion(c *domain.FoodUnitConversion) error {
	targetUnit, err := s.unitService.ByID(c.TargetUnitID)
	if err != nil {
		return sentinels.BadRequest("invalid target unit")
	}
	if !targetUnit.Convertible() {
		return sentinels.BadRequest("target unit must be a convertible unit (e.g. grams, ml)")
	}
	if err := s.repo.UpdateConversion(c); err != nil {
		return fmt.Errorf("update conversion: %w", err)
	}
	return nil
}

func (s *foodService) DeleteConversion(foodID, id uuid.UUID) error {
	return s.repo.DeleteConversion(foodID, id)
}

func (s *foodService) ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]domain.FoodUnitConversion, error) {
	return s.repo.ConversionsByFoodIDs(foodIDs)
}
