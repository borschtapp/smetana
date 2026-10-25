package repositories_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/database"
	"borscht.app/smetana/internal/repositories"
	"borscht.app/smetana/internal/sentinels"
)

// seedShoppingList creates a list in a new household.
func seedShoppingList(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	listID := uuid.New()
	require.NoError(t, db.Create(&domain.ShoppingList{ID: listID, HouseholdID: seedHousehold(t, db), Name: "Groceries"}).Error)
	return listID
}

func TestShoppingListRepository_CreateItems_DuplicateFoodInSameList_RejectedByUniqueConstraint(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewShoppingListRepository(db)

	listID := seedShoppingList(t, db)

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

	listID := seedShoppingList(t, db)

	first := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, Text: "something"}
	second := &domain.ShoppingItem{ID: uuid.New(), ShoppingListID: listID, Text: "something else"}

	require.NoError(t, repo.CreateItems([]*domain.ShoppingItem{first}))
	err := repo.CreateItems([]*domain.ShoppingItem{second})

	assert.NoError(t, err, "free-text items without a food reference must not be constrained by the food uniqueness index")
}

func TestShoppingListRepository_UpdateItem_LoadedItem_ClearsAmountAndKeepsOtherFields(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewShoppingListRepository(db)

	listID := seedShoppingList(t, db)
	foodID := uuid.New()
	require.NoError(t, db.Create(&domain.Food{ID: foodID, Name: "Milk", Slug: foodID.String()}).Error)

	itemID := uuid.New()
	require.NoError(t, repo.CreateItems([]*domain.ShoppingItem{{ID: itemID, ShoppingListID: listID, FoodID: &foodID, Text: "milk", Amount: new(2.0)}}))

	item, err := repo.ItemByID(itemID)
	require.NoError(t, err)
	item.Amount = nil
	item.IsBought = true
	require.NoError(t, repo.UpdateItem(item))

	got, err := repo.ItemByID(itemID)
	require.NoError(t, err)
	assert.Nil(t, got.Amount, "a nil amount on a loaded item must be written as NULL")
	assert.True(t, got.IsBought)
	assert.Equal(t, "milk", got.Text)
	assert.Equal(t, &foodID, got.FoodID)
}

func TestShoppingListRepository_UpdateItem_ChangedUnit_ReloadsUnitAssociation(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewShoppingListRepository(db)

	listID := seedShoppingList(t, db)
	gram := &domain.Unit{Name: "gram", Slug: "g", BaseFactor: 1}
	kilo := &domain.Unit{Name: "kilogram", Slug: "kg", BaseFactor: 1000}
	require.NoError(t, db.Create(gram).Error)
	require.NoError(t, db.Create(kilo).Error)

	itemID := uuid.New()
	require.NoError(t, repo.CreateItems([]*domain.ShoppingItem{{ID: itemID, ShoppingListID: listID, Text: "flour", Amount: new(500.0), UnitID: &gram.ID}}))

	item, err := repo.ItemByID(itemID)
	require.NoError(t, err)
	require.NotNil(t, item.Unit)
	item.UnitID = &kilo.ID
	require.NoError(t, repo.UpdateItem(item))

	require.NotNil(t, item.Unit)
	assert.Equal(t, kilo.ID, item.Unit.ID, "the preloaded unit must follow the new unit_id")

	item.UnitID = nil
	require.NoError(t, repo.UpdateItem(item))
	assert.Nil(t, item.Unit, "clearing unit_id must clear the preloaded unit")
}

func defaultListsOf(t *testing.T, db *gorm.DB, householdID uuid.UUID) []domain.ShoppingList {
	t.Helper()
	var lists []domain.ShoppingList
	require.NoError(t, db.Scopes(repositories.HouseholdOwned(householdID)).Where("is_default = ?", true).Find(&lists).Error)
	return lists
}

func TestUserRepository_Create_CreatesUnnamedDefaultShoppingList(t *testing.T) {
	db := openPrivateTestDB(t)
	repo := repositories.NewUserRepository(db)

	user := &domain.User{Name: "Alice", Email: "alice@example.com"}
	require.NoError(t, repo.Create(user))

	lists := defaultListsOf(t, db, user.HouseholdID)
	require.Len(t, lists, 1)
	assert.Empty(t, lists[0].Name, "the default list is unnamed so clients can localize it")
}

func TestHouseholdRepository_MoveUserToNewHousehold_CreatesDefaultShoppingList(t *testing.T) {
	db := openPrivateTestDB(t)
	user := &domain.User{Name: "Bob", Email: "bob@example.com"}
	require.NoError(t, repositories.NewUserRepository(db).Create(user))

	household, err := repositories.NewHouseholdRepository(db).MoveUserToNewHousehold(user, "EUR")
	require.NoError(t, err)

	assert.Len(t, defaultListsOf(t, db, household.ID), 1)
}

func TestSeedDefaultShoppingLists_BackfillsOnlyHouseholdsWithoutOne(t *testing.T) {
	db := openPrivateTestDB(t)

	withoutList := seedHousehold(t, db)
	withList := seedHousehold(t, db)
	require.NoError(t, db.Create(&domain.ShoppingList{HouseholdID: withList, Name: "Einkauf", IsDefault: true}).Error)

	require.NoError(t, database.SeedDefaultShoppingLists(db))
	require.NoError(t, database.SeedDefaultShoppingLists(db), "a second run must be a no-op")

	backfilled := defaultListsOf(t, db, withoutList)
	require.Len(t, backfilled, 1)
	assert.Empty(t, backfilled[0].Name)

	existing := defaultListsOf(t, db, withList)
	require.Len(t, existing, 1)
	assert.Equal(t, "Einkauf", existing[0].Name, "an existing default list must be left alone")
}
