package api

import (
	"time"

	"borscht.app/smetana/internal/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type MealPlanHandler struct {
	mealPlanService domain.MealPlanService
}

func NewMealPlanHandler(mealPlanService domain.MealPlanService) *MealPlanHandler {
	return &MealPlanHandler{
		mealPlanService: mealPlanService,
	}
}

// GetMealPlan godoc
// @Summary List meal plan entries.
// @Description Returns meal plan entries for the current household. Supports date range filtering.
// @Tags mealplan
// @Accept */*
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param page query int false "Page number"
// @param offset query int false "Offset for pagination (alternative to page)"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.MealPlan]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan [get]
func (h *MealPlanHandler) GetMealPlan(c fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	var from, to *time.Time
	if fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = &t
		}
	}
	if toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = &t
		}
	}

	p := types.GetPagination(c)
	mealPlans, total, err := h.mealPlanService.List(tokenData.HouseholdID, from, to, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.MealPlan]{
		Data: mealPlans,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

type MealPlanForm struct {
	Date     time.Time  `validate:"required" json:"date" swaggertype:"string" format:"date" example:"2024-12-25"`
	MealType string     `validate:"required,oneof=breakfast lunch dinner" json:"meal_type" enums:"breakfast,lunch,dinner" example:"dinner"`
	RecipeID *uuid.UUID `json:"recipe_id"`
	Servings *int       `validate:"omitempty,min=1" json:"servings" example:"4"`
	Note     *string    `json:"note"`
}

// CreateMealPlan godoc
// @Summary Schedule a meal.
// @Description Adds a new entry to the household's meal plan.
// @Tags mealplan
// @Accept json
// @Produce json
// @Param mealplan body MealPlanForm true "Meal plan data"
// @Success 201 {object} domain.MealPlan
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan [post]
func (h *MealPlanHandler) CreateMealPlan(c fiber.Ctx) error {
	var form MealPlanForm
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

	mealPlan := &domain.MealPlan{
		Date:     form.Date,
		MealType: form.MealType,
		RecipeID: form.RecipeID,
		Servings: form.Servings,
		Note:     form.Note,
	}

	if err := h.mealPlanService.Create(mealPlan, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(mealPlan)
}

type UpdateMealPlanForm struct {
	Date     *time.Time `json:"date" swaggertype:"string" format:"date" example:"2024-12-26"`
	MealType *string    `validate:"omitempty,oneof=breakfast lunch dinner" json:"meal_type" enums:"breakfast,lunch,dinner" example:"lunch"`
	RecipeID *uuid.UUID `json:"recipe_id"`
	Servings *int       `validate:"omitempty,min=1" json:"servings" example:"2"`
	Note     *string    `json:"note"`
}

// UpdateMealPlan godoc
// @Summary Reschedule a meal.
// @Description Update an existing meal plan entry (e.g. change date, servings, or note).
// @Tags mealplan
// @Accept json
// @Produce json
// @Param id path string true "Meal Plan ID"
// @Param mealplan body UpdateMealPlanForm true "Meal plan update data"
// @Success 200 {object} domain.MealPlan
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan/{id} [patch]
func (h *MealPlanHandler) UpdateMealPlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid meal plan id")
	}

	var form UpdateMealPlanForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	mealPlan, err := h.mealPlanService.ByIDWithRecipes(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	if form.Date != nil {
		mealPlan.Date = *form.Date
	}
	if form.MealType != nil {
		mealPlan.MealType = *form.MealType
	}
	if form.RecipeID != nil {
		mealPlan.RecipeID = form.RecipeID
	}
	if form.Servings != nil {
		mealPlan.Servings = form.Servings
	}
	if form.Note != nil {
		mealPlan.Note = form.Note
	}

	if err := h.mealPlanService.Update(mealPlan, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.JSON(mealPlan)
}

// DeleteMealPlan godoc
// @Summary Cancel a meal.
// @Description Remove a meal from the plan.
// @Tags mealplan
// @Accept */*
// @Produce json
// @Param id path string true "Meal Plan ID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 403 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan/{id} [delete]
func (h *MealPlanHandler) DeleteMealPlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid meal plan id")
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.mealPlanService.Delete(id, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
