package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/services"
	"borscht.app/smetana/internal/suggestions"
)

const testSuggestionType domain.SuggestionType = "test_type"

type stubSuggestionRepo struct {
	domain.SuggestionRepository
	byID    map[uuid.UUID]*domain.Suggestion
	cursors map[string]time.Time
}

func newStubSuggestionRepo(rows ...*domain.Suggestion) *stubSuggestionRepo {
	r := &stubSuggestionRepo{byID: make(map[uuid.UUID]*domain.Suggestion), cursors: make(map[string]time.Time)}
	for _, s := range rows {
		r.byID[s.ID] = s
	}
	return r
}

func (r *stubSuggestionRepo) ByID(id uuid.UUID) (*domain.Suggestion, error) {
	s, ok := r.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *s
	return &cp, nil
}

func (r *stubSuggestionRepo) Update(s *domain.Suggestion) error {
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

func (r *stubSuggestionRepo) UpsertByDedupKey(s *domain.Suggestion) error {
	for _, existing := range r.byID {
		if existing.DedupKey == s.DedupKey {
			s.ID = existing.ID
			s.Status = existing.Status
			return nil
		}
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

func (r *stubSuggestionRepo) Cursor(key string) (time.Time, error) {
	return r.cursors[key], nil
}

func (r *stubSuggestionRepo) AdvanceCursor(key string, watermark time.Time) error {
	r.cursors[key] = watermark
	return nil
}

type fakeDetector struct {
	typ    domain.SuggestionType
	scope  suggestions.Scope
	result []*domain.Suggestion
}

func (d *fakeDetector) Type() domain.SuggestionType { return d.typ }
func (d *fakeDetector) Scope() suggestions.Scope    { return d.scope }
func (d *fakeDetector) Detect(_ context.Context, _ time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	return d.result, nil
}

type fakeApplier struct {
	typ        domain.SuggestionType
	applyFn    func(*domain.Suggestion) error
	applyCalls int
}

func (a *fakeApplier) Type() domain.SuggestionType { return a.typ }
func (a *fakeApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	a.applyCalls++
	if a.applyFn != nil {
		return a.applyFn(s)
	}
	return nil
}

func newTestSuggestionService(repo domain.SuggestionRepository, registry *suggestions.Registry) domain.SuggestionService {
	return services.NewSuggestionService(repo, &stubFoodRepo{}, &fakeUnitRepo{units: map[uuid.UUID]*domain.Unit{}}, &stubTaxonomyRepo{}, registry)
}

type stubFoodRepo struct{ domain.FoodRepository }

func (s *stubFoodRepo) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Food, error) {
	return make(map[uuid.UUID]*domain.Food), nil
}

func TestSuggestionService_Accept_RoutesToApplierAndAccepts(t *testing.T) {
	sg := &domain.Suggestion{ID: uuid.New(), Type: testSuggestionType, Status: domain.StatusPending}
	repo := newStubSuggestionRepo(sg)
	applier := &fakeApplier{typ: testSuggestionType} // manual review only
	registry := suggestions.NewRegistry()
	registry.Register(&fakeDetector{typ: testSuggestionType}, applier)
	svc := newTestSuggestionService(repo, registry)

	userID := uuid.New()
	require.NoError(t, svc.Accept(context.Background(), sg.ID, userID))

	assert.Equal(t, 1, applier.applyCalls)
	stored := repo.byID[sg.ID]
	assert.Equal(t, domain.StatusAccepted, stored.Status)
	assert.Equal(t, &userID, stored.ResolvedBy)
	assert.NotNil(t, stored.ResolvedAt)
}

func TestSuggestionService_Accept_RejectsWhenNotPendingOrFailed(t *testing.T) {
	sg := &domain.Suggestion{ID: uuid.New(), Type: testSuggestionType, Status: domain.StatusAccepted}
	repo := newStubSuggestionRepo(sg)
	svc := newTestSuggestionService(repo, suggestions.NewRegistry())

	err := svc.Accept(context.Background(), sg.ID, uuid.New())
	require.Error(t, err)
}

func TestSuggestionService_Reject_SetsRejected(t *testing.T) {
	sg := &domain.Suggestion{ID: uuid.New(), Type: testSuggestionType, Status: domain.StatusPending}
	repo := newStubSuggestionRepo(sg)
	svc := newTestSuggestionService(repo, suggestions.NewRegistry())

	userID := uuid.New()
	require.NoError(t, svc.Reject(context.Background(), sg.ID, userID))

	stored := repo.byID[sg.ID]
	assert.Equal(t, domain.StatusRejected, stored.Status)
	assert.Equal(t, &userID, stored.ResolvedBy)
}

func TestSuggestionService_Reconcile_AutoAppliesAboveThreshold(t *testing.T) {
	candidate := &domain.Suggestion{
		Type: testSuggestionType, Status: domain.StatusPending,
		Confidence: 0.95, DedupKey: "test:1",
	}
	repo := newStubSuggestionRepo()
	applier := &fakeApplier{typ: testSuggestionType}
	registry := suggestions.NewRegistry()
	registry.RegisterAuto(&fakeDetector{typ: testSuggestionType, scope: suggestions.ScopeGlobal, result: []*domain.Suggestion{candidate}}, applier, 0.9)
	svc := newTestSuggestionService(repo, registry)

	require.NoError(t, svc.Reconcile(context.Background(), nil))

	require.Equal(t, 1, applier.applyCalls)
	require.Len(t, repo.byID, 1)
	for _, stored := range repo.byID {
		assert.Equal(t, domain.StatusAutoApplied, stored.Status)
	}
}

func TestSuggestionService_Reconcile_BelowThresholdStaysPending(t *testing.T) {
	candidate := &domain.Suggestion{
		Type: testSuggestionType, Status: domain.StatusPending,
		Confidence: 0.5, DedupKey: "test:2",
	}
	repo := newStubSuggestionRepo()
	applier := &fakeApplier{typ: testSuggestionType}
	registry := suggestions.NewRegistry()
	registry.RegisterAuto(&fakeDetector{typ: testSuggestionType, scope: suggestions.ScopeGlobal, result: []*domain.Suggestion{candidate}}, applier, 0.9)
	svc := newTestSuggestionService(repo, registry)

	require.NoError(t, svc.Reconcile(context.Background(), nil))

	assert.Equal(t, 0, applier.applyCalls)
	require.Len(t, repo.byID, 1)
	for _, stored := range repo.byID {
		assert.Equal(t, domain.StatusPending, stored.Status)
	}
}

func TestSuggestionService_Reconcile_AutoApplyFailureSetsFailedStatus(t *testing.T) {
	candidate := &domain.Suggestion{
		Type: testSuggestionType, Status: domain.StatusPending,
		Confidence: 0.95, DedupKey: "test:3",
	}
	repo := newStubSuggestionRepo()
	applier := &fakeApplier{
		typ: testSuggestionType,
		applyFn: func(s *domain.Suggestion) error {
			return errors.New("subject changed since suggestion was generated")
		},
	}
	registry := suggestions.NewRegistry()
	registry.RegisterAuto(&fakeDetector{typ: testSuggestionType, scope: suggestions.ScopeGlobal, result: []*domain.Suggestion{candidate}}, applier, 0.9)
	svc := newTestSuggestionService(repo, registry)

	require.NoError(t, svc.Reconcile(context.Background(), nil))

	require.Len(t, repo.byID, 1)
	for _, stored := range repo.byID {
		assert.Equal(t, domain.StatusFailed, stored.Status)
		require.NotNil(t, stored.Error)
		assert.Contains(t, *stored.Error, "subject changed")
	}
}
