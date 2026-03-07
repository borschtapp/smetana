package api

import (
	"borscht.app/smetana/internal/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type CollectionHandler struct {
	collectionService domain.CollectionService
}

func NewCollectionHandler(collectionService domain.CollectionService) *CollectionHandler {
	return &CollectionHandler{
		collectionService: collectionService,
	}
}

// GetCollections godoc
// @Summary List user's collections.
// @Description Returns collections associated with the user's household.
// @Tags collections
// @Accept */*
// @Produce json
// @Param page query int false "Page number"
// @param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Collection]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections [get]
func (h *CollectionHandler) GetCollections(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	p := types.GetPagination(c)
	collections, total, err := h.collectionService.List(tokenData.HouseholdID, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Collection]{
		Data: collections,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

type CollectionForm struct {
	Name        string `validate:"required,min=2" json:"name"`
	Description string `json:"description"`
}

// CreateCollection godoc
// @Summary Create a new collection.
// @Description Create a new recipe collection for the current household.
// @Tags collections
// @Accept json
// @Produce json
// @Param collection body CollectionForm true "Collection data"
// @Success 201 {object} domain.Collection
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections [post]
func (h *CollectionHandler) CreateCollection(c fiber.Ctx) error {
	var form CollectionForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(form); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	collection := &domain.Collection{
		Name:        form.Name,
		Description: form.Description,
	}

	if err := h.collectionService.Create(collection, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(collection)
}

// GetCollection godoc
// @Summary Returns a collection with all its recipes.
// @Tags collections
// @Accept */*
// @Produce json
// @Param id path string true "Collection ID"
// @Success 200 {object} domain.Collection
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id} [get]
func (h *CollectionHandler) GetCollection(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid collection id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	collection, err := h.collectionService.ByIDWithRecipes(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	return c.JSON(collection)
}

type UpdateCollectionForm struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

// UpdateCollection godoc
// @Summary Update collection.
// @Description Rename collection or manage its recipes.
// @Tags collections
// @Accept json
// @Produce json
// @Param id path string true "Collection ID"
// @Param collection body UpdateCollectionForm true "Collection update data"
// @Success 200 {object} domain.Collection
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id} [patch]
func (h *CollectionHandler) UpdateCollection(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid collection id")
	}

	var form UpdateCollectionForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	collection, err := h.collectionService.ByID(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}
	if form.Name != nil {
		collection.Name = *form.Name
	}
	if form.Description != nil {
		collection.Description = *form.Description
	}

	if err := h.collectionService.Update(collection, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.JSON(collection)
}

// AddRecipeToCollection godoc
// @Summary Add a recipe to a collection.
// @Tags collections
// @Param id path string true "Collection ID"
// @Param recipeId path string true "Recipe ID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id}/recipes/{recipeId} [post]
func (h *CollectionHandler) AddRecipeToCollection(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid collection id")
	}

	recipeID, err := uuid.Parse(c.Params("recipeId"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.collectionService.AddRecipe(id, recipeID, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveRecipeFromCollection godoc
// @Summary Remove a recipe from a collection.
// @Tags collections
// @Param id path string true "Collection ID"
// @Param recipeId path string true "Recipe ID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id}/recipes/{recipeId} [delete]
func (h *CollectionHandler) RemoveRecipeFromCollection(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid collection id")
	}

	recipeID, err := uuid.Parse(c.Params("recipeId"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.collectionService.RemoveRecipe(id, recipeID, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// DeleteCollection godoc
// @Summary Delete collection.
// @Description Delete a collection.
// @Tags collections
// @Accept */*
// @Produce json
// @Param id path string true "Collection ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id} [delete]
func (h *CollectionHandler) DeleteCollection(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid collection id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.collectionService.Delete(id, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
