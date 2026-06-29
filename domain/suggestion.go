package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SuggestionType string

const (
	SuggestionFoodMerge       SuggestionType = "food_merge"
	SuggestionUnitMerge       SuggestionType = "unit_merge"
	SuggestionTaxonomyMerge   SuggestionType = "taxonomy_merge"
	SuggestionFoodDefaultUnit SuggestionType = "food_default_unit"
	SuggestionFoodConversion  SuggestionType = "food_conversion"
	SuggestionIngredientFood  SuggestionType = "ingredient_food_link"
)

type SuggestionStatus string

const (
	StatusPending     SuggestionStatus = "pending"
	StatusAccepted    SuggestionStatus = "accepted"
	StatusAutoApplied SuggestionStatus = "auto_applied"
	StatusRejected    SuggestionStatus = "rejected"
	StatusFailed      SuggestionStatus = "failed"
)

type Suggestion struct {
	ID          uuid.UUID        `gorm:"type:char(36);primaryKey" json:"id"`
	Type        SuggestionType   `gorm:"index" json:"type"`
	Status      SuggestionStatus `gorm:"index;default:'pending'" json:"status"`
	Source      string           `json:"source"`     // "algorithm" | "llm" | "external"
	Confidence  float64          `json:"confidence"` // 0..1
	HouseholdID *uuid.UUID       `gorm:"type:char(36);index" json:"household_id,omitempty"`

	// Subject columns: indexed so we can list "suggestions about food X" without parsing JSON.
	SubjectType string     `gorm:"index:idx_sugg_subject" json:"subject_type"` // food|unit|taxonomy|ingredient|recipe
	SubjectID   uuid.UUID  `gorm:"type:char(36);index:idx_sugg_subject" json:"subject_id"`
	TargetID    *uuid.UUID `gorm:"type:char(36);index" json:"target_id,omitempty"` // "keep" entity in a merge

	Payload  types.JSON `gorm:"type:text" json:"payload"` // type-specific proposal
	Reason   string     `json:"reason"`
	DedupKey string     `gorm:"uniqueIndex" json:"-"` // deterministic; prevents re-suggesting

	ResolvedBy *uuid.UUID `gorm:"type:char(36)" json:"resolved_by,omitempty"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	Error      *string    `json:"error,omitempty"`
	Updated    time.Time  `gorm:"autoUpdateTime" json:"-"`
	Created    time.Time  `gorm:"autoCreateTime" json:"created"`
}

func (s *Suggestion) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		var err error
		s.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

// DetectorCursor tracks the watermark for each detector.
// Key encodes both detector type and household: "food_merge" or "food_price:<householdID>".
type DetectorCursor struct {
	Key       string `gorm:"primaryKey"`
	Watermark time.Time
}

type SuggestionFilter struct {
	Type        SuggestionType
	Status      SuggestionStatus
	SubjectType string
	SubjectID   *uuid.UUID
	HouseholdID *uuid.UUID // when set, includes matching household rows + all global rows
	// Scope narrows by visibility: "global" (household_id IS NULL), "household" (matches
	// HouseholdID only), or "all" (default — today's global-OR-household behavior).
	Scope string
}

type SuggestionStat struct {
	Type   SuggestionType   `json:"type"`
	Status SuggestionStatus `json:"status"`
	Count  int64            `json:"count"`
}

// SuggestionView pairs a suggestion with hydrated display names for the subject and target.
type SuggestionView struct {
	Suggestion
	View map[string]any `json:"view,omitempty"`
}

type SuggestionRepository interface {
	ByID(id uuid.UUID) (*Suggestion, error)
	// UpsertByDedupKey inserts if not exists; resets failed→pending; skips all other existing rows.
	UpsertByDedupKey(s *Suggestion) error
	List(filter SuggestionFilter, p types.Pagination) ([]Suggestion, int64, error)
	// Stats counts by type+status over the rows visible to householdID: its own rows plus
	// all global ones. A nil householdID restricts the count to global rows.
	Stats(householdID *uuid.UUID) ([]SuggestionStat, error)
	Update(s *Suggestion) error
	Cursor(key string) (time.Time, error)
	AdvanceCursor(key string, watermark time.Time) error
}

type SuggestionService interface {
	List(filter SuggestionFilter, p types.Pagination) ([]SuggestionView, int64, error)
	Get(id uuid.UUID) (*SuggestionView, error)
	Stats(householdID *uuid.UUID) ([]SuggestionStat, error)
	Accept(ctx context.Context, id, userID uuid.UUID) error
	Reject(ctx context.Context, id, userID uuid.UUID) error
	Reconcile(ctx context.Context, householdID *uuid.UUID, filterTypes ...SuggestionType) error
}
