package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/handlers/api"
)

// stubFoodService returns a stored food and records what is saved.
type stubFoodService struct {
	domain.FoodService

	stored *domain.Food
	saved  *domain.Food
}

func (s *stubFoodService) ByID(uuid.UUID) (*domain.Food, error) {
	food := *s.stored
	return &food, nil
}

func (s *stubFoodService) Update(food *domain.Food) error {
	s.saved = food
	return nil
}

func TestFoodHandler_UpdateFood_StoredValuesAreNotValidated(t *testing.T) {
	// Imported descriptions can exceed the update limit.
	longDescription := strings.Repeat("x", 2000)
	svc := &stubFoodService{stored: &domain.Food{ID: uuid.New(), Name: "Flour", Description: &longDescription}}
	app, protected := newProtectedApp(t)
	protected.Patch("/food/:id", api.NewFoodHandler(svc).UpdateFood)

	resp := sendJSON(t, app, http.MethodPatch, "/api/v1/food/"+svc.stored.ID.String(), `{"pantry": true}`)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, svc.saved.Pantry)
}

func TestFoodHandler_UpdateFood_DefaultUnit(t *testing.T) {
	storedUnit, newUnit := uuid.New(), uuid.New()

	tests := []struct {
		name     string
		body     string
		wantUnit *uuid.UUID
	}{
		{name: "absent keeps it", body: `{"pantry": true}`, wantUnit: &storedUnit},
		{name: "null clears it", body: `{"default_unit_id": null}`, wantUnit: nil},
		{name: "value replaces it", body: `{"default_unit_id": "` + newUnit.String() + `"}`, wantUnit: &newUnit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubFoodService{stored: &domain.Food{ID: uuid.New(), Name: "Flour", DefaultUnitID: &storedUnit}}
			app, protected := newProtectedApp(t)
			protected.Patch("/food/:id", api.NewFoodHandler(svc).UpdateFood)

			resp := sendJSON(t, app, http.MethodPatch, "/api/v1/food/"+svc.stored.ID.String(), tt.body)

			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.wantUnit, svc.saved.DefaultUnitID)
		})
	}
}
