package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type IngredientFoodLinkDetector struct {
	recipeRepo domain.RecipeRepository
	foodRepo   domain.FoodRepository
}

func NewIngredientFoodLinkDetector(recipeRepo domain.RecipeRepository, foodRepo domain.FoodRepository) *IngredientFoodLinkDetector {
	return &IngredientFoodLinkDetector{recipeRepo: recipeRepo, foodRepo: foodRepo}
}

func (d *IngredientFoodLinkDetector) Type() domain.SuggestionType {
	return domain.SuggestionIngredientFood
}
func (d *IngredientFoodLinkDetector) Scope() Scope { return ScopeGlobal }

func (d *IngredientFoodLinkDetector) Detect(_ context.Context, since time.Time, _ *uuid.UUID) ([]*domain.Suggestion, error) {
	ingredients, err := d.recipeRepo.IngredientsWithoutFoodSince(since)
	if err != nil {
		return nil, fmt.Errorf("ingredient_food_link detect: %w", err)
	}

	// Collect candidate slugs/names across the whole work set so we can resolve
	// matching foods with a single batched query instead of one First() per ingredient.
	// Each ingredient's slug is cached here too, so the second loop below doesn't recompute it.
	slugSet := make(map[string]bool)
	nameSet := make(map[string]bool)
	slugs := make([]string, len(ingredients))
	for i := range ingredients {
		ing := &ingredients[i]
		if ing.Name == nil || *ing.Name == "" {
			continue
		}
		slugs[i] = utils.CreateTag(*ing.Name)
		slugSet[slugs[i]] = true
		nameSet[strings.ToLower(*ing.Name)] = true
	}
	if len(slugSet) == 0 {
		return nil, nil
	}

	candidates, err := d.foodRepo.BySlugsOrNames(utils.MapKeys(slugSet), utils.MapKeys(nameSet))
	if err != nil {
		return nil, fmt.Errorf("ingredient_food_link detect: batch food lookup: %w", err)
	}

	bySlug := make(map[string]*domain.Food, len(candidates))
	byName := make(map[string]*domain.Food, len(candidates))
	for i := range candidates {
		f := &candidates[i]
		bySlug[f.Slug] = f
		byName[strings.ToLower(f.Name)] = f
	}

	var suggestions []*domain.Suggestion
	for i := range ingredients {
		ing := &ingredients[i]
		if ing.Name == nil || *ing.Name == "" {
			continue
		}

		food, ok := bySlug[slugs[i]]
		if !ok {
			food, ok = byName[strings.ToLower(*ing.Name)]
		}
		if !ok {
			// No matching food found; skip this ingredient.
			continue
		}

		suggestions = append(suggestions, newSuggestion(domain.SuggestionIngredientFood, "ingredient", ing.ID, &food.ID, 0.85,
			map[string]any{
				"ingredient_id":   ing.ID,
				"recipe_id":       ing.RecipeID,
				"food_id":         food.ID,
				"food_name":       food.Name,
				"ingredient_name": *ing.Name,
			},
			fmt.Sprintf("Ingredient %q matches food %q", *ing.Name, food.Name),
			fmt.Sprintf("ingredient_food_link:%s", ing.ID),
		))
	}
	return suggestions, nil
}

// --- Applier ---

type IngredientFoodLinkApplier struct {
	recipeRepo domain.RecipeRepository
}

func NewIngredientFoodLinkApplier(recipeRepo domain.RecipeRepository) *IngredientFoodLinkApplier {
	return &IngredientFoodLinkApplier{recipeRepo: recipeRepo}
}

func (a *IngredientFoodLinkApplier) Type() domain.SuggestionType {
	return domain.SuggestionIngredientFood
}

func (a *IngredientFoodLinkApplier) Apply(_ context.Context, s *domain.Suggestion) error {
	var payload struct {
		IngredientID uuid.UUID `json:"ingredient_id"`
		RecipeID     uuid.UUID `json:"recipe_id"`
		FoodID       uuid.UUID `json:"food_id"`
	}
	if err := json.Unmarshal(s.Payload, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	ing := &domain.RecipeIngredient{
		ID:       payload.IngredientID,
		RecipeID: payload.RecipeID,
		FoodID:   &payload.FoodID,
	}
	return a.recipeRepo.UpdateIngredient(ing)
}
