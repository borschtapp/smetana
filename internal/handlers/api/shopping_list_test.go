package api_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/handlers/api"
)

// stubShoppingListService applies updates to a stored item and records the result.
type stubShoppingListService struct {
	domain.ShoppingListService

	stored *domain.ShoppingItem
	saved  *domain.ShoppingItem
}

func (s *stubShoppingListService) UpdateItem(itemID, _, _ uuid.UUID, apply func(*domain.ShoppingItem)) (*domain.ShoppingItem, error) {
	item := *s.stored
	item.ID = itemID
	apply(&item)
	s.saved = &item
	return &item, nil
}

func patchShoppingItem(t *testing.T, svc *stubShoppingListService, body string) *http.Response {
	t.Helper()
	app, protected := newProtectedApp(t)
	protected.Patch("/shoppinglists/:id/items/:itemId", api.NewShoppingListHandler(svc).UpdateShoppingItem)
	return sendJSON(t, app, http.MethodPatch, "/api/v1/shoppinglists/"+uuid.NewString()+"/items/"+uuid.NewString(), body)
}

func storedShoppingItem() *domain.ShoppingItem {
	unitID, foodID := uuid.New(), uuid.New()
	return &domain.ShoppingItem{Text: "milk", Amount: new(2.0), UnitID: &unitID, FoodID: &foodID}
}

func TestShoppingListHandler_UpdateShoppingItem_KeepsFieldsNotSent(t *testing.T) {
	stored := storedShoppingItem()
	svc := &stubShoppingListService{stored: stored}

	resp := patchShoppingItem(t, svc, `{"is_bought": true}`)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, svc.saved)
	assert.True(t, svc.saved.IsBought)
	assert.Equal(t, stored.Text, svc.saved.Text)
	assert.Equal(t, stored.Amount, svc.saved.Amount)
	assert.Equal(t, stored.UnitID, svc.saved.UnitID)
	assert.Equal(t, stored.FoodID, svc.saved.FoodID)

	var got domain.ShoppingItem
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "milk", got.Text, "the response must contain the full saved item")
}

func TestShoppingListHandler_UpdateShoppingItem_StoredValuesAreNotValidated(t *testing.T) {
	// Food-only items are stored with empty text.
	foodID := uuid.New()
	svc := &stubShoppingListService{stored: &domain.ShoppingItem{FoodID: &foodID, Amount: new(0.0)}}

	resp := patchShoppingItem(t, svc, `{"is_bought": true}`)

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, svc.saved.IsBought)
}

func TestShoppingListHandler_UpdateShoppingItem_Validation(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantAmount *float64
	}{
		{name: "absent amount keeps it", body: `{"text": "oat milk"}`, wantStatus: http.StatusOK, wantAmount: new(2.0)},
		{name: "null amount clears it", body: `{"amount": null}`, wantStatus: http.StatusOK, wantAmount: nil},
		{name: "amount value replaces it", body: `{"amount": 3}`, wantStatus: http.StatusOK, wantAmount: new(3.0)},
		{name: "zero amount is rejected", body: `{"amount": 0}`, wantStatus: http.StatusBadRequest},
		{name: "negative amount is rejected", body: `{"amount": -1}`, wantStatus: http.StatusBadRequest},
		{name: "empty text is rejected", body: `{"text": ""}`, wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &stubShoppingListService{stored: storedShoppingItem()}

			resp := patchShoppingItem(t, svc, tt.body)

			require.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantStatus != http.StatusOK {
				assert.Nil(t, svc.saved, "an invalid request must not be saved")
				return
			}
			assert.Equal(t, tt.wantAmount, svc.saved.Amount)
		})
	}
}
