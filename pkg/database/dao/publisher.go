package dao

import (
	"errors"

	"github.com/gofiber/fiber/v2/log"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/utils"
)

func FindOrCreatePublisher(pub *domain.Publisher) error {
	tag := []string{utils.CreateTag(pub.Name), utils.CreateHostnameTag(pub.Url)}

	subQuery := database.DB.Select("publisher_id").Where("tag IN (?)", tag).Table("publisher_tags").Limit(1)
	if err := database.DB.Model(&domain.Publisher{}).Where("id = (?)", subQuery).First(&pub).Error; err == nil {
		return nil // found
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else {
		if pub.Image != nil && len(*pub.Image) != 0 {
			path := pub.FilePath()
			if storedImage, err := utils.DownloadAndPutObject(*pub.Image, path); err != nil {
				log.Warnf("en error on download publisher image %v: %s", pub, err.Error())
			} else {
				pub.Image = &storedImage
			}
		}

		if err := database.DB.Create(&pub).Error; err != nil {
			return err
		}

		return nil
	}
}
