package api

import (
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
)

func GetPublishers(c *fiber.Ctx) error {
	var publishers []domain.Publisher
	if err := database.DB.Model(&domain.Publisher{}).Find(&publishers).Error; err != nil {
		return err
	}
	return c.JSON(publishers)
}
