package repositories

import (
	"errors"

	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

// mapErr translates GORM infrastructure errors into domain sentinel errors
func mapErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.ErrRecordNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return domain.ErrAlreadyExists
	default:
		return err
	}
}
