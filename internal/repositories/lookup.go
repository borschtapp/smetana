package repositories

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// byIDs bulk-loads rows by primary key into an ID-keyed map
func byIDs[T any](db *gorm.DB, ids []uuid.UUID, id func(*T) uuid.UUID) (map[uuid.UUID]*T, error) {
	result := make(map[uuid.UUID]*T)
	if len(ids) == 0 {
		return result, nil
	}

	var rows []T
	tx := db.Where("id IN ?", ids).Find(&rows)
	if tx.Error != nil {
		return nil, fmt.Errorf("%s by ids: %w", tx.Statement.Table, mapErr(tx.Error))
	}
	for i := range rows {
		result[id(&rows[i])] = &rows[i]
	}
	return result, nil
}

// detach clears an alias column
func detach[T any](db *gorm.DB, id uuid.UUID, aliasCol string) error {
	var zero T
	tx := db.Model(&zero).Where("id = ?", id).Update(aliasCol, nil)
	if tx.Error != nil {
		return fmt.Errorf("detach %s %s: %w", tx.Statement.Table, id, mapErr(tx.Error))
	}
	return nil
}
