package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/services"
)

func newTestShoppingListService(repo *stubShoppingListRepo) domain.ShoppingListService {
	return services.NewShoppingListService(repo, nil, &stubFoodService{}, &stubUnitService{})
}

// ownedListRepo returns a byIDFn that satisfies ensureOwned's household check for any list ID.
func ownedListRepo(householdID uuid.UUID) func(uuid.UUID) (*domain.ShoppingList, error) {
	return func(id uuid.UUID) (*domain.ShoppingList, error) {
		return &domain.ShoppingList{ID: id, HouseholdID: householdID}, nil
	}
}

func TestShoppingListService_AddItems_DuplicateFoodInBatch_MergesIntoOneRow(t *testing.T) {
	listID := uuid.New()
	householdID := uuid.New()
	foodID := uuid.New()

	var created []*domain.ShoppingItem
	repo := &stubShoppingListRepo{
		byIDFn:               ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) { return nil, nil },
		createItemsFn: func(items []*domain.ShoppingItem) error {
			created = items
			return nil
		},
	}

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{
		{FoodID: &foodID, Amount: new(2.0), Text: "flour"},
		{FoodID: &foodID, Amount: new(3.0), Text: "flour"},
	}
	result, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.NoError(t, err)
	require.Len(t, result, 1, "returned slice must only contain the merged item")
	assert.Equal(t, 5.0, *result[0].Amount)
	require.Len(t, created, 1, "duplicate food IDs within one batch must be combined into a single row")
	require.NotNil(t, created[0].Amount)
	assert.Equal(t, 5.0, *created[0].Amount, "amounts for the merged rows must be summed")
}

func TestShoppingListService_AddItems_MatchesExistingUnboughtItem_Accumulates(t *testing.T) {
	listID := uuid.New()
	householdID := uuid.New()
	foodID := uuid.New()
	existing := domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, FoodID: &foodID, Amount: new(1.0)}

	var updated *domain.ShoppingItem
	createCalled := false
	repo := &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) {
			return []domain.ShoppingItem{existing}, nil
		},
		updateItemFn: func(item *domain.ShoppingItem) error {
			updated = item
			return nil
		},
		createItemsFn: func([]*domain.ShoppingItem) error {
			createCalled = true
			return nil
		},
	}

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{{FoodID: &foodID, Amount: new(2.0)}}
	result, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 3.0, *result[0].Amount)
	require.NotNil(t, updated)
	require.NotNil(t, updated.Amount)
	assert.Equal(t, 3.0, *updated.Amount, "the new amount must accumulate onto the existing unbought item")
	assert.False(t, createCalled, "a matched item must be updated, not created again")
}

func TestShoppingListService_AddItems_AllItemsMatchExisting_NeverCallsCreate(t *testing.T) {
	listID := uuid.New()
	householdID := uuid.New()
	foodID := uuid.New()
	existing := domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, FoodID: &foodID, IsBought: true}

	createCalled := false
	repo := &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) {
			return []domain.ShoppingItem{existing}, nil
		},
		updateItemFn: func(*domain.ShoppingItem) error { return nil },
		createItemsFn: func([]*domain.ShoppingItem) error {
			createCalled = true
			return nil
		},
	}

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{{FoodID: &foodID, Amount: new(1.0)}}
	result, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.NoError(t, err)
	assert.False(t, createCalled)
	require.Len(t, result, 1)
}

func TestShoppingListService_AddItems_CreateFails_ReturnsError(t *testing.T) {
	listID := uuid.New()
	householdID := uuid.New()
	foodID := uuid.New()

	repo := &stubShoppingListRepo{
		byIDFn:               ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) { return nil, nil },
		createItemsFn:        func([]*domain.ShoppingItem) error { return errDB },
	}

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{{FoodID: &foodID, Amount: new(1.0)}}
	_, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.ErrorIs(t, err, errDB)
}

func TestShoppingListService_AddItems_DifferentUnits_ConvertsAndAccumulates(t *testing.T) {
	listID := uuid.New()
	householdID := uuid.New()
	foodID := uuid.New()
	unitGramID := uuid.New()
	unitKgID := uuid.New()

	existing := domain.ShoppingItem{
		ID:             uuid.New(),
		ShoppingListID: listID,
		FoodID:         &foodID,
		UnitID:         &unitGramID,
		Amount:         new(500.0),
	}

	var updated *domain.ShoppingItem
	repo := &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) {
			return []domain.ShoppingItem{existing}, nil
		},
		updateItemFn: func(item *domain.ShoppingItem) error {
			updated = item
			return nil
		},
	}

	unitService := &stubUnitService{
		convertFn: func(amount float64, from, to uuid.UUID) (float64, error) {
			if from == unitKgID && to == unitGramID {
				return amount * 1000, nil
			}
			return amount, nil
		},
	}

	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, unitService)
	items := []*domain.ShoppingItem{{FoodID: &foodID, UnitID: &unitKgID, Amount: new(1.5)}}
	result, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.NoError(t, err)
	require.Len(t, result, 1)
	require.NotNil(t, updated)
	assert.Equal(t, 2000.0, *updated.Amount, "1.5kg must convert to 1500g and accumulate with existing 500g")
	assert.Equal(t, unitGramID, *updated.UnitID)
}
