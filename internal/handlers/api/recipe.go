package api

import (
	"borscht.app/smetana/internal/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type RecipeHandler struct {
	recipeService domain.RecipeService
}

func NewRecipeHandler(recipeService domain.RecipeService) *RecipeHandler {
	return &RecipeHandler{recipeService: recipeService}
}

// Search godoc
// @Summary Search recipes.
// @Description Query user's recipes by text, tags. Supports full-text search on name/description.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param q query string false "Text search"
// @Param taxonomies query string false "Comma-separated taxonomy IDs to filter by (using OR logic)"
// @Param preload query string false "Comma-separated extras to include: publisher, feed, images, ingredients, instructions, taxonomies, collections and saved"
// @param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @param order query string false "Sort order: asc or desc (default: desc)"
// @Param page query int false "Page number"
// @param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Recipe]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes [get]
func (h *RecipeHandler) Search(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	opts, err := types.GetSearchOptions(c)
	if err != nil {
		return err
	}

	recipes, total, err := h.recipeService.Search(tokenData.ID, tokenData.HouseholdID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Total: int(total),
			Page:  opts.Page,
		},
	})
}

// GetRecipe godoc
// @Summary Return details of a specific recipe by its ID.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 200 {object} domain.Recipe
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [get]
func (h *RecipeHandler) GetRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	recipe, err := h.recipeService.ByID(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	return c.JSON(recipe)
}

// CreateRecipe godoc
// @Summary Create a new recipe.
// @Description Create a new recipe from JSON body. The recipe is automatically saved for the creator.
// @Tags recipes
// @Accept json
// @Produce json
// @Param recipe body domain.Recipe true "Recipe data"
// @Success 201 {object} domain.Recipe
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes [post]
func (h *RecipeHandler) CreateRecipe(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	recipe := new(domain.Recipe)
	if err := c.Bind().Body(&recipe); err != nil {
		return err
	}

	if err := h.recipeService.Create(recipe, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(recipe)
}

// UpdateRecipe godoc
// @Summary Update a recipe.
// @Description Update an existing recipe. Allows users to correct details.
// @Tags recipes
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param recipe body domain.Recipe true "Updated recipe data"
// @Success 200 {object} domain.Recipe
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [patch]
func (h *RecipeHandler) UpdateRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	var recipe domain.Recipe
	if err := c.Bind().Body(&recipe); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	recipe.ID = id

	if err := h.recipeService.Update(&recipe, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}

	// Update may have cloned the recipe (copy-on-write), so recipe.ID may now point to the cloned record
	updated, err := h.recipeService.ByID(recipe.ID, tokenData.HouseholdID)
	if err != nil {
		return err
	}
	return c.JSON(updated)
}

// DeleteRecipe godoc
// @Summary Delete a recipe.
// @Description Delete a specific recipe by ID.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [delete]
func (h *RecipeHandler) DeleteRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.Delete(id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// SaveRecipe godoc
// @Summary Save a recipe.
// @Description Adds the recipe to the user's personal "Favorites" list.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 201
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/favorite [post]
func (h *RecipeHandler) SaveRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.UserSave(id, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusCreated)
}

// UnsaveRecipe godoc
// @Summary Remove a recipe from the user's collection.
// @Description Remove a recipe from the "Favorites" list.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/favorite [delete]
func (h *RecipeHandler) UnsaveRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.UserUnsave(id, tokenData.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// CreateIngredient godoc
// @Summary Create a recipe ingredient.
// @Description Add a new ingredient to a recipe.
// @Tags recipes
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param ingredient body domain.RecipeIngredient true "Ingredient data"
// @Success 201 {object} domain.RecipeIngredient
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/ingredients [post]
func (h *RecipeHandler) CreateIngredient(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := c.Bind().Body(&ingredient); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	ingredient.RecipeID = recipeID

	if err := h.recipeService.CreateIngredient(ingredient, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(ingredient)
}

// UpdateIngredient godoc
// @Summary Update a recipe ingredient.
// @Description Update an existing recipe ingredient.
// @Tags recipes
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param ingredientId path string true "Ingredient ID"
// @Param ingredient body domain.RecipeIngredient true "Updated ingredient data"
// @Success 200 {object} domain.RecipeIngredient
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/ingredients/{ingredientId} [patch]
func (h *RecipeHandler) UpdateIngredient(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	ingredientID, err := uuid.Parse(c.Params("ingredientId"))
	if err != nil {
		return sentinels.BadRequest("invalid ingredient id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := c.Bind().Body(&ingredient); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	ingredient.ID = ingredientID
	ingredient.RecipeID = recipeID

	if err := h.recipeService.UpdateIngredient(ingredient, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.JSON(ingredient)
}

// DeleteIngredient godoc
// @Summary Delete a recipe ingredient.
// @Description Delete a specific recipe ingredient.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Param ingredientId path string true "Ingredient ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/ingredients/{ingredientId} [delete]
func (h *RecipeHandler) DeleteIngredient(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	ingredientID, err := uuid.Parse(c.Params("ingredientId"))
	if err != nil {
		return sentinels.BadRequest("invalid ingredient id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteIngredient(ingredientID, recipeID, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// CreateInstruction godoc
// @Summary Create a recipe instruction.
// @Description Add a new instruction to a recipe.
// @Tags recipes
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param instruction body domain.RecipeInstruction true "Instruction data"
// @Success 201 {object} domain.RecipeInstruction
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/instructions [post]
func (h *RecipeHandler) CreateInstruction(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := c.Bind().Body(&instruction); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	instruction.RecipeID = recipeID

	if err := h.recipeService.CreateInstruction(instruction, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(instruction)
}

// UpdateInstruction godoc
// @Summary Update a recipe instruction.
// @Description Update an existing recipe instruction.
// @Tags recipes
// @Accept json
// @Produce json
// @Param id path string true "Recipe ID"
// @Param instructionId path string true "Instruction ID"
// @Param instruction body domain.RecipeInstruction true "Updated instruction data"
// @Success 200 {object} domain.RecipeInstruction
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/instructions/{instructionId} [patch]
func (h *RecipeHandler) UpdateInstruction(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	instructionID, err := uuid.Parse(c.Params("instructionId"))
	if err != nil {
		return sentinels.BadRequest("invalid instruction id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := c.Bind().Body(&instruction); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	instruction.ID = instructionID
	instruction.RecipeID = recipeID

	if err := h.recipeService.UpdateInstruction(instruction, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.JSON(instruction)
}

// DeleteInstruction godoc
// @Summary Delete a recipe instruction.
// @Description Delete a specific recipe instruction.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Param instructionId path string true "Instruction ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/instructions/{instructionId} [delete]
func (h *RecipeHandler) DeleteInstruction(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	instructionID, err := uuid.Parse(c.Params("instructionId"))
	if err != nil {
		return sentinels.BadRequest("invalid instruction id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteInstruction(instructionID, recipeID, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
