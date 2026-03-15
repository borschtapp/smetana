package api_test

import (
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

	byIDFn   func(uuid.UUID, uuid.UUID) (*domain.Recipe, error)
	searchFn func(uuid.UUID, uuid.UUID, types.SearchOptions) ([]domain.Recipe, int64, error)
}

func (s *stubRecipeService) ByID(id, hid uuid.UUID) (*domain.Recipe, error) {
	return s.byIDFn(id, hid)
}
func (s *stubRecipeService) Search(uid, hid uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	return s.searchFn(uid, hid, opts)
}
func (s *stubRecipeService) ImportRecipe(_ context.Context, _ *domain.Recipe) (*domain.Recipe, error) {
	panic("not implemented in this test")
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
		byIDFn: func(id, h uuid.UUID) (*domain.Recipe, error) {
			assert.Equal(t, recipeID, id)
			assert.Equal(t, hid, h)
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
		byIDFn: func(_, _ uuid.UUID) (*domain.Recipe, error) {
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
		byIDFn: func(_, _ uuid.UUID) (*domain.Recipe, error) {
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
		searchFn: func(u, h uuid.UUID, _ types.SearchOptions) ([]domain.Recipe, int64, error) {
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
		searchFn: func(_, _ uuid.UUID, _ types.SearchOptions) ([]domain.Recipe, int64, error) {
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
