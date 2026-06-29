package suggestions

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

const taxonomyMergeReviewThreshold = 0.88

type TaxonomyMergeDetector struct {
	taxonomyRepo domain.TaxonomyRepository
}

func NewTaxonomyMergeDetector(taxonomyRepo domain.TaxonomyRepository) *TaxonomyMergeDetector {
	return &TaxonomyMergeDetector{taxonomyRepo: taxonomyRepo}
}

func (d *TaxonomyMergeDetector) Type() domain.SuggestionType { return domain.SuggestionTaxonomyMerge }
func (d *TaxonomyMergeDetector) Scope() Scope                { return ScopeGlobal }

func (d *TaxonomyMergeDetector) Detect(ctx context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	workSet, catalog, err := d.taxonomyRepo.MergeCandidates(since)
	if err != nil {
		return nil, fmt.Errorf("taxonomy_merge detect: %w", err)
	}

	return detectMerges(ctx, workSet, catalog, mergeDetectorConfig[domain.Taxonomy]{
		suggType:    domain.SuggestionTaxonomyMerge,
		subjectType: "taxonomy",
		threshold:   taxonomyMergeReviewThreshold,

		id:   func(t domain.Taxonomy) uuid.UUID { return t.ID },
		name: func(t domain.Taxonomy) string { return t.Label },
		// Only merge taxonomies of the same kind (e.g. never a "cuisine" into a "diet").
		comparable: func(w, c domain.Taxonomy) bool { return w.Type == c.Type },
		chooseKeep: chooseKeepTaxonomy,

		payload: func(catalog map[uuid.UUID]domain.Taxonomy, keepID, mergeID uuid.UUID, sim float64) map[string]any {
			return map[string]any{
				"keep_id":     keepID,
				"merge_id":    mergeID,
				"keep_label":  catalog[keepID].Label,
				"merge_label": catalog[mergeID].Label,
				"type":        catalog[mergeID].Type,
				"similarity":  sim,
			}
		},
		reason: func(catalog map[uuid.UUID]domain.Taxonomy, keepID, mergeID uuid.UUID, sim float64) string {
			return fmt.Sprintf("%.0f%% label similarity between %q and %q (type: %s)",
				sim*100, catalog[mergeID].Label, catalog[keepID].Label, catalog[mergeID].Type)
		},
	})
}

// chooseKeepTaxonomy keeps whichever taxonomy was created first (lower UUID v7), the same
// tie-break food/unit merge fall back to when there's no stronger quality signal to prefer.
func chooseKeepTaxonomy(a, b domain.Taxonomy) (keep, merge uuid.UUID) {
	return minUUID(a.ID, b.ID), maxUUID(a.ID, b.ID)
}

// --- Applier ---

type TaxonomyMergeApplier struct {
	taxonomyService domain.TaxonomyService
}

func NewTaxonomyMergeApplier(taxonomyService domain.TaxonomyService) *TaxonomyMergeApplier {
	return &TaxonomyMergeApplier{taxonomyService: taxonomyService}
}

func (a *TaxonomyMergeApplier) Type() domain.SuggestionType { return domain.SuggestionTaxonomyMerge }

func (a *TaxonomyMergeApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	keepID, mergeID, err := decodeMergePayload(s.Payload)
	if err != nil {
		return err
	}
	return a.taxonomyService.Merge(keepID, mergeID)
}
