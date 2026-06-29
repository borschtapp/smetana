package suggestions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

const foodMergeReviewThreshold = 0.85 // fuzzy match → queue for review

type FoodMergeDetector struct {
	foodRepo domain.FoodRepository
}

func NewFoodMergeDetector(foodRepo domain.FoodRepository) *FoodMergeDetector {
	return &FoodMergeDetector{foodRepo: foodRepo}
}

func (d *FoodMergeDetector) Type() domain.SuggestionType { return domain.SuggestionFoodMerge }
func (d *FoodMergeDetector) Scope() Scope                { return ScopeGlobal }

func (d *FoodMergeDetector) Detect(ctx context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	workSet, catalog, err := d.foodRepo.MergeCandidates(since)
	if err != nil {
		return nil, fmt.Errorf("food_merge detect: %w", err)
	}

	name := func(f domain.Food) string { return f.Name }
	return detectMerges(ctx, workSet, catalog, mergeDetectorConfig[domain.Food]{
		suggType:    domain.SuggestionFoodMerge,
		subjectType: "food",
		threshold:   foodMergeReviewThreshold,

		id:         func(f domain.Food) uuid.UUID { return f.ID },
		name:       name,
		comparable: func(_, _ domain.Food) bool { return true },
		chooseKeep: chooseKeepFood,

		payload: defaultMergePayload(name),
		reason:  defaultMergeReason(name),
	})
}

// chooseKeepFood picks the higher-quality food to retain after a merge.
func chooseKeepFood(a, b domain.Food) (keep, merge uuid.UUID) {
	scoreA, scoreB := 0, 0
	if a.ImagePath != nil {
		scoreA++
	}
	if a.DefaultUnitID != nil {
		scoreA++
	}
	if a.Pantry {
		scoreA++
	}
	if b.ImagePath != nil {
		scoreB++
	}
	if b.DefaultUnitID != nil {
		scoreB++
	}
	if b.Pantry {
		scoreB++
	}
	// Tie-break: prefer older (lower UUID v7 = created earlier).
	if scoreA == scoreB {
		return minUUID(a.ID, b.ID), maxUUID(a.ID, b.ID)
	}
	if scoreA > scoreB {
		return a.ID, b.ID
	}
	return b.ID, a.ID
}

// --- Applier ---

type FoodMergeApplier struct {
	foodService domain.FoodService
}

func NewFoodMergeApplier(foodService domain.FoodService) *FoodMergeApplier {
	return &FoodMergeApplier{foodService: foodService}
}

func (a *FoodMergeApplier) Type() domain.SuggestionType { return domain.SuggestionFoodMerge }

func (a *FoodMergeApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	keepID, mergeID, err := decodeMergePayload(s.Payload)
	if err != nil {
		return err
	}
	return a.foodService.Merge(keepID, mergeID)
}
