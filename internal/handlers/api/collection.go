package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
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
// @Param q query string false "Text search"
// @Param preload query string false "Comma-separated extras to include: recipes:5, recipes.images and total_recipes"
// @Param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Collection]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections [get]
func (h *CollectionHandler) GetCollections(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	collections, total, err := h.collectionService.Search(tokenData.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Collection]{
		Data: collections,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
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
	if err := bindBody(c, &form); err != nil {
		return err
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var form UpdateCollectionForm
	if err := bindBody(c, &form); err != nil {
		return err
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

// ListRecipes godoc
// @Summary List recipes in a collection.
// @Description Returns recipes in a collection with optional search, pagination and extras.
// @Tags collections
// @Accept json
// @Produce json
// @Param q query string false "Text search"
// @Param preload query string false "Comma-separated extras to include: publisher, feed, images, ingredients, instructions, taxonomies, collections and saved"
// @Param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Recipe]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/collections/{id}/recipes [get]
func (h *CollectionHandler) ListRecipes(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	claims, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	recipes, total, err := h.collectionService.ListRecipes(id, claims.ID, claims.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Pagination: opts.Pagination,
			Total:      int(total),
		},
	})
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
	id, recipeID, err := types.UuidParams(c, "id", "recipeId")
	if err != nil {
		return err
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
	id, recipeID, err := types.UuidParams(c, "id", "recipeId")
	if err != nil {
		return err
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
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
