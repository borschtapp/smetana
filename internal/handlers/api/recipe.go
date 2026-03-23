package api

import (
	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
	"github.com/gofiber/fiber/v3"
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
// @Param publishers query string false "Comma-separated publisher IDs to filter by"
// @Param authors query string false "Comma-separated author IDs to filter by"
// @Param equipment query string false "Comma-separated equipment IDs to filter by"
// @Param cook_time_max query int false "Max cook time in seconds (e.g. 1800 = 30 min)"
// @Param total_time_max query int false "Max total time in seconds (e.g. 3600 = 1 hour)"
// @Param preload query string false "Comma-separated extras to include: publisher, author, feed, images, ingredients, instructions, nutrition, taxonomies, collections and saved"
// @Param sort query string false "Sort by field: id, name, created, updated (default: id)"
// @Param order query string false "Sort order: asc or desc (default: desc)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
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
	if err := opts.Validate("publisher", "author", "feed", "images", "ingredients", "equipment", "instructions", "nutrition", "taxonomies", "collections", "saved"); err != nil {
		return err
	}

	recipes, total, err := h.recipeService.Search(tokenData.ID, tokenData.HouseholdID, domain.RecipeSearchOptions{SearchOptions: opts})
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	recipe, err := h.recipeService.ByIDPreload(id, tokenData.ID, tokenData.HouseholdID, types.Preload("all"))
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
	if err := bindBody(c, recipe); err != nil {
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	var recipe domain.Recipe
	if err := bindBody(c, &recipe); err != nil {
		return err
	}
	recipe.ID = id

	if err := h.recipeService.Update(&recipe, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}

	// Update may have cloned the recipe (copy-on-write), so recipe.ID may now point to the cloned record
	updated, err := h.recipeService.ByIDPreload(recipe.ID, tokenData.ID, tokenData.HouseholdID, types.Preload("all"))
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.UserSave(id, tokenData.ID, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := bindBody(c, ingredient); err != nil {
		return err
	}
	ingredient.RecipeID = id

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
	id, ingredientID, err := types.UuidParams(c, "id", "ingredientId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := bindBody(c, ingredient); err != nil {
		return err
	}
	ingredient.ID = ingredientID
	ingredient.RecipeID = id

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
	id, ingredientID, err := types.UuidParams(c, "id", "ingredientId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteIngredient(ingredientID, id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// AddEquipment godoc
// @Summary Add equipment to a recipe.
// @Description Associate existing equipment with a recipe.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Param equipmentId path string true "Equipment ID"
// @Success 201
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/equipment/{equipmentId} [post]
func (h *RecipeHandler) AddEquipment(c fiber.Ctx) error {
	id, equipmentID, err := types.UuidParams(c, "id", "equipmentId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.AddEquipment(id, equipmentID, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// RemoveEquipment godoc
// @Summary Remove equipment from a recipe.
// @Description Remove equipment association from a recipe.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Param equipmentId path string true "Equipment ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/equipment/{equipmentId} [delete]
func (h *RecipeHandler) RemoveEquipment(c fiber.Ctx) error {
	id, equipmentID, err := types.UuidParams(c, "id", "equipmentId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.RemoveEquipment(id, equipmentID, tokenData.HouseholdID); err != nil {
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
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := bindBody(c, instruction); err != nil {
		return err
	}
	instruction.RecipeID = id

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
	id, instructionID, err := types.UuidParams(c, "id", "instructionId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := bindBody(c, instruction); err != nil {
		return err
	}
	instruction.ID = instructionID
	instruction.RecipeID = id

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
	id, instructionID, err := types.UuidParams(c, "id", "instructionId")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteInstruction(instructionID, id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
