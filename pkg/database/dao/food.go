package dao

import (
	"errors"

	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/utils"
)

func FindOrCreateFood(food *domain.Food) error {
	tag := []string{utils.CreateTag(food.Name)}

	subQuery := database.DB.Select("food_id").Where("tag IN (?)", tag).Table("food_tags").Limit(1)
	if err := database.DB.Model(&domain.Food{}).Where("id = (?)", subQuery).First(&food).Error; err == nil {
		return nil // found
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else {
		if err := database.DB.Create(&food).Error; err != nil {
			return err
		}

		return nil
	}
}
