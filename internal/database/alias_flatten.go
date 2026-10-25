package database

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

// FlattenAliases points every food and unit alias directly at its canonical row.
// Older merges left chains (a -> b -> c); lookups follow only one hop.
func FlattenAliases(db *gorm.DB) error {
	if err := flattenAliases(db, &domain.Food{}, "canonical_food_id"); err != nil {
		return fmt.Errorf("flatten food aliases: %w", err)
	}
	if err := flattenAliases(db, &domain.Unit{}, "canonical_unit_id"); err != nil {
		return fmt.Errorf("flatten unit aliases: %w", err)
	}
	return nil
}

func flattenAliases(db *gorm.DB, model any, column string) error {
	var aliases []struct{ ID, CanonicalID uuid.UUID }
	if err := db.Model(model).Select("id, " + column + " AS canonical_id").Where(column + " IS NOT NULL").Scan(&aliases).Error; err != nil {
		return err
	}
	target := make(map[uuid.UUID]uuid.UUID, len(aliases))
	for _, a := range aliases {
		target[a.ID] = a.CanonicalID
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, a := range aliases {
			root := a.CanonicalID
			for seen := map[uuid.UUID]bool{a.ID: true}; !seen[root]; {
				next, ok := target[root]
				if !ok {
					break
				}
				seen[root] = true
				root = next
			}
			if root == a.CanonicalID || root == a.ID {
				continue // already flat, or a cycle
			}
			if err := tx.Model(model).Where("id = ?", a.ID).Update(column, root).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
