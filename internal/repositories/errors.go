package repositories

import (
	"errors"

	"gorm.io/gorm"

	"borscht.app/smetana/internal/sentinels"
)

// mapErr translates GORM infrastructure errors into domain sentinel errors
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return sentinels.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return sentinels.ErrAlreadyExists
	default:
		return err
	}
}
