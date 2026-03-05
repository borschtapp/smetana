package api

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type ShoppingListHandler struct {
	shoppingListService domain.ShoppingListService
}

func NewShoppingListHandler(shoppingListService domain.ShoppingListService) *ShoppingListHandler {
	return &ShoppingListHandler{
		shoppingListService: shoppingListService,
	}
}

// GetShoppingList godoc
// @Summary List shopping list items.
// @Description Returns all shopping list items for the current household.
// @Tags shoppinglist
// @Accept */*
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.ShoppingList]
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist [get]
func (h *ShoppingListHandler) GetShoppingList(c fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	p := types.GetPagination(c)
	items, total, err := h.shoppingListService.List(tokenData.HouseholdID, p.Offset(), p.Limit)
	if err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.ShoppingList]{
		Data: items,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

type ShoppingListForm struct {
	Name     string     `validate:"required" json:"name" example:"Milk"`
	Quantity *float64   `validate:"omitempty,gt=0" json:"quantity" example:"2"`
	UnitID   *uuid.UUID `json:"unit_id"`
}

// CreateShoppingListItem godoc
// @Summary Add item to shopping list.
// @Description Add a new item to the household shopping list.
// @Tags shoppinglist
// @Accept json
// @Produce json
// @Param item body ShoppingListForm true "Shopping list item data"
// @Success 201 {object} domain.ShoppingList
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist [post]
func (h *ShoppingListHandler) CreateShoppingListItem(c fiber.Ctx) error {
	var form ShoppingListForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	if err := validate.Struct(form); err != nil {
		return sentinels.BadRequestVal(err)
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	item := &domain.ShoppingList{
		Product:  form.Name,
		Quantity: form.Quantity,
		UnitID:   form.UnitID,
	}

	if err := h.shoppingListService.Create(item, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

type UpdateShoppingListForm struct {
	Name     *string  `json:"name" example:"Organic Milk"`
	Quantity *float64 `validate:"omitempty,gt=0" json:"quantity" example:"1"`
	IsBought *bool    `json:"is_bought" example:"true"`
}

// UpdateShoppingListItem godoc
// @Summary Update shopping list item.
// @Description Update an existing item on the shopping list.
// @Tags shoppinglist
// @Accept json
// @Produce json
// @Param id path string true "Item ID"
// @Param item body UpdateShoppingListForm true "Shopping list item data"
// @Success 200 {object} domain.ShoppingList
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist/{id} [patch]
func (h *ShoppingListHandler) UpdateShoppingListItem(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid item id")
	}

	var form UpdateShoppingListForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	item, err := h.shoppingListService.ByID(id, tokenData.HouseholdID)
	if err != nil {
		return err
	}

	if form.Name != nil {
		item.Product = *form.Name
	}
	if form.Quantity != nil {
		item.Quantity = form.Quantity
	}
	if form.IsBought != nil {
		item.IsBought = *form.IsBought
	}

	if err := h.shoppingListService.Update(item, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.JSON(item)
}

// DeleteShoppingListItem godoc
// @Summary Remove shopping list item.
// @Description Delete an item from the shopping list.
// @Tags shoppinglist
// @Accept */*
// @Produce json
// @Param id path string true "Item ID"
// @Success 204
// @Failure 400 {object} domain.Error
// @Failure 401 {object} domain.Error
// @Failure 403 {object} domain.Error
// @Failure 404 {object} domain.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist/{id} [delete]
func (h *ShoppingListHandler) DeleteShoppingListItem(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sentinels.BadRequest("invalid item id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	if err := h.shoppingListService.Delete(id, tokenData.HouseholdID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
