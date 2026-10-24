package database

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

// SeedDefaultShoppingLists creates a default list for households that have none.
func SeedDefaultShoppingLists(db *gorm.DB) error {
	hasDefault := db.Model(&domain.ShoppingList{}).Select("1").
		Where("shopping_lists.household_id = households.id AND shopping_lists.is_default = ?", true)

	var householdIDs []uuid.UUID
	if err := db.Model(&domain.Household{}).Where("NOT EXISTS (?)", hasDefault).Pluck("id", &householdIDs).Error; err != nil {
		return err
	}
	if len(householdIDs) == 0 {
		return nil
	}

	lists := make([]domain.ShoppingList, len(householdIDs))
	for i, id := range householdIDs {
		lists[i] = domain.ShoppingList{HouseholdID: id, IsDefault: true}
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(&lists, 100).Error
}
