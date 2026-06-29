package suggestions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

type Scope int

const (
	ScopeGlobal    Scope = iota
	ScopeHousehold       // detector runs once per household
)

// Detector mines the DB for candidate problems.
// since is the reconciliation watermark; only entities updated after it need examination.
type Detector interface {
	Type() domain.SuggestionType
	Scope() Scope
	Detect(ctx context.Context, since time.Time, householdID *uuid.UUID) ([]*domain.Suggestion, error)
}

// Applier enacts an accepted suggestion. Applying is one-way: there is no undo, because
// the destructive operations (Merge) delete rows that no snapshot captures. Use the
// Detach endpoints to un-alias a merge; other changes are edited through the normal API.
type Applier interface {
	Type() domain.SuggestionType
	Apply(ctx context.Context, s *domain.Suggestion) error
}

type Entry struct {
	Detector Detector
	Applier  Applier
	// AutoApply and Threshold gate the reconcile sweep: when AutoApply is true, a candidate
	// with Confidence >= Threshold is applied immediately instead of queued for review.
	AutoApply bool
	Threshold float64
}

type Registry struct {
	entries []Entry
	// byType indexes entries for Applier lookup, which runs on every accept/reject.
	// entries keeps registration order for sweeps.
	byType map[domain.SuggestionType]Applier
}

func NewRegistry() *Registry {
	return &Registry{byType: make(map[domain.SuggestionType]Applier)}
}

// Register pairs a detector with the applier that enacts its suggestions, for types that
// always need manual review. Passing a nil applier registers a detect-only type, whose
// suggestions can be listed but not applied.
func (r *Registry) Register(d Detector, a Applier) {
	r.add(Entry{Detector: d, Applier: a})
}

// RegisterAuto is Register plus a confidence threshold: matching suggestions apply
// immediately during the reconcile sweep instead of waiting for manual review.
func (r *Registry) RegisterAuto(d Detector, a Applier, threshold float64) {
	r.add(Entry{Detector: d, Applier: a, AutoApply: true, Threshold: threshold})
}

func (r *Registry) add(e Entry) {
	r.entries = append(r.entries, e)
	if e.Applier != nil {
		r.byType[e.Detector.Type()] = e.Applier
	}
}

func (r *Registry) All() []Entry {
	return r.entries
}

func (r *Registry) Applier(t domain.SuggestionType) (Applier, bool) {
	a, ok := r.byType[t]
	return a, ok
}

// CursorKey returns the stable key used in detector_cursors for a detector + optional household.
func CursorKey(detectorType domain.SuggestionType, householdID *uuid.UUID) string {
	if householdID == nil {
		return string(detectorType)
	}
	return string(detectorType) + ":" + householdID.String()
}
