package suggestions

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
)

// fakeRecipeRepo is an in-memory stand-in for domain.RecipeRepository, covering only
// UpdateIngredient, which is all IngredientFoodLinkApplier needs.
type fakeRecipeRepo struct {
	domain.RecipeRepository
	updated []*domain.RecipeIngredient
}

func (r *fakeRecipeRepo) UpdateIngredient(i *domain.RecipeIngredient) error {
	r.updated = append(r.updated, i)
	return nil
}

func TestIngredientFoodLinkApplier_Apply_LinksIngredientToFood(t *testing.T) {
	ingredientID, recipeID, foodID := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeRecipeRepo{}
	applier := NewIngredientFoodLinkApplier(repo)

	payload, _ := json.Marshal(map[string]any{
		"ingredient_id": ingredientID,
		"recipe_id":     recipeID,
		"food_id":       foodID,
	})
	sg := &domain.Suggestion{Payload: payload}

	require.NoError(t, applier.Apply(context.Background(), sg))
	require.Len(t, repo.updated, 1)
	assert.Equal(t, &foodID, repo.updated[0].FoodID)
	assert.Equal(t, ingredientID, repo.updated[0].ID)
}

func TestIngredientFoodLinkDetector_Detect_BatchLookupFindsMatches(t *testing.T) {
	db := openPrivateTestDB(t)
	recipe := seedRecipe(t, db)

	// One ingredient matches an existing food by slug, one by lowercase name, one has no match.
	carrotFood := seedFood(t, db, "carrot", "Carrot")
	onionFood := seedFood(t, db, "yellow-onion", "onion") // matched by name, not slug
	seedIngredient(t, db, recipe.ID, "carrot")
	seedIngredient(t, db, recipe.ID, "onion")
	seedIngredient(t, db, recipe.ID, "no such ingredient")

	detector := NewIngredientFoodLinkDetector(repositories.NewRecipeRepository(db), repositories.NewFoodRepository(db))
	got, err := detector.Detect(context.Background(), time.Time{}, nil)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byIngredientName := make(map[string]*domain.Suggestion, len(got))
	for _, s := range got {
		var payload struct {
			IngredientName string `json:"ingredient_name"`
		}
		require.NoError(t, json.Unmarshal(s.Payload, &payload))
		byIngredientName[payload.IngredientName] = s
	}

	require.Contains(t, byIngredientName, "carrot")
	assert.Equal(t, carrotFood.ID, *byIngredientName["carrot"].TargetID)

	require.Contains(t, byIngredientName, "onion")
	assert.Equal(t, onionFood.ID, *byIngredientName["onion"].TargetID)
}
