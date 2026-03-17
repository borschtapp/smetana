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

// GetShoppingLists godoc
// @Summary List all shopping lists for the household.
// @Tags shopping-lists
// @Produce json
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.ShoppingList]
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [get]
func (h *ShoppingListHandler) GetShoppingLists(c fiber.Ctx) error {
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	p := types.GetPagination(c)

	lists, total, err := h.service.Lists(tokenData.HouseholdID, p.Offset, p.Limit)
	if err != nil {
		return err
	}
	return c.JSON(types.ListResponse[domain.ShoppingList]{
		Data: lists,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}

type ShoppingListForm struct {
	Name string `validate:"required" json:"name" example:"Weekly Shop"`
}

// CreateShoppingList godoc
// @Summary Create a new shopping list.
// @Tags shopping-lists
// @Accept json
// @Produce json
// @Param list body ShoppingListForm true "List data"
// @Success 201 {object} domain.ShoppingList
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [post]
func (h *ShoppingListHandler) CreateShoppingList(c fiber.Ctx) error {
	var form ShoppingListForm
	if err := bindBody(c, &form); err != nil {
		return err
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
// @Tags shopping-lists
// @Produce json
// @Param id path string true "List ID"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.ShoppingItem]
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items [get]
func (h *ShoppingListHandler) GetShoppingListItems(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	p := types.GetPagination(c)

	items, total, err := h.service.Items(id, tokenData.HouseholdID, p.Offset, p.Limit)
	if err != nil {
		return err
	}
	return c.JSON(types.ListResponse[domain.ShoppingItem]{
		Data: items,
		Meta: types.Meta{
			Pagination: p,
			Total:      int(total),
		},
	})
}

// DeleteShoppingList godoc
// @Summary Delete a shopping list.
// @Tags shopping-lists
// @Param id path string true "List ID"
// @Success 204
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id} [delete]
func (h *ShoppingListHandler) DeleteShoppingList(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	if err := h.service.DeleteList(id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type ShoppingItemForm struct {
	Text   string     `validate:"required_without=FoodID" json:"text" example:"2 cups of milk"`
	Amount *float64   `validate:"omitempty,gt=0" json:"amount" example:"2"`
	FoodID *uuid.UUID `json:"food_id"`
	UnitID *uuid.UUID `json:"unit_id"`
}

// AddShoppingItem godoc
// @Summary Add one or more items to a shopping list.
// @Tags shopping-lists
// @Accept json
// @Produce json
// @Param id path string true "List ID"
// @Param item body ShoppingItemForm true "Item data (object or array)"
// @Success 201 {object} domain.ShoppingItem
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items [post]
func (h *ShoppingListHandler) AddShoppingItem(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	body := c.Body()
	isArray := len(body) > 0 && body[0] == '['

	var forms []ShoppingItemForm
	if isArray {
		if err := bindBody(c, &forms); err != nil {
			return err
		}
	} else {
		var form ShoppingItemForm
		if err := bindBody(c, &form); err != nil {
			return err
		}
		forms = []ShoppingItemForm{form}
	}

	items := make([]*domain.ShoppingItem, len(forms))
	for i, form := range forms {
		if err := validate.Struct(form); err != nil {
			return sentinels.BadRequestVal(err)
		}
		items[i] = &domain.ShoppingItem{Text: form.Text, Amount: form.Amount, FoodID: form.FoodID, UnitID: form.UnitID}
	}

	if err := h.service.AddItems(c.Context(), items, id, tokenData.HouseholdID); err != nil {
		return err
	}
	if isArray {
		return c.Status(fiber.StatusCreated).JSON(items)
	}
	return c.Status(fiber.StatusCreated).JSON(items[0])
}

type UpdateShoppingItemForm struct {
	Name     *string  `json:"name" example:"Organic Milk"`
	Amount   *float64 `validate:"omitempty,gt=0" json:"amount" example:"1"`
	IsBought *bool    `json:"is_bought" example:"true"`
}

// UpdateShoppingItem godoc
// @Summary Update a shopping list item.
// @Tags shopping-lists
// @Accept json
// @Produce json
// @Param id path string true "List ID"
// @Param itemId path string true "Item ID"
// @Param item body UpdateShoppingItemForm true "Item data"
// @Success 200 {object} domain.ShoppingItem
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items/{itemId} [patch]
func (h *ShoppingListHandler) UpdateShoppingItem(c fiber.Ctx) error {
	id, itemID, err := types.UuidParams(c, "id", "itemId")
	if err != nil {
		return err
	}

	var form UpdateShoppingItemForm
	if err := bindBody(c, &form); err != nil {
		return err
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}
	patch := &domain.ShoppingItem{ID: itemID}
	if form.Name != nil {
		patch.Text = *form.Name
	}
	if form.Amount != nil {
		patch.Amount = form.Amount
	}
	if form.IsBought != nil {
		patch.IsBought = *form.IsBought
	}

	if err := h.service.UpdateItem(patch, id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.JSON(patch)
}

// DeleteShoppingItem godoc
// @Summary Remove a shopping list item.
// @Tags shopping-lists
// @Param id path string true "List ID"
// @Param itemId path string true "Item ID"
// @Success 204
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items/{itemId} [delete]
func (h *ShoppingListHandler) DeleteShoppingItem(c fiber.Ctx) error {
	id, itemID, err := types.UuidParams(c, "id", "itemId")
	if err != nil {
		return err
	}
	tokenData, err := tokens.ParseJwtClaims(c)
	if err != nil {
		return err
	}

	if err := h.service.DeleteItem(itemID, id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
