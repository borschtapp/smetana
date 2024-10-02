package api

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/utils"
)

func GetRecipes(c *fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var recipes []domain.Recipe
	if err := database.DB.Model(&domain.User{ID: tokenData.ID}).
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		Association("Recipes").
		Find(&recipes); err != nil {
		return err
	}
	return c.JSON(recipes)
}

func GetRecipe(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return err
	}

	var recipe domain.Recipe
	if err := database.DB.
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipe, id).Error; err != nil {
		return err
	}
	return c.JSON(recipe)
}

func CreateRecipe(c *fiber.Ctx) error {
	recipe := new(domain.Recipe)
	if err := c.BodyParser(&recipe); err != nil {
		return err
	}

	if err := database.DB.Create(&recipe).Error; err != nil {
		return err
	}
	return c.JSON(recipe)
}

func UpdateRecipe(c *fiber.Ctx) error {
	var recipe domain.Recipe
	if err := c.BodyParser(&recipe); err != nil {
		return err
	}

	if err := database.DB.Save(&recipe).Error; err != nil {
		return err
	}
	return c.JSON(recipe)
}

func DeleteRecipe(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&domain.Recipe{}, id).Error; err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func ExploreRecipes(c *fiber.Ctx) error {
	var recipes []domain.Recipe
	if err := database.DB.Model(&domain.Recipe{}).
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		Find(&recipes).Error; err != nil {
		return err
	}
	return c.JSON(recipes)
}

func SaveRecipe(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return err
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	// #nosec G115
	if err := database.DB.Model(&domain.User{ID: tokenData.ID}).Association("Recipes").Append(&domain.Recipe{ID: uint64(id)}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusAccepted)
}

func UnsaveRecipe(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return err
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	// #nosec G115
	if err := database.DB.Model(&domain.User{ID: tokenData.ID}).Association("Recipes").Delete(&domain.Recipe{ID: uint64(id)}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusAccepted)
}
