package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/handlers/api"
	"borscht.app/smetana/internal/middlewares"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"

	"github.com/gofiber/fiber/v3"
)

type stubRecipeService struct {
	domain.RecipeService

	byIDFn             func(uuid.UUID, uuid.UUID) (*domain.Recipe, error)
	byIDPreloadFn      func(uuid.UUID, uuid.UUID, uuid.UUID, types.PreloadOptions) (*domain.Recipe, error)
	searchFn           func(uuid.UUID, uuid.UUID, domain.RecipeSearchOptions) ([]domain.Recipe, int64, error)
	createFn           func(*domain.Recipe, uuid.UUID, uuid.UUID) error
	updateFn           func(*domain.Recipe, uuid.UUID, uuid.UUID) error
	deleteFn           func(uuid.UUID, uuid.UUID) error
	userSaveFn         func(uuid.UUID, uuid.UUID, uuid.UUID) error
	userUnsaveFn       func(uuid.UUID, uuid.UUID) error
	addEquipmentFn     func(uuid.UUID, uuid.UUID, uuid.UUID) error
	removeEquipmentFn  func(uuid.UUID, uuid.UUID, uuid.UUID) error
	createIngredientFn func(*domain.RecipeIngredient, uuid.UUID) error
	deleteIngredientFn func(uuid.UUID, uuid.UUID, uuid.UUID) error
}

func (s *stubRecipeService) ByID(id, hid uuid.UUID) (*domain.Recipe, error) {
	return s.byIDFn(id, hid)
}
func (s *stubRecipeService) ByIDPreload(id, uid, hid uuid.UUID, preload types.PreloadOptions) (*domain.Recipe, error) {
	if s.byIDPreloadFn != nil {
		return s.byIDPreloadFn(id, uid, hid, preload)
	}
	return nil, nil
}
func (s *stubRecipeService) Search(uid, hid uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	return s.searchFn(uid, hid, opts)
}
func (s *stubRecipeService) ImportRecipe(_ context.Context, _ *domain.Recipe) (*domain.Recipe, error) {
	panic("not implemented in this test")
}
func (s *stubRecipeService) Create(r *domain.Recipe, uid, hid uuid.UUID) error {
	if s.createFn != nil {
		return s.createFn(r, uid, hid)
	}
	return nil
}
func (s *stubRecipeService) Update(r *domain.Recipe, uid, hid uuid.UUID) error {
	if s.updateFn != nil {
		return s.updateFn(r, uid, hid)
	}
	return nil
}
func (s *stubRecipeService) Delete(id, hid uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(id, hid)
	}
	return nil
}
func (s *stubRecipeService) UserSave(rid, uid, hid uuid.UUID) error {
	if s.userSaveFn != nil {
		return s.userSaveFn(rid, uid, hid)
	}
	return nil
}
func (s *stubRecipeService) UserUnsave(rid, uid uuid.UUID) error {
	if s.userUnsaveFn != nil {
		return s.userUnsaveFn(rid, uid)
	}
	return nil
}
func (s *stubRecipeService) AddEquipment(rid, eid, hid uuid.UUID) error {
	if s.addEquipmentFn != nil {
		return s.addEquipmentFn(rid, eid, hid)
	}
	return nil
}
func (s *stubRecipeService) RemoveEquipment(rid, eid, hid uuid.UUID) error {
	if s.removeEquipmentFn != nil {
		return s.removeEquipmentFn(rid, eid, hid)
	}
	return nil
}
func (s *stubRecipeService) CreateIngredient(ing *domain.RecipeIngredient, hid uuid.UUID) error {
	if s.createIngredientFn != nil {
		return s.createIngredientFn(ing, hid)
	}
	return nil
}
func (s *stubRecipeService) DeleteIngredient(id, rid, hid uuid.UUID) error {
	if s.deleteIngredientFn != nil {
		return s.deleteIngredientFn(id, rid, hid)
	}
	return nil
}

// buildApp creates a Fiber app with a stubbed RecipeService already wired in.
func buildApp(t *testing.T, svc *stubRecipeService) *fiber.App {
	t.Helper()
	const testSecret = "test-jwt-secret-key-for-handler-tests"
	t.Setenv("JWT_SECRET_KEY", testSecret)
	t.Setenv("JWT_SECRET_EXPIRE_MINUTES", "60")

	app := fiber.New(configs.FiberConfig())
	handler := api.NewRecipeHandler(svc)
	protected := app.Group("/api/v1", middlewares.Protected())
	protected.Get("/recipes/:id", handler.GetRecipe)
	protected.Get("/recipes", handler.Search)
	protected.Post("/recipes", handler.CreateRecipe)
	protected.Patch("/recipes/:id", handler.UpdateRecipe)
	protected.Delete("/recipes/:id", handler.DeleteRecipe)
	protected.Post("/recipes/:id/favorite", handler.SaveRecipe)
	protected.Delete("/recipes/:id/favorite", handler.UnsaveRecipe)
	protected.Post("/recipes/:id/equipment/:equipmentId", handler.AddEquipment)
	protected.Delete("/recipes/:id/equipment/:equipmentId", handler.RemoveEquipment)
	protected.Post("/recipes/:id/ingredients", handler.CreateIngredient)
	protected.Delete("/recipes/:id/ingredients/:ingredientId", handler.DeleteIngredient)

	return app
}

// makeToken generates a signed JWT for the given user and household.
func makeToken(t *testing.T, userID, householdID uuid.UUID) string {
	t.Helper()
	tok, err := tokens.GenerateNew(userID, householdID)
	require.NoError(t, err)
	return "Bearer " + tok.Access
}

func TestRecipeHandler_GetRecipe_ValidJWT_ReturnsRecipe(t *testing.T) {
	recipeID := uuid.New()
	hid := uuid.New()
	uid := uuid.New()
	name := "Borsch"

	svc := &stubRecipeService{
		byIDPreloadFn: func(id, receivedUid, receivedHid uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			assert.Equal(t, recipeID, id)
			assert.Equal(t, uid, receivedUid)
			assert.Equal(t, hid, receivedHid)
			return &domain.Recipe{ID: recipeID, Name: &name}, nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+recipeID.String(), nil)
	req.Header.Set("Authorization", makeToken(t, uid, hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got domain.Recipe
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, recipeID, got.ID)
}

func TestRecipeHandler_GetRecipe_NoJWT_Returns401(t *testing.T) {
	svc := &stubRecipeService{}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+uuid.New().String(), nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestRecipeHandler_GetRecipe_RecipeNotFound_Returns404(t *testing.T) {
	hid := uuid.New()
	svc := &stubRecipeService{
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			return nil, sentinels.ErrNotFound
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var errBody sentinels.Error
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&errBody))
	assert.Equal(t, http.StatusNotFound, errBody.Status)
}

func TestRecipeHandler_GetRecipe_ForbiddenHousehold_Returns403(t *testing.T) {
	hid := uuid.New()
	svc := &stubRecipeService{
		byIDPreloadFn: func(_, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			return nil, sentinels.ErrForbidden
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/"+uuid.New().String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestRecipeHandler_GetRecipe_InvalidUUID_Returns400(t *testing.T) {
	svc := &stubRecipeService{}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes/not-a-uuid", nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), uuid.New()))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRecipeHandler_Search_ReturnsListResponse(t *testing.T) {
	hid := uuid.New()
	uid := uuid.New()
	r1 := domain.Recipe{ID: uuid.New()}
	r2 := domain.Recipe{ID: uuid.New()}

	svc := &stubRecipeService{
		searchFn: func(u, h uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			assert.Equal(t, uid, u)
			assert.Equal(t, hid, h)
			return []domain.Recipe{r1, r2}, 2, nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes", nil)
	req.Header.Set("Authorization", makeToken(t, uid, hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body types.ListResponse[domain.Recipe]
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, 2, body.Meta.Total)
	assert.Len(t, body.Data, 2)
}

func TestRecipeHandler_Search_EmptyResult_ReturnsZeroTotal(t *testing.T) {
	svc := &stubRecipeService{
		searchFn: func(_, _ uuid.UUID, _ domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
			return nil, 0, nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/recipes", nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), uuid.New()))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body types.ListResponse[domain.Recipe]
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, 0, body.Meta.Total)
}

func TestRecipeHandler_CreateRecipe_Returns201WithRecipe(t *testing.T) {
	hid := uuid.New()
	uid := uuid.New()
	newID := uuid.New()
	svc := &stubRecipeService{
		createFn: func(r *domain.Recipe, u, h uuid.UUID) error {
			assert.Equal(t, uid, u)
			assert.Equal(t, hid, h)
			r.ID = newID
			return nil
		},
	}
	app := buildApp(t, svc)

	body, _ := json.Marshal(domain.Recipe{Name: new("Varenyky")})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", makeToken(t, uid, hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var got domain.Recipe
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, newID, got.ID)
}

func TestRecipeHandler_UpdateRecipe_Returns200WithUpdatedRecipe(t *testing.T) {
	recipeID := uuid.New()
	hid := uuid.New()
	uid := uuid.New()
	updatedName := "Updated Borsch"

	svc := &stubRecipeService{
		updateFn: func(r *domain.Recipe, u, h uuid.UUID) error {
			assert.Equal(t, recipeID, r.ID)
			assert.Equal(t, uid, u)
			assert.Equal(t, hid, h)
			return nil
		},
		byIDPreloadFn: func(id, _, _ uuid.UUID, _ types.PreloadOptions) (*domain.Recipe, error) {
			return &domain.Recipe{ID: id, Name: &updatedName}, nil
		},
	}
	app := buildApp(t, svc)

	body, _ := json.Marshal(domain.Recipe{Name: &updatedName})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/recipes/"+recipeID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", makeToken(t, uid, hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var got domain.Recipe
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, updatedName, *got.Name)
}

func TestRecipeHandler_DeleteRecipe_Returns204(t *testing.T) {
	recipeID := uuid.New()
	hid := uuid.New()

	deleteCalled := false
	svc := &stubRecipeService{
		deleteFn: func(id, h uuid.UUID) error {
			deleteCalled = true
			assert.Equal(t, recipeID, id)
			assert.Equal(t, hid, h)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/"+recipeID.String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.True(t, deleteCalled)
}

func TestRecipeHandler_SaveRecipe_Returns204(t *testing.T) {
	recipeID := uuid.New()
	hid := uuid.New()
	uid := uuid.New()

	svc := &stubRecipeService{
		userSaveFn: func(rid, u, h uuid.UUID) error {
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, uid, u)
			assert.Equal(t, hid, h)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/"+recipeID.String()+"/favorite", nil)
	req.Header.Set("Authorization", makeToken(t, uid, hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRecipeHandler_UnsaveRecipe_Returns204(t *testing.T) {
	recipeID := uuid.New()
	uid := uuid.New()

	svc := &stubRecipeService{
		userUnsaveFn: func(rid, u uuid.UUID) error {
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, uid, u)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/"+recipeID.String()+"/favorite", nil)
	req.Header.Set("Authorization", makeToken(t, uid, uuid.New()))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRecipeHandler_AddEquipment_Returns204(t *testing.T) {
	recipeID := uuid.New()
	equipmentID := uuid.New()
	hid := uuid.New()

	svc := &stubRecipeService{
		addEquipmentFn: func(rid, eid, h uuid.UUID) error {
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, equipmentID, eid)
			assert.Equal(t, hid, h)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/"+recipeID.String()+"/equipment/"+equipmentID.String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRecipeHandler_RemoveEquipment_Returns204(t *testing.T) {
	recipeID := uuid.New()
	equipmentID := uuid.New()
	hid := uuid.New()

	svc := &stubRecipeService{
		removeEquipmentFn: func(rid, eid, h uuid.UUID) error {
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, equipmentID, eid)
			assert.Equal(t, hid, h)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/"+recipeID.String()+"/equipment/"+equipmentID.String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRecipeHandler_CreateIngredient_Returns201WithIngredient(t *testing.T) {
	recipeID := uuid.New()
	hid := uuid.New()
	rawText := "2 cups flour"

	svc := &stubRecipeService{
		createIngredientFn: func(ing *domain.RecipeIngredient, h uuid.UUID) error {
			assert.Equal(t, recipeID, ing.RecipeID)
			assert.Equal(t, hid, h)
			ing.ID = uuid.New()
			return nil
		},
	}
	app := buildApp(t, svc)

	body, _ := json.Marshal(domain.RecipeIngredient{RawText: rawText})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recipes/"+recipeID.String()+"/ingredients", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var got domain.RecipeIngredient
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.NotEqual(t, uuid.Nil, got.ID)
}

func TestRecipeHandler_DeleteIngredient_Returns204(t *testing.T) {
	recipeID := uuid.New()
	ingredientID := uuid.New()
	hid := uuid.New()

	svc := &stubRecipeService{
		deleteIngredientFn: func(id, rid, h uuid.UUID) error {
			assert.Equal(t, ingredientID, id)
			assert.Equal(t, recipeID, rid)
			assert.Equal(t, hid, h)
			return nil
		},
	}
	app := buildApp(t, svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/recipes/"+recipeID.String()+"/ingredients/"+ingredientID.String(), nil)
	req.Header.Set("Authorization", makeToken(t, uuid.New(), hid))
	resp, err := app.Test(req)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
