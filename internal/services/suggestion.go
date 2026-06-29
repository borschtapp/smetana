package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/suggestions"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type suggestionService struct {
	repo         domain.SuggestionRepository
	foodRepo     domain.FoodRepository
	unitRepo     domain.UnitRepository
	taxonomyRepo domain.TaxonomyRepository
	registry     *suggestions.Registry
}

func NewSuggestionService(
	repo domain.SuggestionRepository,
	foodRepo domain.FoodRepository,
	unitRepo domain.UnitRepository,
	taxonomyRepo domain.TaxonomyRepository,
	registry *suggestions.Registry,
) domain.SuggestionService {
	return &suggestionService{
		repo:         repo,
		foodRepo:     foodRepo,
		unitRepo:     unitRepo,
		taxonomyRepo: taxonomyRepo,
		registry:     registry,
	}
}

func (s *suggestionService) List(filter domain.SuggestionFilter, p types.Pagination) ([]domain.SuggestionView, int64, error) {
	rows, total, err := s.repo.List(filter, p)
	if err != nil {
		return nil, 0, err
	}
	views, err := s.hydrateViews(rows)
	if err != nil {
		return nil, 0, err
	}
	return views, total, nil
}

func (s *suggestionService) Get(id uuid.UUID) (*domain.SuggestionView, error) {
	sg, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	views, err := s.hydrateViews([]domain.Suggestion{*sg})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *suggestionService) Stats(householdID *uuid.UUID) ([]domain.SuggestionStat, error) {
	return s.repo.Stats(householdID)
}

func (s *suggestionService) Accept(ctx context.Context, id, userID uuid.UUID) error {
	sg, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if sg.Status != domain.StatusPending && sg.Status != domain.StatusFailed {
		return sentinels.BadRequest(fmt.Sprintf("suggestion is already %s", sg.Status))
	}
	return s.apply(ctx, sg, userID, domain.StatusAccepted)
}

func (s *suggestionService) Reject(ctx context.Context, id, userID uuid.UUID) error {
	sg, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if sg.Status != domain.StatusPending {
		return sentinels.BadRequest(fmt.Sprintf("suggestion is already %s", sg.Status))
	}

	now := time.Now()
	sg.Status = domain.StatusRejected
	sg.ResolvedBy = &userID
	sg.ResolvedAt = &now
	return s.repo.Update(sg)
}

func (s *suggestionService) Reconcile(ctx context.Context, householdID *uuid.UUID, filterTypes ...domain.SuggestionType) error {
	typeSet := make(map[domain.SuggestionType]bool, len(filterTypes))
	for _, t := range filterTypes {
		typeSet[t] = true
	}

	for _, entry := range s.registry.All() {
		if len(typeSet) > 0 && !typeSet[entry.Detector.Type()] {
			continue
		}
		// Scope gating: global detectors only run when householdID is nil.
		if entry.Detector.Scope() == suggestions.ScopeGlobal && householdID != nil {
			continue
		}
		if entry.Detector.Scope() == suggestions.ScopeHousehold && householdID == nil {
			continue
		}

		if err := s.runDetector(ctx, entry, householdID); err != nil {
			log.Warnw("detector run failed", "type", entry.Detector.Type(), "error", err)
			// Continue with remaining detectors rather than aborting the whole sweep.
		}
	}
	return nil
}

func (s *suggestionService) runDetector(ctx context.Context, entry suggestions.Entry, householdID *uuid.UUID) error {
	cursorKey := suggestions.CursorKey(entry.Detector.Type(), householdID)
	watermark, err := s.repo.Cursor(cursorKey)
	if err != nil {
		return fmt.Errorf("read cursor: %w", err)
	}

	// Capture time before scanning so entities written during the scan are included next sweep.
	scanTime := time.Now()

	candidates, err := entry.Detector.Detect(ctx, watermark, householdID)
	if err != nil {
		return fmt.Errorf("detect: %w", err)
	}

	for _, c := range candidates {
		if err := s.repo.UpsertByDedupKey(c); err != nil {
			return fmt.Errorf("upsert suggestion: %w", err) // don't advance cursor on partial failure
		}
		// Auto-apply if the type opts in and the confidence threshold is met.
		if entry.Applier != nil && entry.AutoApply && c.Confidence >= entry.Threshold && c.Status == domain.StatusPending {
			if err := s.apply(ctx, c, uuid.Nil, domain.StatusAutoApplied); err != nil {
				log.Warnw("auto-apply failed", "suggestion", c.ID, "type", c.Type, "error", err)
			}
		}
	}

	return s.repo.AdvanceCursor(cursorKey, scanTime)
}

func (s *suggestionService) apply(ctx context.Context, sg *domain.Suggestion, userID uuid.UUID, newStatus domain.SuggestionStatus) error {
	applier, ok := s.registry.Applier(sg.Type)
	if !ok {
		return sentinels.BadRequest(fmt.Sprintf("no applier registered for type %s", sg.Type))
	}

	err := applier.Apply(ctx, sg)
	now := time.Now()

	if err != nil {
		errMsg := err.Error()
		sg.Status = domain.StatusFailed
		sg.Error = &errMsg
		_ = s.repo.Update(sg)
		return fmt.Errorf("apply suggestion %s: %w", sg.ID, err)
	}

	sg.Status = newStatus
	sg.Error = nil
	if userID != uuid.Nil {
		sg.ResolvedBy = &userID
	}
	sg.ResolvedAt = &now
	return s.repo.Update(sg)
}

// hydrateViews enriches suggestions with display names for their subject/target entities.
func (s *suggestionService) hydrateViews(rows []domain.Suggestion) ([]domain.SuggestionView, error) {
	views := make([]domain.SuggestionView, len(rows))

	// Collect IDs per entity type so we can batch-lookup names. "ingredient" subjects carry
	// their food info directly in the payload, so they're not in this set.
	idSets := map[string]map[uuid.UUID]bool{"food": {}, "unit": {}, "taxonomy": {}}
	for i := range rows {
		sg := &rows[i]
		set, ok := idSets[sg.SubjectType]
		if !ok {
			continue
		}
		set[sg.SubjectID] = true
		if sg.TargetID != nil {
			set[*sg.TargetID] = true
		}
	}

	foodMap, err := s.foodRepo.ByIDs(utils.MapKeys(idSets["food"]))
	if err != nil {
		return nil, fmt.Errorf("hydrate views: foods: %w", err)
	}
	unitMap, err := s.unitRepo.ByIDs(utils.MapKeys(idSets["unit"]))
	if err != nil {
		return nil, fmt.Errorf("hydrate views: units: %w", err)
	}
	taxonomyMap, err := s.taxonomyRepo.ByIDs(utils.MapKeys(idSets["taxonomy"]))
	if err != nil {
		return nil, fmt.Errorf("hydrate views: taxonomies: %w", err)
	}

	for i := range rows {
		sg := &rows[i]
		view := make(map[string]any)

		switch sg.SubjectType {
		case "food":
			if f, ok := foodMap[sg.SubjectID]; ok {
				view["subject"] = map[string]any{"id": f.ID, "name": f.Name}
			}
			if sg.TargetID != nil {
				if f, ok := foodMap[*sg.TargetID]; ok {
					view["target"] = map[string]any{"id": f.ID, "name": f.Name}
				}
			}
		case "unit":
			if u, ok := unitMap[sg.SubjectID]; ok {
				view["subject"] = map[string]any{"id": u.ID, "name": u.Name}
			}
			if sg.TargetID != nil {
				if u, ok := unitMap[*sg.TargetID]; ok {
					view["target"] = map[string]any{"id": u.ID, "name": u.Name}
				}
			}
		case "taxonomy":
			if t, ok := taxonomyMap[sg.SubjectID]; ok {
				view["subject"] = map[string]any{"id": t.ID, "label": t.Label, "type": t.Type}
			}
			if sg.TargetID != nil {
				if t, ok := taxonomyMap[*sg.TargetID]; ok {
					view["target"] = map[string]any{"id": t.ID, "label": t.Label}
				}
			}
		case "ingredient":
			// Payload already carries food_name / ingredient_name for display.
			var payloadData map[string]json.RawMessage
			if err := json.Unmarshal(sg.Payload, &payloadData); err == nil {
				view["subject"] = payloadData
			}
		}

		views[i] = domain.SuggestionView{Suggestion: *sg, View: view}
	}
	return views, nil
}
