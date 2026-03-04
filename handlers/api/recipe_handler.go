package api

import (
	"strings"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/sentinels"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type RecipeHandler struct {
	recipeService domain.RecipeService
}

func NewRecipeHandler(recipeService domain.RecipeService) *RecipeHandler {
	return &RecipeHandler{recipeService: recipeService}
}

// GetRecipes godoc
// @Summary Search recipes.
// @Description Query user's recipes by text, tags, or cuisine. Supports full-text search on name/description. Taxonomies are comma-separated and used with OR logic.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param q query string false "Text search"
// @Param taxonomies query string false "Comma-separated taxonomy labels"
// @Param cuisine query string false "Cuisine filter (slug)"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Recipe]
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes [get]
func (h *RecipeHandler) GetRecipes(c fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	q := c.Query("q")
	cuisine := c.Query("cuisine")
	taxonomiesStr := c.Query("taxonomies")
	var taxonomies []string
	if taxonomiesStr != "" {
		taxonomies = strings.Split(taxonomiesStr, ",")
	}

	p := types.GetPagination(c)

	recipes, total, err := h.recipeService.UserSearch(tokenData.ID, q, taxonomies, cuisine, p.Offset(), p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Recipe]{
		Data: recipes,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
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
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [get]
func (h *RecipeHandler) GetRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	recipe, err := h.recipeService.ById(id, tokenData.ID)
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes [post]
func (h *RecipeHandler) CreateRecipe(c fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	recipe := new(domain.Recipe)
	if err := c.Bind().Body(&recipe); err != nil {
		return err
	}

	if err := h.recipeService.Create(recipe, tokenData.ID); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [patch]
func (h *RecipeHandler) UpdateRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	// Verify access before accepting the update body
	if _, err := h.recipeService.ById(id, tokenData.ID); err != nil {
		return err
	}

	var recipe domain.Recipe
	if err := c.Bind().Body(&recipe); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	recipe.ID = id

	if err := h.recipeService.Update(&recipe, tokenData.ID); err != nil {
		return err
	}
	return c.JSON(recipe)
}

// DeleteRecipe godoc
// @Summary Delete a recipe.
// @Description Delete a specific recipe by ID.
// @Tags recipes
// @Accept */*
// @Produce json
// @Param id path string true "Recipe ID"
// @Success 204
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id} [delete]
func (h *RecipeHandler) DeleteRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.Delete(id, tokenData.ID); err != nil {
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
// @Failure 401 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/favorite [post]
func (h *RecipeHandler) SaveRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return err
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.UserSave(tokenData.ID, id); err != nil {
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
// @Failure 401 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/favorite [delete]
func (h *RecipeHandler) UnsaveRecipe(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return err
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.UserUnsave(tokenData.ID, id); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/ingredients [post]
func (h *RecipeHandler) CreateIngredient(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := c.Bind().Body(&ingredient); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	ingredient.RecipeID = recipeID

	if err := h.recipeService.CreateIngredient(ingredient, tokenData.ID); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
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

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	ingredient := new(domain.RecipeIngredient)
	if err := c.Bind().Body(&ingredient); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	ingredient.ID = ingredientID
	ingredient.RecipeID = recipeID

	if err := h.recipeService.UpdateIngredient(ingredient, tokenData.ID); err != nil {
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
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
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

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteIngredient(ingredientID, recipeID, tokenData.ID); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/recipes/{id}/instructions [post]
func (h *RecipeHandler) CreateInstruction(c fiber.Ctx) error {
	recipeID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid recipe id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := c.Bind().Body(&instruction); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	instruction.RecipeID = recipeID

	if err := h.recipeService.CreateInstruction(instruction, tokenData.ID); err != nil {
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
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
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

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	instruction := new(domain.RecipeInstruction)
	if err := c.Bind().Body(&instruction); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	instruction.ID = instructionID
	instruction.RecipeID = recipeID

	if err := h.recipeService.UpdateInstruction(instruction, tokenData.ID); err != nil {
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
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
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

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.recipeService.DeleteInstruction(instructionID, recipeID, tokenData.ID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
