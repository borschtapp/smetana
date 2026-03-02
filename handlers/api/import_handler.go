package api

import (
	"errors"
	"net/http"

	"borscht.app/smetana/domain"
	sErrors "borscht.app/smetana/pkg/sentinels"
	"borscht.app/smetana/pkg/utils"
	"github.com/gofiber/fiber/v3"
)

type ImportHandler struct {
	recipeService domain.RecipeService
}

func NewImportHandler(recipeService domain.RecipeService) *ImportHandler {
	return &ImportHandler{
		recipeService: recipeService,
	}
}

type ImportRequest struct {
	URL    string `json:"url" validate:"required"`
	Update bool   `json:"update"`
}

// Import godoc
// @Summary Import a recipe from URL.
// @Description Backend attempts semantic extraction first, then AI extraction. Returns the imported Recipe object.
// @Tags recipes
// @Accept json
// @Produce json
// @Param import body ImportRequest true "Import request"
// @Success 201 {object} domain.Recipe
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/import [post]
func (h *ImportHandler) Import(c fiber.Ctx) error {
	var request ImportRequest
	if err := c.Bind().Body(&request); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	// Check if recipe already exists by URL
	recipeByUrl, err := h.recipeService.ByUrl(request.URL)
	if err != nil && !errors.Is(err, domain.ErrRecordNotFound) {
		return err
	}
	if recipeByUrl != nil {
		if !request.Update {
			// Recipe exists, just add to user's recipes
			if err := h.recipeService.UserSave(tokenData.ID, recipeByUrl.ID); err != nil {
				return err
			}
			return c.JSON(recipeByUrl)
		} else {
			// TODO: the recipe should not be deleted, but updated instead, only if user has permission to do that. If not, create a duplicate
			// Delete existing to re-import
			if err := h.recipeService.Delete(recipeByUrl.ID); err != nil {
				return err
			}
		}
	}

	// Import recipe from URL
	recipe, err := h.recipeService.ImportFromURL(request.URL)
	if err != nil {
		return err
	}

	// Add to user's recipes
	if err := h.recipeService.UserSave(tokenData.ID, recipe.ID); err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}
