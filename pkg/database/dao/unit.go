package dao

import (
	"errors"

	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/utils"
)

func FindOrCreateUnit(unit *domain.Unit) error {
	tag := []string{utils.CreateTag(unit.Name)}

	subQuery := database.DB.Select("unit_id").Where("tag IN (?)", tag).Table("unit_tags").Limit(1)
	if err := database.DB.Model(&domain.Unit{}).Where("id = (?)", subQuery).First(&unit).Error; err == nil {
		return nil // found
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else {
		if err := database.DB.Create(&unit).Error; err != nil {
			return err
		}

		return nil
	}
}
