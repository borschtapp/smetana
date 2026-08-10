package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
)

func TestShoppingListRepository_CreateItems_DuplicateFoodInSameList_RejectedByUniqueConstraint(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewShoppingListRepository(db)

	householdID := seedHousehold(t, db)
	listID := uuid.New()
	require.NoError(t, db.Create(&domain.ShoppingList{ID: listID, HouseholdID: householdID, Name: "Groceries"}).Error)

	foodID := uuid.New()
	require.NoError(t, db.Create(&domain.Food{ID: foodID, Name: "Flour", Slug: foodID.String()}).Error)

	first := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, FoodID: &foodID, Text: "flour"}
	require.NoError(t, repo.CreateItems([]*domain.ShoppingItem{first}))

	second := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, FoodID: &foodID, Text: "flour again"}
	err := repo.CreateItems([]*domain.ShoppingItem{second})

	require.ErrorIs(t, err, sentinels.ErrAlreadyExists, "a second row for the same (list, food) pair must be rejected, not silently duplicated")
}

func TestShoppingListRepository_CreateItems_NilFoodDoesNotCollide(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewShoppingListRepository(db)

	householdID := seedHousehold(t, db)
	listID := uuid.New()
	require.NoError(t, db.Create(&domain.ShoppingList{ID: listID, HouseholdID: householdID, Name: "Groceries"}).Error)

	first := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, Text: "something"}
	second := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, Text: "something else"}

	require.NoError(t, repo.CreateItems([]*domain.ShoppingItem{first}))
	err := repo.CreateItems([]*domain.ShoppingItem{second})

	assert.NoError(t, err, "free-text items without a food reference must not be constrained by the food uniqueness index")
}
