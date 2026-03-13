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
// @Param id path string true "List ID"
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
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
		Meta: types.Meta{Total: int(total), Page: p.Page},
	})
}

// DeleteShoppingList godoc
// @Summary Delete a shopping list.
// @Tags shoppinglist
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
// @Tags shoppinglist
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
		if err := c.Bind().Body(&forms); err != nil {
			return sentinels.BadRequest(err.Error())
		}
	} else {
		var form ShoppingItemForm
		if err := c.Bind().Body(&form); err != nil {
			return sentinels.BadRequest(err.Error())
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

	if err := h.service.AddItems(items, id, tokenData.HouseholdID); err != nil {
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
// @Tags shoppinglist
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
	if err := c.Bind().Body(&form); err != nil {
		return sentinels.BadRequest(err.Error())
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
// @Tags shoppinglist
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
