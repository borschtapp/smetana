package api

import (
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
)

func GetRecipes(c *fiber.Ctx) error {
	var recipes []domain.Recipe
	if err := database.DB.Find(&recipes).Error; err != nil {
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
	if err := database.DB.First(&recipe, uint(id)).Error; err != nil {
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

	if err := database.DB.Delete(&domain.Recipe{}, uint(id)).Error; err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
