package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// HouseholdOwned restricts a query to rows belonging to the given household via household_id.
func HouseholdOwned(householdID uuid.UUID) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("household_id = ?", householdID)
	}
}

// ActiveFeed restricts a feeds query to active feeds only.
func ActiveFeed(db *gorm.DB) *gorm.DB {
	return db.Where("active = ?", true)
}

// FeedSubscribedByHousehold restricts a feeds query to feeds the given household is subscribed to,
// joining through the feed_subscriptions junction table.
func FeedSubscribedByHousehold(householdID uuid.UUID) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Joins(
			"JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id AND feed_subscriptions.household_id = ?",
			householdID,
		)
	}
}

// RecipeScopeExistsFilter appends a WHERE EXISTS filter that limits results to entities with
// recipes visible to the given household under the named scope. entitySubquery must be a SQL
// fragment of the form "SELECT 1 FROM … WHERE <entity_condition>" without a trailing semicolon;
// the household-scoped recipe visibility condition is appended as an additional AND clause.
//
// The filter is skipped (all entities returned) when householdID is uuid.Nil (global/admin browse)
// or scope is empty (caller explicitly requests unfiltered results — e.g. a public listing page).
func RecipeScopeExistsFilter(entitySubquery, scope string, householdID uuid.UUID) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if householdID == uuid.Nil || scope == "" {
			return db
		}
		scopeWhere, scopeArgs := scopeWhereArgs(scope, householdID)
		return db.Where("EXISTS ("+entitySubquery+" AND "+scopeWhere+")", scopeArgs...)
	}
}

// IsCanonicalFood restricts a foods query to canonical (non-alias) rows only.
func IsCanonicalFood(db *gorm.DB) *gorm.DB {
	return db.Where("canonical_food_id IS NULL")
}

// IsCanonicalUnit restricts a units query to canonical (non-alias) rows only.
func IsCanonicalUnit(db *gorm.DB) *gorm.DB {
	return db.Where("canonical_unit_id IS NULL")
}

// WithPreloadItemAssociations eagerly loads a shopping item's Unit and Food relations.
func WithPreloadItemAssociations(db *gorm.DB) *gorm.DB {
	return db.Preload("Unit").Preload("Food")
}

// WithPreloadIngredients eagerly loads a recipe's Ingredients relation, joining Food and Unit.
func WithPreloadIngredients(db *gorm.DB) *gorm.DB {
	return db.Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
		return db.Joins("Food").Joins("Unit")
	})
}

// WithPreloadInstructions eagerly loads a recipe's Instructions relation, ordered by step index.
func WithPreloadInstructions(db *gorm.DB) *gorm.DB {
	return db.Preload("Instructions", func(db *gorm.DB) *gorm.DB {
		// "order" is a reserved word: build the column so the dialect quotes it.
		return db.Order(clause.OrderByColumn{Column: clause.Column{Table: "recipe_instructions", Name: "order"}})
	})
}
