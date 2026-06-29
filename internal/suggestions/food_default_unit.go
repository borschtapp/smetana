package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

// FoodDefaultUnitDetector proposes setting Food.DefaultUnitID to the most-used unit
// across recipe ingredients for foods that currently have no default unit.
// Pure SQL GROUP BY — no external calls, safe to auto-apply above threshold.
type FoodDefaultUnitDetector struct {
	foodRepo domain.FoodRepository
}

func NewFoodDefaultUnitDetector(foodRepo domain.FoodRepository) *FoodDefaultUnitDetector {
	return &FoodDefaultUnitDetector{foodRepo: foodRepo}
}

func (d *FoodDefaultUnitDetector) Type() domain.SuggestionType {
	return domain.SuggestionFoodDefaultUnit
}
func (d *FoodDefaultUnitDetector) Scope() Scope { return ScopeGlobal }

func (d *FoodDefaultUnitDetector) Detect(_ context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	rows, err := d.foodRepo.DefaultUnitUsageSince(since)
	if err != nil {
		return nil, fmt.Errorf("food_default_unit detect: %w", err)
	}

	// A food's per-(food,unit) counts always partition its full ingredient usage, so the
	// total is just their sum — pick the most-used unit per food in the same pass.
	type best struct {
		unitID      uuid.UUID
		usageCount  int
		foodUpdated time.Time
	}
	totalByFood := make(map[uuid.UUID]int)
	byFood := make(map[uuid.UUID]*best)
	for _, r := range rows {
		totalByFood[r.FoodID] += r.Count
		if cur, ok := byFood[r.FoodID]; !ok || r.Count > cur.usageCount {
			byFood[r.FoodID] = &best{
				unitID:      r.UnitID,
				usageCount:  r.Count,
				foodUpdated: r.Updated,
			}
		}
	}

	var suggestions []*domain.Suggestion
	for foodID, b := range byFood {
		total := totalByFood[foodID]
		if total == 0 {
			continue
		}
		confidence := float64(b.usageCount) / float64(total)

		suggestions = append(suggestions, newSuggestion(domain.SuggestionFoodDefaultUnit, "food", foodID, &b.unitID, confidence,
			map[string]any{
				"food_id":      foodID,
				"unit_id":      b.unitID,
				"usage_count":  b.usageCount,
				"food_updated": b.foodUpdated,
			},
			fmt.Sprintf("Unit used in %d of %d ingredient rows", b.usageCount, total),
			fmt.Sprintf("food_default_unit:%s", foodID),
		))
	}
	return suggestions, nil
}

// --- Applier ---

type FoodDefaultUnitApplier struct {
	foodRepo domain.FoodRepository
}

func NewFoodDefaultUnitApplier(foodRepo domain.FoodRepository) *FoodDefaultUnitApplier {
	return &FoodDefaultUnitApplier{foodRepo: foodRepo}
}

func (a *FoodDefaultUnitApplier) Type() domain.SuggestionType {
	return domain.SuggestionFoodDefaultUnit
}

func (a *FoodDefaultUnitApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	var payload struct {
		FoodID      uuid.UUID `json:"food_id"`
		UnitID      uuid.UUID `json:"unit_id"`
		FoodUpdated time.Time `json:"food_updated"`
	}
	if err := json.Unmarshal(s.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	food, err := a.foodRepo.ByID(payload.FoodID)
	if err != nil {
		return err
	}

	// The food may have changed since detection (someone else set a default unit,
	// or edited the row for an unrelated reason) — re-validate before overwriting.
	if food.DefaultUnitID != nil || !food.Updated.Equal(payload.FoodUpdated) {
		return fmt.Errorf("food_default_unit apply %s: food changed since suggestion was generated", food.ID)
	}

	food.DefaultUnitID = &payload.UnitID
	return a.foodRepo.Update(food)
}
