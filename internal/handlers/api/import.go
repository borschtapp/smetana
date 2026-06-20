package api

import (
	"net/http"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"github.com/gofiber/fiber/v3"
)

type ImportHandler struct {
	importService domain.ImportService
}

func NewImportHandler(importService domain.ImportService) *ImportHandler {
	return &ImportHandler{
		importService: importService,
	}
}

type ImportRequest struct {
	URL    string `json:"url" validate:"required,url"`
	Update bool   `json:"update"`
	Type   string `json:"type" validate:"omitempty,oneof=auto recipe feed"` // "auto", "recipe", "feed"
}

// FIXME: this method is deprecated and should be removed in the future
// Import godoc
// @Summary Import a recipe from URL.
// @Description Backend attempts semantic extraction first, then AI extraction. Returns the imported Recipe object.
// @Tags recipes
// @Accept json
// @Produce json
// @Param import body ImportRequest true "Import request"
// @Success 201 {object} domain.Recipe
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 422 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/import [post]
func (h *ImportHandler) Import(c fiber.Ctx) error {
	var request ImportRequest
	if err := bindBody(c, &request); err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	recipe, err := h.importService.ImportFromURL(c.Context(), request.URL, request.Update, tokenData.ID, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}

// DetectAndImport godoc
// @Summary Import a recipe or subscribe to a feed from a URL.
// @Description Detects whether the URL points to a single recipe or a feed/listing. Returns the imported recipe or the feed subscription.
// @Tags import
// @Accept json
// @Produce json
// @Param import body ImportRequest true "Import request (type can be auto, recipe, or feed)"
// @Success 201 {object} domain.ImportResult
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/import [post]
func (h *ImportHandler) DetectAndImport(c fiber.Ctx) error {
	var request ImportRequest
	if err := bindBody(c, &request); err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	result, err := h.importService.DetectAndImport(c.Context(), request.URL, request.Type, request.Update, tokenData.ID, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	return c.Status(status).JSON(result)
}
