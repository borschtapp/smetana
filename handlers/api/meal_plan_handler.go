package api

import (
	"errors"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	sErrors "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
)

// GetMealPlan godoc
// @Summary List meal plan entries.
// @Description Returns meal plan entries for the current household. Supports date range filtering.
// @Tags mealplan
// @Accept */*
// @Produce json
// @Param from query string false "Start date (YYYY-MM-DD)"
// @Param to query string false "End date (YYYY-MM-DD)"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.MealPlan]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan [get]
func GetMealPlan(c fiber.Ctx) error {
	fromStr := c.Query("from")
	toStr := c.Query("to")

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	p := types.GetPagination(c)
	query := database.DB.Preload("Recipe").Where("household_id = ?", user.HouseholdID)

	if fromStr != "" {
		if from, err := time.Parse("2006-01-02", fromStr); err == nil {
			query = query.Where("date >= ?", from)
		}
	}
	if toStr != "" {
		if to, err := time.Parse("2006-01-02", toStr); err == nil {
			query = query.Where("date <= ?", to)
		}
	}

	var total int64
	query.Model(&domain.MealPlan{}).Count(&total)

	var mealPlans []domain.MealPlan
	if err := query.Offset(p.Offset()).Limit(p.Limit).Find(&mealPlans).Error; err != nil {
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan [post]
func CreateMealPlan(c fiber.Ctx) error {
	var form MealPlanForm
	if err := c.Bind().Body(&form); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	validate := validator.New()
	if err := validate.Struct(form); err != nil {
		return sErrors.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	mealPlan := &domain.MealPlan{
		HouseholdID: user.HouseholdID,
		Date:        form.Date,
		MealType:    form.MealType,
		RecipeID:    form.RecipeID,
		Servings:    form.Servings,
		Note:        form.Note,
	}

	if err := database.DB.Create(mealPlan).Error; err != nil {
		return err
	}

	// Preload recipe if provided
	if mealPlan.RecipeID != nil {
		database.DB.Preload("Recipe").First(mealPlan)
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan/{id} [patch]
func UpdateMealPlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid meal plan id")
	}

	var form UpdateMealPlanForm
	if err := c.Bind().Body(&form); err != nil {
		return sErrors.BadRequest(err.Error())
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	var mealPlan domain.MealPlan
	if err := database.DB.First(&mealPlan, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("meal plan not found")
		}
		return err
	}

	if mealPlan.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("meal plan does not belong to your household")
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

	if err := database.DB.Save(&mealPlan).Error; err != nil {
		return err
	}

	if mealPlan.RecipeID != nil {
		database.DB.Preload("Recipe").First(&mealPlan)
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/mealplan/{id} [delete]
func DeleteMealPlan(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid meal plan id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	var mealPlan domain.MealPlan
	if err := database.DB.First(&mealPlan, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("meal plan not found")
		}
		return err
	}

	if mealPlan.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("meal plan does not belong to your household")
	}

	if err := database.DB.Delete(&mealPlan).Error; err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
