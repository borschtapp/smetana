package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
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

	createCalled := false
	repo, updated := repoWithExisting(householdID, existing)
	repo.createItemsFn = func([]*domain.ShoppingItem) error {
		createCalled = true
		return nil
	}

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{{FoodID: &foodID, Amount: new(2.0)}}
	result, err := svc.AddItems(context.Background(), items, listID, householdID)

	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 3.0, *result[0].Amount)
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
	repo, _ := repoWithExisting(householdID, existing)
	repo.createItemsFn = func([]*domain.ShoppingItem) error {
		createCalled = true
		return nil
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

	repo, updated := repoWithExisting(householdID, existing)
	unitService := &stubUnitService{
		convertFn: func(amount float64, from, to *uuid.UUID, _ []domain.FoodUnitConversion) (float64, error) {
			if *from == unitKgID && *to == unitGramID {
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
	require.NotNil(t, updated.Amount)
	assert.Equal(t, 2000.0, *updated.Amount, "1.5kg must convert to 1500g and accumulate with existing 500g")
	assert.Equal(t, unitGramID, *updated.UnitID)
}

// repoWithExisting returns a repo whose list holds existing, and the item UpdateItem last persisted.
func repoWithExisting(householdID uuid.UUID, existing domain.ShoppingItem) (*stubShoppingListRepo, *domain.ShoppingItem) {
	updated := &domain.ShoppingItem{}
	return &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		findItemsByFoodIDsFn: func(uuid.UUID, []uuid.UUID) ([]domain.ShoppingItem, error) {
			return []domain.ShoppingItem{existing}, nil
		},
		updateItemFn: func(item *domain.ShoppingItem) error {
			*updated = *item
			return nil
		},
	}, updated
}

func noBridgeUnitService() *stubUnitService {
	return &stubUnitService{
		convertFn: func(float64, *uuid.UUID, *uuid.UUID, []domain.FoodUnitConversion) (float64, error) {
			return 0, sentinels.ErrUnitConversion
		},
	}
}

func TestShoppingListService_AddItems_IncompatibleUnits_ReturnsUnprocessable(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	unitKgID := uuid.New()
	unitPieceID := uuid.New()

	repo, _ := repoWithExisting(householdID, domain.ShoppingItem{FoodID: &foodID, UnitID: &unitKgID, Amount: new(1.0), Text: "apples"})
	repo.updateItemFn = func(*domain.ShoppingItem) error {
		t.Fatal("incompatible amounts must not be persisted")
		return nil
	}

	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, noBridgeUnitService())
	items := []*domain.ShoppingItem{{FoodID: &foodID, UnitID: &unitPieceID, Amount: new(3.0)}}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion, "3 pieces must not be silently dropped or added to 1 kg")
	assert.ErrorContains(t, err, "apples")
}

func TestShoppingListService_AddItems_IncompatibleUnitsInBatch_ReturnsUnprocessable(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	unitKgID := uuid.New()
	unitPieceID := uuid.New()

	repo := &stubShoppingListRepo{byIDFn: ownedListRepo(householdID)}
	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, noBridgeUnitService())
	items := []*domain.ShoppingItem{
		{FoodID: &foodID, UnitID: &unitKgID, Amount: new(1.0)},
		{FoodID: &foodID, UnitID: &unitPieceID, Amount: new(3.0)},
	}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.ErrorIs(t, err, sentinels.ErrUnitConversion)
}

func TestShoppingListService_AddItems_IntoUnitlessItem_ConvertsToItsMissingUnit(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	unitPieceID := uuid.New()

	repo, updated := repoWithExisting(householdID, domain.ShoppingItem{FoodID: &foodID, Amount: new(2.0)})
	unitService := &stubUnitService{
		convertFn: func(amount float64, from, to *uuid.UUID, _ []domain.FoodUnitConversion) (float64, error) {
			require.Equal(t, &unitPieceID, from)
			require.Nil(t, to, "a unitless item must be passed on as unitless, the unit service resolves it")
			return amount, nil
		},
	}

	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, unitService)
	items := []*domain.ShoppingItem{{FoodID: &foodID, UnitID: &unitPieceID, Amount: new(3.0)}}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	require.NotNil(t, updated.Amount)
	assert.Equal(t, 5.0, *updated.Amount, "2 (no unit) and 3 pc must add up as pieces")
	assert.Nil(t, updated.UnitID, "the existing item keeps its unit")
}

func TestShoppingListService_AddItems_UnitlessIntoMass_PassesMissingUnit(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	unitGramID := uuid.New()

	repo, updated := repoWithExisting(householdID, domain.ShoppingItem{FoodID: &foodID, UnitID: &unitGramID, Amount: new(500.0)})
	unitService := &stubUnitService{
		convertFn: func(amount float64, from, to *uuid.UUID, _ []domain.FoodUnitConversion) (float64, error) {
			require.Nil(t, from, "a unitless amount must be passed on as unitless, the unit service resolves it")
			require.Equal(t, &unitGramID, to)
			return amount * 200, nil
		},
	}

	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, unitService)
	items := []*domain.ShoppingItem{{FoodID: &foodID, Amount: new(2.0)}}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	require.NotNil(t, updated.Amount)
	assert.Equal(t, 900.0, *updated.Amount, "2 pieces x 200 g must accumulate with existing 500 g")
	assert.Equal(t, unitGramID, *updated.UnitID)
}

func TestShoppingListService_AddItems_DifferentDimensions_UsesFoodConversion(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	unitGramID := uuid.New()
	unitPieceID := uuid.New()
	pieceToGram := domain.FoodUnitConversion{FoodID: foodID, UnitID: unitPieceID, TargetAmount: 200, TargetUnitID: unitGramID}

	repo, updated := repoWithExisting(householdID, domain.ShoppingItem{FoodID: &foodID, UnitID: &unitGramID, Amount: new(500.0)})
	foodService := &stubFoodService{
		conversionsByFoodIDs: func([]uuid.UUID) (map[uuid.UUID][]domain.FoodUnitConversion, error) {
			return map[uuid.UUID][]domain.FoodUnitConversion{foodID: {pieceToGram}}, nil
		},
	}
	unitService := &stubUnitService{
		convertFn: func(amount float64, from, to *uuid.UUID, conversions []domain.FoodUnitConversion) (float64, error) {
			require.Equal(t, []domain.FoodUnitConversion{pieceToGram}, conversions)
			require.Equal(t, &unitPieceID, from)
			require.Equal(t, &unitGramID, to)
			return amount * 200, nil
		},
	}

	svc := services.NewShoppingListService(repo, nil, foodService, unitService)
	items := []*domain.ShoppingItem{{FoodID: &foodID, UnitID: &unitPieceID, Amount: new(3.0)}}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	require.NotNil(t, updated.Amount)
	assert.Equal(t, 1100.0, *updated.Amount, "3 pieces x 200 g must accumulate with existing 500 g")
	assert.Equal(t, unitGramID, *updated.UnitID)
}

func TestShoppingListService_AddItems_ClientUnitIDs_AreCanonicalized(t *testing.T) {
	householdID := uuid.New()
	alias, canonical, parsedUnit := uuid.New(), uuid.New(), &domain.Unit{ID: uuid.New()}

	var created []*domain.ShoppingItem
	repo := &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		createItemsFn: func(items []*domain.ShoppingItem) error {
			created = items
			return nil
		},
	}
	units := &stubUnitService{aliases: map[uuid.UUID]uuid.UUID{alias: canonical}}

	svc := services.NewShoppingListService(repo, nil, &stubFoodService{}, units)
	clientUnit := alias // rewritten in place
	items := []*domain.ShoppingItem{
		{FoodID: new(uuid.New()), UnitID: &clientUnit},
		{FoodID: new(uuid.New()), UnitID: &parsedUnit.ID, Unit: parsedUnit},
	}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	require.Len(t, created, 2)
	assert.Equal(t, canonical, *created[0].UnitID, "a client-supplied alias must be stored as its canonical unit")
	assert.Equal(t, []uuid.UUID{alias}, units.canonicalized, "parsed units come from FindOrCreate and are canonical already")
}

func TestShoppingListService_AddItems_AliasAndCanonicalFood_MergeIntoOneRow(t *testing.T) {
	householdID := uuid.New()
	alias, canonical := uuid.New(), uuid.New()

	var created []*domain.ShoppingItem
	repo := &stubShoppingListRepo{
		byIDFn: ownedListRepo(householdID),
		createItemsFn: func(items []*domain.ShoppingItem) error {
			created = items
			return nil
		},
	}
	foods := &stubFoodService{aliases: map[uuid.UUID]uuid.UUID{alias: canonical}}

	svc := services.NewShoppingListService(repo, nil, foods, &stubUnitService{})
	clientFood := alias // rewritten in place
	items := []*domain.ShoppingItem{
		{FoodID: &clientFood, Amount: new(2.0)},
		{FoodID: new(canonical), Amount: new(3.0)},
	}
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	require.Len(t, created, 1, "an alias and its canonical food are the same food")
	assert.Equal(t, canonical, *created[0].FoodID)
	assert.Equal(t, 5.0, *created[0].Amount)
}

func TestShoppingListService_AddItems_RestoreWithSameUnit_KeepsLoadedUnit(t *testing.T) {
	householdID := uuid.New()
	foodID := uuid.New()
	gram := &domain.Unit{ID: uuid.New(), Slug: "g"}

	existing := domain.ShoppingItem{ID: uuid.New(), FoodID: &foodID, UnitID: &gram.ID, Unit: gram, Amount: new(100.0), IsBought: true}
	repo, updated := repoWithExisting(householdID, existing)

	svc := newTestShoppingListService(repo)
	items := []*domain.ShoppingItem{{FoodID: &foodID, UnitID: new(gram.ID), Amount: new(200.0)}} // client sends only the ID
	_, err := svc.AddItems(context.Background(), items, uuid.New(), householdID)

	require.NoError(t, err)
	assert.Same(t, gram, updated.Unit, "an unchanged unit ID must keep the loaded unit, so the repository need not reload it")
	assert.Equal(t, 200.0, *updated.Amount)
}
