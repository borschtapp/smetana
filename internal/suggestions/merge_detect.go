package suggestions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

// mergeDetectorConfig parameterizes the near-duplicate scan shared by food_merge, unit_merge,
// and taxonomy_merge: compare a work set of recently-updated, not-yet-merged rows against the
// full not-yet-merged catalog, and propose a merge above a similarity threshold. Loading the
// work set and catalog is the caller's repository's job (see each detector's Detect).
type mergeDetectorConfig[T any] struct {
	suggType    domain.SuggestionType
	subjectType string
	threshold   float64

	id   func(T) uuid.UUID
	name func(T) string
	// comparable reports whether w and c may even be compared (e.g. same imperial flag,
	// same taxonomy type). w/c with c.id == w.id are excluded before this is called.
	comparable func(w, c T) bool
	chooseKeep func(w, c T) (keep, merge uuid.UUID)
	payload    func(catalogByID map[uuid.UUID]T, keepID, mergeID uuid.UUID, sim float64) map[string]any
	reason     func(catalogByID map[uuid.UUID]T, keepID, mergeID uuid.UUID, sim float64) string
}

func detectMerges[T any](ctx context.Context, workSet, catalog []T, cfg mergeDetectorConfig[T]) ([]*domain.Suggestion, error) {
	if len(workSet) == 0 {
		return nil, nil
	}

	catalogByID := make(map[uuid.UUID]T, len(catalog))
	catalogKeys := make(map[uuid.UUID]nameKey, len(catalog))
	for _, c := range catalog {
		cID := cfg.id(c)
		catalogByID[cID] = c
		catalogKeys[cID] = buildNameKey(cfg.name(c))
	}

	var suggestions []*domain.Suggestion
	seen := make(map[string]bool)

	for _, w := range workSet {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		wID := cfg.id(w)
		wKey := buildNameKey(cfg.name(w))
		for _, c := range catalog {
			cID := cfg.id(c)
			if cID == wID || !cfg.comparable(w, c) {
				continue
			}
			sim := nameKeySimilarity(wKey, catalogKeys[cID])
			if sim < cfg.threshold {
				continue
			}

			// Deterministic dedup: always order UUIDs so A↔B and B↔A produce the same key.
			lo, hi := minUUID(wID, cID), maxUUID(wID, cID)
			key := fmt.Sprintf("%s:%s:%s", cfg.suggType, lo, hi)
			if seen[key] {
				continue
			}
			seen[key] = true

			keepID, mergeID := cfg.chooseKeep(w, c)

			suggestions = append(suggestions, newSuggestion(cfg.suggType, cfg.subjectType, mergeID, &keepID, sim,
				cfg.payload(catalogByID, keepID, mergeID, sim), cfg.reason(catalogByID, keepID, mergeID, sim), key))
		}
	}
	return suggestions, nil
}

// newSuggestion builds the pending, algorithm-sourced suggestion shape every detector
// (merge and non-merge alike) hands off to the review queue.
func newSuggestion(t domain.SuggestionType, subjectType string, subjectID uuid.UUID, targetID *uuid.UUID, confidence float64, payload map[string]any, reason, dedupKey string) *domain.Suggestion {
	p, _ := json.Marshal(payload)
	return &domain.Suggestion{
		Type:        t,
		Status:      domain.StatusPending,
		Source:      "algorithm",
		Confidence:  confidence,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		TargetID:    targetID,
		Payload:     p,
		Reason:      reason,
		DedupKey:    dedupKey,
	}
}

// defaultMergePayload/defaultMergeReason build the {keep_id, merge_id, keep_name,
// merge_name, similarity} shape shared by every merge detector except taxonomy, which
// adds a type field and uses its own label wording.
func defaultMergePayload[T any](name func(T) string) func(map[uuid.UUID]T, uuid.UUID, uuid.UUID, float64) map[string]any {
	return func(catalog map[uuid.UUID]T, keepID, mergeID uuid.UUID, sim float64) map[string]any {
		return map[string]any{
			"keep_id":    keepID,
			"merge_id":   mergeID,
			"keep_name":  name(catalog[keepID]),
			"merge_name": name(catalog[mergeID]),
			"similarity": sim,
		}
	}
}

func defaultMergeReason[T any](name func(T) string) func(map[uuid.UUID]T, uuid.UUID, uuid.UUID, float64) string {
	return func(catalog map[uuid.UUID]T, keepID, mergeID uuid.UUID, sim float64) string {
		return fmt.Sprintf("%.0f%% name similarity between %q and %q", sim*100, name(catalog[mergeID]), name(catalog[keepID]))
	}
}

// decodeMergePayload extracts the keep/merge UUID pair shared by the food/unit/taxonomy
// merge appliers' payloads.
func decodeMergePayload(payload []byte) (keep, merge uuid.UUID, err error) {
	var p struct {
		KeepID  uuid.UUID `json:"keep_id"`
		MergeID uuid.UUID `json:"merge_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return uuid.Nil, uuid.Nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return p.KeepID, p.MergeID, nil
}

func minUUID(a, b uuid.UUID) uuid.UUID {
	if a.String() <= b.String() {
		return a
	}
	return b
}

func maxUUID(a, b uuid.UUID) uuid.UUID {
	if a.String() >= b.String() {
		return a
	}
	return b
}
