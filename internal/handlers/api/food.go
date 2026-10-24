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
	Name          *string                   `json:"name"            validate:"omitnil,min=1,max=255"`
	Description   types.Optional[string]    `json:"description"     validate:"omitnil,max=1000" swaggertype:"string" extensions:"x-nullable"` // null clears it
	DefaultUnitID types.Optional[uuid.UUID] `json:"default_unit_id" swaggertype:"string" format:"uuid" extensions:"x-nullable"`               // null clears it
	Pantry        *bool                     `json:"pantry"`
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
	if req.Description.Set {
		food.Description = req.Description.Value
	}
	if req.DefaultUnitID.Set {
		food.DefaultUnitID = req.DefaultUnitID.Value
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
	tokenData := tokens.MustClaims(c)
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
	tokenData := tokens.MustClaims(c)
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
	tokenData := tokens.MustClaims(c)
	id, err := types.UuidParam(c, "priceId")
	if err != nil {
		return err
	}

	if err := h.service.DeletePrice(tokenData.HouseholdID, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// GetConversions godoc
// @Summary Get conversions for a food
// @Description Get all unit conversions associated with a food.
// @Tags food
// @Produce json
// @Param id path string true "Food UUID"
// @Success 200 {array} domain.FoodUnitConversion
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/conversions [get]
// @Security ApiKeyAuth
func (h *FoodHandler) GetConversions(c fiber.Ctx) error {
	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	conversions, err := h.service.ListConversions(foodID)
	if err != nil {
		return err
	}
	return c.JSON(conversions)
}

type createConversionRequest struct {
	UnitID       uuid.UUID `json:"unit_id" validate:"required"`
	TargetAmount float64   `json:"target_amount" validate:"required,gt=0"`
	TargetUnitID uuid.UUID `json:"target_unit_id" validate:"required"`
}

type updateConversionRequest struct {
	TargetAmount float64   `json:"target_amount" validate:"required,gt=0"`
	TargetUnitID uuid.UUID `json:"target_unit_id" validate:"required"`
}

// CreateConversion godoc
// @Summary Create a new conversion for a food
// @Description Creates a new unit conversion for a food (e.g., 1 piece = 200 g).
// @Tags food
// @Accept json
// @Produce json
// @Param id path string true "Food UUID"
// @Param conversion body createConversionRequest true "Conversion request"
// @Success 201 {object} domain.FoodUnitConversion
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 409 {object} sentinels.Error
// @Router /api/v1/food/{id}/conversions [post]
// @Security ApiKeyAuth
func (h *FoodHandler) CreateConversion(c fiber.Ctx) error {
	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	var req createConversionRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	conversion := &domain.FoodUnitConversion{
		FoodID:       foodID,
		UnitID:       req.UnitID,
		TargetAmount: req.TargetAmount,
		TargetUnitID: req.TargetUnitID,
	}

	if err := h.service.CreateConversion(conversion); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(conversion)
}

// UpdateConversion godoc
// @Summary Update an existing conversion for a food
// @Description Updates target amount and target unit of an existing conversion. The source unit (unit_id) is immutable after creation.
// @Tags food
// @Accept json
// @Param id path string true "Food UUID"
// @Param conversionId path string true "Conversion UUID"
// @Param conversion body updateConversionRequest true "Conversion update request"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/conversions/{conversionId} [put]
// @Security ApiKeyAuth
func (h *FoodHandler) UpdateConversion(c fiber.Ctx) error {
	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}
	conversionID, err := types.UuidParam(c, "conversionId")
	if err != nil {
		return err
	}

	var req updateConversionRequest
	if err := bindBody(c, &req); err != nil {
		return err
	}

	conversion := &domain.FoodUnitConversion{
		ID:           conversionID,
		FoodID:       foodID,
		TargetAmount: req.TargetAmount,
		TargetUnitID: req.TargetUnitID,
	}

	if err := h.service.UpdateConversion(conversion); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// DeleteConversion godoc
// @Summary Delete a food unit conversion
// @Tags food
// @Param id path string true "Food UUID"
// @Param conversionId path string true "Conversion UUID"
// @Success 204
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Router /api/v1/food/{id}/conversions/{conversionId} [delete]
// @Security ApiKeyAuth
func (h *FoodHandler) DeleteConversion(c fiber.Ctx) error {
	foodID, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}
	conversionID, err := types.UuidParam(c, "conversionId")
	if err != nil {
		return err
	}

	if err := h.service.DeleteConversion(foodID, conversionID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
