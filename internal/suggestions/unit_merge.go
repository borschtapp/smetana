package suggestions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

const unitMergeReviewThreshold = 0.88 // fuzzy match → queue for review

type UnitMergeDetector struct {
	unitRepo domain.UnitRepository
}

func NewUnitMergeDetector(unitRepo domain.UnitRepository) *UnitMergeDetector {
	return &UnitMergeDetector{unitRepo: unitRepo}
}

func (d *UnitMergeDetector) Type() domain.SuggestionType { return domain.SuggestionUnitMerge }
func (d *UnitMergeDetector) Scope() Scope                { return ScopeGlobal }

func (d *UnitMergeDetector) Detect(ctx context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	workSet, catalog, err := d.unitRepo.MergeCandidates(since)
	if err != nil {
		return nil, fmt.Errorf("unit_merge detect: %w", err)
	}

	name := func(u domain.Unit) string { return u.Name }
	return detectMerges(ctx, workSet, catalog, mergeDetectorConfig[domain.Unit]{
		suggType:    domain.SuggestionUnitMerge,
		subjectType: "unit",
		threshold:   unitMergeReviewThreshold,

		id:   func(u domain.Unit) uuid.UUID { return u.ID },
		name: name,
		// Only compare units in the same measurement system.
		comparable: func(w, c domain.Unit) bool { return w.Imperial == c.Imperial },
		chooseKeep: chooseKeepUnit,

		payload: defaultMergePayload(name),
		reason:  defaultMergeReason(name),
	})
}

// chooseKeepUnit picks the better-quality unit: prefer convertible, then older.
func chooseKeepUnit(a, b domain.Unit) (keep, merge uuid.UUID) {
	scoreA, scoreB := 0, 0
	if a.Convertible() {
		scoreA++
	}
	if b.Convertible() {
		scoreB++
	}
	if scoreA >= scoreB {
		return a.ID, b.ID
	}
	return b.ID, a.ID
}

// --- Applier ---

type UnitMergeApplier struct {
	unitService domain.UnitService
}

func NewUnitMergeApplier(unitService domain.UnitService) *UnitMergeApplier {
	return &UnitMergeApplier{unitService: unitService}
}

func (a *UnitMergeApplier) Type() domain.SuggestionType { return domain.SuggestionUnitMerge }

func (a *UnitMergeApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	keepID, mergeID, err := decodeMergePayload(s.Payload)
	if err != nil {
		return err
	}
	return a.unitService.Merge(keepID, mergeID)
}
