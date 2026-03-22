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

// GetPrice godoc
// @Summary Get price history for a food
// @Description Get paginated price history for food within the household.
// @Tags food
// @Produce json
// @Param id path string true "Food UUID"
// @Success 200 {object} types.ListResponse[domain.FoodPrice]
// @Router /food/{id}/price [get]
// @Security BearerAuth
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
// @Router /food/{id}/price [post]
// @Security BearerAuth
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
// @Router /food/{id}/price/{priceId} [delete]
// @Security BearerAuth
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
