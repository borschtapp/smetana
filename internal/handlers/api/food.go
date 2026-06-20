package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/types"
)

type FoodHandler struct {
	service domain.FoodService
}

func NewFoodHandler(service domain.FoodService) *FoodHandler {
	return &FoodHandler{service: service}
}

// GetFoods godoc
// @Summary Search foods by name
// @Description Search canonical foods by name or slug with pagination.
// @Tags food
// @Produce json
// @Param q query string false "Search query (matches name or slug)"
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.Food]
// @Failure 401 {object} sentinels.Error
// @Router /api/v1/food [get]
// @Security ApiKeyAuth
func (h *FoodHandler) GetFoods(c fiber.Ctx) error {
	query := c.Query("q")
	p := types.GetPagination(c)

	foods, total, err := h.service.Search(query, p.Offset, p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Food]{
		Data: foods,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}

type updateFoodRequest struct {
	Name          *string    `json:"name"           validate:"omitempty,min=1,max=255"`
	Description   *string    `json:"description"    validate:"omitempty,max=1000"`
	DefaultUnitID *uuid.UUID `json:"default_unit_id"`
	Pantry        *bool      `json:"pantry"`
}

// UpdateFood godoc
// @Summary Update a food
// @Description Update name, description, default unit, or pantry status of a food. Only provided fields are changed.
// @Tags food
// @Accept json
// @Produce json
// @Param id path string true "Food UUID"
// @Param food body updateFoodRequest true "Fields to update"
// @Success 200 {object} domain.Food
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id} [patch]
// @Security ApiKeyAuth
func (h *FoodHandler) UpdateFood(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req updateFoodRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	food, err := h.service.ByID(id)
	if err != nil {
		return err
	}

	if req.Name != nil {
		food.Name = *req.Name
	}
	if req.Description != nil {
		food.Description = req.Description
	}
	if req.DefaultUnitID != nil {
		food.DefaultUnitID = req.DefaultUnitID
	}
	if req.Pantry != nil {
		food.Pantry = *req.Pantry
	}

	if err := h.service.Update(food); err != nil {
		return err
	}
	return c.JSON(food)
}

// MergeFood godoc
// @Summary Merge two food items
// @Description Reassigns all ingredients, shopping items, and prices from the food at {id} to merge_into, then marks {id} as an alias so future imports resolve correctly.
// @Tags food
// @Accept json
// @Produce json
// @Param id path string true "Food UUID to merge away (becomes alias)"
// @Param body body mergeRequest true "Target food to keep"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/merge [post]
// @Security ApiKeyAuth
func (h *FoodHandler) MergeFood(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req mergeRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	if err := h.service.Merge(req.MergeInto, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetPrice godoc
// @Summary Get price history for a food
// @Description Get paginated price history for food within the household.
// @Tags food
// @Produce json
// @Param id path string true "Food UUID"
// @Success 200 {object} types.ListResponse[domain.FoodPrice]
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/price [get]
// @Security ApiKeyAuth
func (h *FoodHandler) GetPrice(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	opts := types.GetPagination(c)

	prices, total, err := h.service.ListPrices(tokenData.HouseholdID, foodID, opts)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.FoodPrice]{
		Data: prices,
		Meta: types.Meta{
			Pagination: opts,
			Total:      int(total),
		},
	})
}

type recordPriceRequest struct {
	Price  float64   `json:"price"   validate:"required,gt=0"`
	Amount float64   `json:"amount"  validate:"required,gt=0"`
	UnitID uuid.UUID `json:"unit_id" validate:"required"`
}

// RecordPrice godoc
// @Summary Record a new price observation for a food
// @Description Record a new price observation for food within the household. Price is expressed as: Price <Currency> per Amount <Unit>. Example: 4.99 EUR per 1 kg of chicken breast.
// @Tags food
// @Accept json
// @Produce json
// @Param id path string true "Food UUID"
// @Param price body recordPriceRequest true "Price observation"
// @Success 201 {object} domain.FoodPrice
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/price [post]
// @Security ApiKeyAuth
func (h *FoodHandler) RecordPrice(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req recordPriceRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	price := &domain.FoodPrice{
		FoodID: foodID,
		UnitID: req.UnitID,
		Price:  req.Price,
		Amount: req.Amount,
	}
	if err := h.service.RecordPrice(tokenData.HouseholdID, price); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(price)
}

// DeletePrice godoc
// @Summary Delete a food price observation
// @Tags food
// @Param id path string true "Food UUID"
// @Param priceId path string true "FoodPrice UUID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/price/{priceId} [delete]
// @Security ApiKeyAuth
func (h *FoodHandler) DeletePrice(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	id, err := types.UuidParam(c, "priceId")
	if err != nil {
		return err
	}

	if err := h.service.DeletePrice(tokenData.HouseholdID, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
