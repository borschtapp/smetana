package api

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	sErrors "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/services"
	"borscht.app/smetana/pkg/utils"
)

type ScrapeHandler struct {
	recipeService *services.RecipeService
}

func NewScrapeHandler(recipeService *services.RecipeService) *ScrapeHandler {
	return &ScrapeHandler{
		recipeService: recipeService,
	}
}

type ImportRequest struct {
	URL    string `json:"url" validate:"required"`
	Update bool   `json:"update"`
}

// Scrape godoc
// @Summary Import a recipe from URL.
// @Description Backend attempts semantic extraction first, then AI extraction. Returns the imported Recipe object.
// @Tags recipes
// @Accept json
// @Produce json
// @Param import body ImportRequest true "Import request"
// @Success 201 {object} domain.Recipe
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/recipes/import [post]
func (h *ScrapeHandler) Scrape(c fiber.Ctx) error {
	var request ImportRequest
	if err := c.Bind().Body(&request); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	// Check if recipe already exists by URL
	var recipeByUrl domain.Recipe
	if err := database.DB.Where(&domain.Recipe{IsBasedOn: &request.URL}).
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipeByUrl).Error; err == nil {
		if !request.Update {
			// Recipe exists, just add to user's recipes
			if err := database.DB.Model(&domain.User{ID: tokenData.ID}).Association("Recipes").Append(&domain.Recipe{ID: recipeByUrl.ID}); err != nil {
				return err
			}
			return c.JSON(recipeByUrl)
		} else {
			// TODO: the recipe should not be deleted, but updated instead, only if user has permission to do that. If not, create a duplicate
			// Delete existing to re-import
			if err := database.DB.Delete(&recipeByUrl).Error; err != nil {
				return err
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Import recipe from URL
	recipe, err := h.recipeService.ImportFromUrl(request.URL)
	if err != nil {
		return err
	}

	// Add to user's recipes
	if err := database.DB.Model(&domain.User{ID: tokenData.ID}).Association("Recipes").Append(&domain.Recipe{ID: recipe.ID}); err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}
