package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// canonicalIDs maps each alias among ids to its canonical row; column holds the alias target.
func canonicalIDs(db *gorm.DB, model any, column string, ids []uuid.UUID) (map[uuid.UUID]uuid.UUID, error) {
	result := make(map[uuid.UUID]uuid.UUID)
	if len(ids) == 0 {
		return result, nil
	}

	var aliases []struct{ ID, CanonicalID uuid.UUID }
	if err := db.Model(model).Select("id, "+column+" AS canonical_id").Where("id IN ? AND "+column+" IS NOT NULL", ids).Scan(&aliases).Error; err != nil {
		return nil, mapErr(err)
	}
	for _, alias := range aliases {
		result[alias.ID] = alias.CanonicalID
	}
	return result, nil
}

// makeAlias turns mergeID into an alias of keepID and points mergeID's own aliases at keepID,
// keeping aliases one hop deep (resolveAlias follows only one).
func makeAlias(tx *gorm.DB, model any, column string, keepID, mergeID uuid.UUID) error {
	return mapErr(tx.Model(model).Where("id = ? OR "+column+" = ?", mergeID, mergeID).Update(column, keepID).Error)
}

// byIDsResolved loads the rows for ids like ByID does: aliases are replaced by their canonical rows.
// The map is keyed by the requested ID, so an alias ID maps to its canonical row; unknown IDs are left out.
func byIDsResolved[T any](db *gorm.DB, ids []uuid.UUID, idOf func(*T) uuid.UUID, canonicalOf func(*T) *uuid.UUID) (map[uuid.UUID]*T, error) {
	result := make(map[uuid.UUID]*T, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	var rows []T
	if err := db.Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, mapErr(err)
	}
	byID := make(map[uuid.UUID]*T, len(rows))
	for i := range rows {
		byID[idOf(&rows[i])] = &rows[i]
	}

	var missing []uuid.UUID
	for _, row := range byID {
		if c := canonicalOf(row); c != nil {
			if _, ok := byID[*c]; !ok {
				missing = append(missing, *c)
			}
		}
	}
	if len(missing) > 0 {
		var canonical []T
		if err := db.Where("id IN ?", missing).Find(&canonical).Error; err != nil {
			return nil, mapErr(err)
		}
		for i := range canonical {
			byID[idOf(&canonical[i])] = &canonical[i]
		}
	}

	for _, id := range ids {
		row := byID[id]
		if row != nil {
			if c := canonicalOf(row); c != nil {
				row = byID[*c]
			}
		}
		if row != nil {
			result[id] = row
		}
	}
	return result, nil
}
