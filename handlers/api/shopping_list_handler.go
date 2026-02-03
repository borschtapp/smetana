package api

import (
	"errors"

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

// GetShoppingList godoc
// @Summary List shopping list items.
// @Description Returns all shopping list items for the current household.
// @Tags shoppinglist
// @Accept */*
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.ShoppingList]
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist [get]
func GetShoppingList(c fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	p := types.GetPagination(c)
	query := database.DB.Preload("Unit").Where("household_id = ?", user.HouseholdID)

	var total int64
	query.Model(&domain.ShoppingList{}).Count(&total)

	var items []domain.ShoppingList
	if err := query.Offset(p.Offset()).Limit(p.Limit).Find(&items).Error; err != nil {
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist [post]
func CreateShoppingListItem(c fiber.Ctx) error {
	var form ShoppingListForm
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

	item := &domain.ShoppingList{
		HouseholdID: user.HouseholdID,
		Product:     form.Name,
		Quantity:    form.Quantity,
		UnitID:      form.UnitID,
	}

	if err := database.DB.Create(item).Error; err != nil {
		return err
	}

	if item.UnitID != nil {
		database.DB.Preload("Unit").First(item)
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist/{id} [patch]
func UpdateShoppingListItem(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid item id")
	}

	var form UpdateShoppingListForm
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

	var item domain.ShoppingList
	if err := database.DB.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("item not found")
		}
		return err
	}

	if item.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("item does not belong to your household")
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

	if err := database.DB.Save(&item).Error; err != nil {
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
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/v1/shoppinglist/{id} [delete]
func DeleteShoppingListItem(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid item id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	var item domain.ShoppingList
	if err := database.DB.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("item not found")
		}
		return err
	}

	if item.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("item does not belong to your household")
	}

	if err := database.DB.Delete(&item).Error; err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
