package repositories

import "github.com/google/uuid"

// scopeWhereArgs returns a SQL fragment and bind args that restrict a recipes-joined
// query to the subset of recipes relevant to the given scope. The fragment is intended
// for use inside EXISTS or correlated-subquery contexts where the outer table is `recipes`.
//
// scope values:
//   - "feeds"  — only recipes from feeds the household is subscribed to
//   - "saved"  — only recipes saved by the household
//   - ""       — all household-visible recipes (own + saved + feeds + collections)
func scopeWhereArgs(scope string, id uuid.UUID) (string, []any) {
	switch scope {
	case "feeds":
		return `EXISTS (SELECT 1 FROM feed_subscriptions WHERE feed_subscriptions.feed_id = recipes.feed_id AND feed_subscriptions.household_id = ?)`, []any{id}
	case "saved":
		return `EXISTS (SELECT 1 FROM recipes_saved WHERE recipes_saved.recipe_id = recipes.id AND recipes_saved.household_id = ?)`, []any{id}
	default:
		return `
			recipes.household_id = ?
			OR EXISTS (SELECT 1 FROM recipes_saved WHERE recipes_saved.recipe_id = recipes.id AND recipes_saved.household_id = ?)
			OR EXISTS (SELECT 1 FROM feed_subscriptions WHERE feed_subscriptions.feed_id = recipes.feed_id AND feed_subscriptions.household_id = ?)
			OR EXISTS (SELECT 1 FROM collection_recipes JOIN collections ON collections.id = collection_recipes.collection_id WHERE collection_recipes.recipe_id = recipes.id AND collections.household_id = ?)
		`, []any{id, id, id, id}
	}
}
