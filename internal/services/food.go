package services

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
	// A merged-away ID stands for the food it was merged into; if both resolve to one food, nothing is left to do.
	if err := s.Canonicalize(&keepID, &mergeID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	if keepID == mergeID {
		return nil
	}
	combine, err := s.shoppingItemCombiner(keepID, mergeID)
	if err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	if err := s.repo.Merge(keepID, mergeID, combine); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	return nil
}

// shoppingItemCombiner combines a list's items for the two foods the way AddItems does, converting units
// with either food's conversions. Only open items with unconvertible units stay apart.
func (s *foodService) shoppingItemCombiner(keepID, mergeID uuid.UUID) (domain.ShoppingItemCombiner, error) {
	byFood, err := s.repo.ConversionsByFoodIDs([]uuid.UUID{keepID, mergeID})
	if err != nil {
		return nil, err
	}
	conversions := slices.Concat(byFood[keepID], byFood[mergeID])
	converter := s.unitService.NewConverter()
	converter.Expect(conversionUnitIDs(byFood)...)

	return func(target, incoming *domain.ShoppingItem) (bool, error) {
		switch {
		case incoming.IsBought:
			return true, nil // already bought: nothing left to add
		case target.IsBought:
			reopenWith(target, incoming)
			return true, nil
		}
		err := accumulateAmount(target, incoming, conversions, converter)
		if errors.Is(err, sentinels.ErrUnitConversion) {
			return false, nil
		}
		return err == nil, err
	}, nil
}

func (s *foodService) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	if err := s.repo.AddTaxonomy(foodID, taxonomy); err != nil {
		return fmt.Errorf("add taxonomy: %w", err)
	}
	return nil
}

func (s *foodService) Canonicalize(foodIDs ...*uuid.UUID) error {
	if err := canonicalize(s.repo.CanonicalIDs, foodIDs); err != nil {
		return fmt.Errorf("canonicalize foods: %w", err)
	}
	return nil
}

func (s *foodService) Update(food *domain.Food) error {
	if err := s.unitService.Canonicalize(food.DefaultUnitID); err != nil {
		return fmt.Errorf("update: %w", err)
	}
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
	price.FoodID = food.ID
	if err := s.unitService.Canonicalize(&price.UnitID); err != nil {
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
	if err := s.Canonicalize(&foodID); err != nil {
		return nil, 0, fmt.Errorf("list prices: %w", err)
	}
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

// validateConversion checks both units and rewrites them to their canonical IDs.
func (s *foodService) validateConversion(c *domain.FoodUnitConversion) error {
	sourceUnit, err := s.unitService.ByID(c.UnitID)
	if err != nil {
		return sentinels.BadRequest("invalid source unit")
	}
	targetUnit, err := s.unitService.ByID(c.TargetUnitID)
	if err != nil {
		return sentinels.BadRequest("invalid target unit")
	}
	c.UnitID, c.TargetUnitID = sourceUnit.ID, targetUnit.ID
	if c.UnitID == c.TargetUnitID {
		return sentinels.BadRequest("source and target units cannot be the same")
	}
	if !targetUnit.Convertible() {
		return sentinels.BadRequest("target unit must be a convertible unit (e.g. grams, ml)")
	}
	return nil
}

func (s *foodService) CreateConversion(c *domain.FoodUnitConversion) error {
	if err := s.Canonicalize(&c.FoodID); err != nil {
		return fmt.Errorf("create conversion: %w", err)
	}
	if err := s.validateConversion(c); err != nil {
		return err
	}
	if err := s.repo.CreateConversion(c); err != nil {
		return fmt.Errorf("create conversion: %w", err)
	}
	return nil
}

func (s *foodService) ListConversions(foodID uuid.UUID) ([]domain.FoodUnitConversion, error) {
	if err := s.Canonicalize(&foodID); err != nil {
		return nil, fmt.Errorf("list conversions: %w", err)
	}
	return s.repo.ListConversions(foodID)
}

func (s *foodService) UpdateConversion(c *domain.FoodUnitConversion) error {
	if err := s.Canonicalize(&c.FoodID); err != nil {
		return fmt.Errorf("update conversion: %w", err)
	}
	existing, err := s.repo.ConversionByID(c.FoodID, c.ID)
	if err != nil {
		return fmt.Errorf("update conversion (fetch): %w", err)
	}
	// The source unit is immutable; validate the new target against the stored one.
	c.UnitID = existing.UnitID
	if err := s.validateConversion(c); err != nil {
		return err
	}
	if err := s.repo.UpdateConversion(c); err != nil {
		return fmt.Errorf("update conversion: %w", err)
	}
	return nil
}

func (s *foodService) DeleteConversion(foodID, id uuid.UUID) error {
	if err := s.Canonicalize(&foodID); err != nil {
		return fmt.Errorf("delete conversion: %w", err)
	}
	return s.repo.DeleteConversion(foodID, id)
}

func (s *foodService) ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]domain.FoodUnitConversion, error) {
	return s.repo.ConversionsByFoodIDs(foodIDs)
}
