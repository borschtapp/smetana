package api

import (
	"net/http"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/sentinels"
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
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(request); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	recipe, err := h.recipeService.ImportForUser(request.URL, tokenData.ID, request.Update)
	if err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}
