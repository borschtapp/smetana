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
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [get]
func (h *ShoppingListHandler) GetShoppingLists(c fiber.Ctx) error {
	tokenData := tokens.MustClaims(c)
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists [post]
func (h *ShoppingListHandler) CreateShoppingList(c fiber.Ctx) error {
	var form ShoppingListForm
	if err := bindBody(c, &form); err != nil {
		return err
	}
	tokenData := tokens.MustClaims(c)
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
// @Param offset query int false "Number of records to skip (default: 0)"
// @Param limit query int false "Maximum number of records to return (default: 10)"
// @Success 200 {object} types.ListResponse[domain.ShoppingItem]
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items [get]
func (h *ShoppingListHandler) GetShoppingListItems(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
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
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id} [delete]
func (h *ShoppingListHandler) DeleteShoppingList(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)
	if err := h.service.DeleteList(id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type ShoppingItemForm struct {
	Text   string     `validate:"required_without=FoodID" json:"text" example:"2 cups of milk"`
	Amount *float64   `json:"amount" example:"2"`
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items [post]
func (h *ShoppingListHandler) AddShoppingItem(c fiber.Ctx) error {
	id, err := types.UuidParam(c, "id")
	if err != nil {
		return err
	}

	tokenData := tokens.MustClaims(c)

	body := c.Body()
	isArray := len(body) > 0 && body[0] == '['

	var forms []ShoppingItemForm
	if isArray {
		if err := c.Bind().Body(&forms); err != nil {
			return sentinels.BadRequest(err.Error())
		}
	} else {
		var form ShoppingItemForm
		if err := bindBody(c, &form); err != nil {
			return err
		}
		forms = []ShoppingItemForm{form}
	}

	if len(forms) == 0 {
		return sentinels.BadRequest("request body must contain at least one item")
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
	Text     *string  `json:"text" example:"Organic Milk"`
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
// @Failure 400 {object} sentinels.Error
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
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
	tokenData := tokens.MustClaims(c)
	patch := &domain.ShoppingItem{ID: itemID}
	if form.Text != nil {
		patch.Text = *form.Text
	}
	if form.Amount != nil {
		patch.Amount = form.Amount
	}
	if form.IsBought != nil {
		patch.IsBought = *form.IsBought
	}

	item, err := h.service.UpdateItem(patch, id, tokenData.HouseholdID)
	if err != nil {
		return err
	}
	return c.JSON(item)
}

// DeleteShoppingItem godoc
// @Summary Remove a shopping list item.
// @Tags shopping-lists
// @Param id path string true "List ID"
// @Param itemId path string true "Item ID"
// @Success 204
// @Failure 401 {object} sentinels.Error
// @Failure 404 {object} sentinels.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglists/{id}/items/{itemId} [delete]
func (h *ShoppingListHandler) DeleteShoppingItem(c fiber.Ctx) error {
	id, itemID, err := types.UuidParams(c, "id", "itemId")
	if err != nil {
		return err
	}
	tokenData := tokens.MustClaims(c)
	if err := h.service.DeleteItem(itemID, id, tokenData.HouseholdID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
