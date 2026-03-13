package api

import (
	"borscht.app/smetana/internal/tokens"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type ShoppingListHandler struct {
	service domain.ShoppingListService
}

func NewShoppingListHandler(service domain.ShoppingListService) *ShoppingListHandler {
	return &ShoppingListHandler{service: service}
}

// listID is a helper to parse the :id path param.
func (h *ShoppingListHandler) listID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return uuid.Nil, sentinels.BadRequest("invalid list id")
	}
	return id, nil
}

// itemID is a helper to parse the :itemId path param (for shopping items).
func (h *ShoppingListHandler) itemID(c fiber.Ctx) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Params("itemId"))
	if err != nil {
		return uuid.Nil, sentinels.BadRequest("invalid item id")
	}
	return id, nil
}

// GetShoppingLists godoc
// @Summary List all shopping lists for the household.
// @Tags shoppinglist
// @Produce json
// @Success 200 {object} []domain.ShoppingList
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [get]
func (h *ShoppingListHandler) GetShoppingLists(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	lists, err := h.service.Lists(tokenData.HouseholdID)
	if err != nil {
		return err
	}
	return c.JSON(lists)
}

type ShoppingListForm struct {
	Name string `validate:"required" json:"name" example:"Weekly Shop"`
}

// CreateShoppingList godoc
// @Summary Create a new shopping list.
// @Tags shoppinglist
// @Accept json
// @Produce json
// @Param list body ShoppingListForm true "List data"
// @Success 201 {object} domain.ShoppingList
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [post]
func (h *ShoppingListHandler) CreateShoppingList(c fiber.Ctx) error {
	var form ShoppingListForm
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
	list := &domain.ShoppingList{Name: form.Name}
	if err := h.service.CreateList(list, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(list)
}

// GetShoppingListItems godoc
// @Summary List items in a shopping list.
// @Tags shoppinglist
// @Produce json
// @Param listID path string true "List ID"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.ShoppingItem]
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{listID}/items [get]
func (h *ShoppingListHandler) GetShoppingListItems(c fiber.Ctx) error {
	lid, err := h.listID(c)
	if err != nil {
		return err
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	p := types.GetPagination(c)
	items, total, err := h.service.Items(lid, tokenData.HouseholdID, p.Offset, p.Limit)
	if err != nil {
		return err
	}
	return c.JSON(types.ListResponse[domain.ShoppingItem]{
		Data: items,
		Meta: types.Meta{Total: int(total), Page: p.Page},
	})
}

// DeleteShoppingList godoc
// @Summary Delete a shopping list.
// @Tags shoppinglist
// @Param listID path string true "List ID"
// @Success 204
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{listID} [delete]
func (h *ShoppingListHandler) DeleteShoppingList(c fiber.Ctx) error {
	lid, err := h.listID(c)
	if err != nil {
		return err
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	if err := h.service.DeleteList(lid, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type ShoppingItemForm struct {
	Name     string     `validate:"required" json:"name" example:"Milk"`
	Quantity *float64   `validate:"omitempty,gt=0" json:"quantity" example:"2"`
	UnitID   *uuid.UUID `json:"unit_id"`
}

// AddShoppingItem godoc
// @Summary Add item to a shopping list.
// @Tags shoppinglist
// @Accept json
// @Produce json
// @Param listID path string true "List ID"
// @Param item body ShoppingItemForm true "Item data"
// @Success 201 {object} domain.ShoppingItem
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{listID}/items [post]
func (h *ShoppingListHandler) AddShoppingItem(c fiber.Ctx) error {
	lid, err := h.listID(c)
	if err != nil {
		return err
	}
	var form ShoppingItemForm
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
	item := &domain.ShoppingItem{Product: form.Name, Quantity: form.Quantity, UnitID: form.UnitID}
	if err := h.service.AddItem(item, lid, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(item)
}

type UpdateShoppingItemForm struct {
	Name     *string  `json:"name" example:"Organic Milk"`
	Quantity *float64 `validate:"omitempty,gt=0" json:"quantity" example:"1"`
	IsBought *bool    `json:"is_bought" example:"true"`
}

// UpdateShoppingItem godoc
// @Summary Update a shopping list item.
// @Tags shoppinglist
// @Accept json
// @Produce json
// @Param listID path string true "List ID"
// @Param id path string true "Item ID"
// @Param item body UpdateShoppingItemForm true "Item data"
// @Success 200 {object} domain.ShoppingItem
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{listID}/items/{id} [patch]
func (h *ShoppingListHandler) UpdateShoppingItem(c fiber.Ctx) error {
	lid, err := h.listID(c)
	if err != nil {
		return err
	}
	id, err := h.itemID(c)
	if err != nil {
		return err
	}
	var form UpdateShoppingItemForm
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	patch := &domain.ShoppingItem{ID: id}
	if form.Name != nil {
		patch.Product = *form.Name
	}
	if form.Quantity != nil {
		patch.Quantity = form.Quantity
	}
	if form.IsBought != nil {
		patch.IsBought = *form.IsBought
	}
	if err := h.service.UpdateItem(patch, lid, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.JSON(patch)
}

// DeleteShoppingItem godoc
// @Summary Remove a shopping list item.
// @Tags shoppinglist
// @Param listID path string true "List ID"
// @Param id path string true "Item ID"
// @Success 204
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{listID}/items/{id} [delete]
func (h *ShoppingListHandler) DeleteShoppingItem(c fiber.Ctx) error {
	lid, err := h.listID(c)
	if err != nil {
		return err
	}
	id, err := h.itemID(c)
	if err != nil {
		return err
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	if err := h.service.DeleteItem(id, lid, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
