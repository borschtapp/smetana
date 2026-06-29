package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

// FoodConversionDetector finds foods where cost estimation fails due to incompatible units
// (ingredient measured in "pieces" but price recorded per "kg"), then proposes a conversion.
type FoodConversionDetector struct {
	unitRepo domain.UnitRepository
}

func NewFoodConversionDetector(unitRepo domain.UnitRepository) *FoodConversionDetector {
	return &FoodConversionDetector{unitRepo: unitRepo}
}

func (d *FoodConversionDetector) Type() domain.SuggestionType { return domain.SuggestionFoodConversion }
func (d *FoodConversionDetector) Scope() Scope                { return ScopeGlobal }

func (d *FoodConversionDetector) Detect(_ context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	// Find foods used with non-convertible units, indicating a dimension mismatch
	// (e.g. piece vs gram) that blocks cost calculation.
	pairs, err := d.unitRepo.NonConvertibleUsageSince(since)
	if err != nil {
		return nil, fmt.Errorf("food_conversion detect: %w", err)
	}

	var suggestions []*domain.Suggestion
	seen := make(map[uuid.UUID]bool)
	for _, p := range pairs {
		if seen[p.FoodID] {
			continue
		}
		seen[p.FoodID] = true

		suggestions = append(suggestions, newSuggestion(domain.SuggestionFoodConversion, "food", p.FoodID, &p.UnitID, 0.7,
			map[string]any{
				"food_id": p.FoodID,
				"unit_id": p.UnitID,
			},
			"Ingredient uses a non-convertible unit; cost calculation requires a weight/volume conversion",
			fmt.Sprintf("food_conversion:%s:%s", p.FoodID, p.UnitID),
		))
	}
	return suggestions, nil
}

// --- Applier ---

type FoodConversionApplier struct {
	foodService domain.FoodService
}

func NewFoodConversionApplier(foodService domain.FoodService) *FoodConversionApplier {
	return &FoodConversionApplier{foodService: foodService}
}

func (a *FoodConversionApplier) Type() domain.SuggestionType { return domain.SuggestionFoodConversion }

func (a *FoodConversionApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	var payload struct {
		FoodID       uuid.UUID `json:"food_id"`
		UnitID       uuid.UUID `json:"unit_id"`
		TargetAmount float64   `json:"target_amount"`
		TargetUnitID uuid.UUID `json:"target_unit_id"`
	}
	if err := json.Unmarshal(s.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	if payload.TargetAmount == 0 || payload.TargetUnitID == uuid.Nil {
		return fmt.Errorf("food_conversion apply: target_amount and target_unit_id must be set before accepting")
	}

	conv := &domain.FoodUnitConversion{
		FoodID:       payload.FoodID,
		UnitID:       payload.UnitID,
		TargetAmount: payload.TargetAmount,
		TargetUnitID: payload.TargetUnitID,
	}
	return a.foodService.CreateConversion(conv)
}
