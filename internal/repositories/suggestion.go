package repositories

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type suggestionRepository struct {
	db *gorm.DB
}

func NewSuggestionRepository(db *gorm.DB) domain.SuggestionRepository {
	return &suggestionRepository{db: db}
}

func (r *suggestionRepository) ByID(id uuid.UUID) (*domain.Suggestion, error) {
	var s domain.Suggestion
	if err := r.db.First(&s, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("suggestion by id %s: %w", id, mapErr(err))
	}
	return &s, nil
}

// UpsertByDedupKey inserts new, resets failed→pending, skips everything else.
func (r *suggestionRepository) UpsertByDedupKey(s *domain.Suggestion) error {
	var existing domain.Suggestion
	err := r.db.Where("dedup_key = ?", s.DedupKey).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err2 := r.db.Create(s).Error; err2 != nil {
			return fmt.Errorf("create suggestion: %w", mapErr(err2))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup suggestion by dedup_key: %w", mapErr(err))
	}

	// Propagate the existing ID and status so the caller sees the DB state.
	s.ID = existing.ID
	s.Status = existing.Status

	if existing.Status != domain.StatusFailed {
		return nil // already handled; skip
	}

	// Reset failed → pending so the next apply attempt can retry.
	updates := map[string]any{
		"status":     domain.StatusPending,
		"confidence": s.Confidence,
		"payload":    s.Payload,
		"reason":     s.Reason,
		"error":      nil,
	}
	if err := r.db.Model(&existing).Updates(updates).Error; err != nil {
		return fmt.Errorf("reset failed suggestion: %w", mapErr(err))
	}
	return nil
}

func (r *suggestionRepository) List(filter domain.SuggestionFilter, p types.Pagination) ([]domain.Suggestion, int64, error) {
	q := r.db.Model(&domain.Suggestion{})

	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.SubjectType != "" {
		q = q.Where("subject_type = ?", filter.SubjectType)
	}
	if filter.SubjectID != nil {
		q = q.Where("subject_id = ?", *filter.SubjectID)
	}
	q = applyVisibility(q, filter.Scope, filter.HouseholdID)

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count suggestions: %w", mapErr(err))
	}

	var suggestions []domain.Suggestion
	if err := q.Order("created DESC").Offset(p.Offset).Limit(p.Limit).Find(&suggestions).Error; err != nil {
		return nil, 0, fmt.Errorf("list suggestions: %w", mapErr(err))
	}
	return suggestions, total, nil
}

func (r *suggestionRepository) Stats(householdID *uuid.UUID) ([]domain.SuggestionStat, error) {
	// "all" scope mirrors List's default visibility rule: own household's rows plus all global ones.
	q := applyVisibility(r.db.Model(&domain.Suggestion{}), "all", householdID)

	var stats []domain.SuggestionStat
	err := q.Select("type, status, count(*) as count").
		Group("type, status").
		Find(&stats).Error
	if err != nil {
		return nil, fmt.Errorf("suggestion stats: %w", mapErr(err))
	}
	return stats, nil
}

func (r *suggestionRepository) Update(s *domain.Suggestion) error {
	cols := []string{"status", "reason", "error", "resolved_by", "resolved_at"}
	if err := r.db.Model(s).Select(cols).Updates(s).Error; err != nil {
		return fmt.Errorf("update suggestion %s: %w", s.ID, mapErr(err))
	}
	return nil
}

// applyVisibility narrows q to the rows visible under scope: "global" restricts to rows with
// no household (household_id IS NULL); "household" restricts to householdID's own rows (none,
// if householdID is nil); anything else ("all", or unset) is the default — householdID's own
// rows plus all global ones.
func applyVisibility(q *gorm.DB, scope string, householdID *uuid.UUID) *gorm.DB {
	switch scope {
	case "global":
		return q.Where("household_id IS NULL")
	case "household":
		if householdID != nil {
			return q.Where("household_id = ?", *householdID)
		}
		return q.Where("1 = 0") // no household in context: nothing household-scoped is visible
	default:
		if householdID != nil {
			return q.Where("household_id = ? OR household_id IS NULL", *householdID)
		}
		return q.Where("household_id IS NULL")
	}
}

func (r *suggestionRepository) Cursor(key string) (time.Time, error) {
	var cursor domain.DetectorCursor
	err := r.db.First(&cursor, "key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, nil // zero = full backfill on first run
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("read cursor %s: %w", key, mapErr(err))
	}
	return cursor.Watermark, nil
}

func (r *suggestionRepository) AdvanceCursor(key string, watermark time.Time) error {
	cursor := domain.DetectorCursor{Key: key, Watermark: watermark}
	err := r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"watermark"}),
	}).Create(&cursor).Error
	if err != nil {
		return fmt.Errorf("advance cursor %s: %w", key, mapErr(err))
	}
	return nil
}
