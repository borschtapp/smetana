package api

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	sErrors "borscht.app/smetana/pkg/errors"
	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
)

// GetCollections godoc
// @Summary List user's collections.
// @Description Returns collections associated with the user's household.
// @Tags collections
// @Accept */*
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} types.ListResponse[domain.Collection]
// @Failure 401 {object} errors.Error
// @Router /api/collections [get]
func GetCollections(c *fiber.Ctx) error {
	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	p := types.GetPagination(c)
	query := database.DB.Where("household_id = ?", user.HouseholdID)

	var total int64
	query.Model(&domain.Collection{}).Count(&total)

	var collections []domain.Collection
	if err := query.Offset(p.Offset()).Limit(p.Limit).Find(&collections).Error; err != nil {
		return err
	}

	return c.JSON(types.ListResponse[domain.Collection]{
		Data: collections,
		Meta: types.Meta{
			Total: int(total),
			Page:  p.Page,
		},
	})
}

type CollectionForm struct {
	Name        string `validate:"required,min=2" json:"name"`
	Description string `json:"description"`
}

// CreateCollection godoc
// @Summary Create a new collection.
// @Description Create a new recipe collection for the current household.
// @Tags collections
// @Accept json
// @Produce json
// @Param collection body CollectionForm true "Collection data"
// @Success 201 {object} domain.Collection
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/collections [post]
func CreateCollection(c *fiber.Ctx) error {
	var form CollectionForm
	if err := c.BodyParser(&form); err != nil {
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

	collection := &domain.Collection{
		HouseholdID: user.HouseholdID,
		Name:        form.Name,
		Description: form.Description,
	}

	if err := database.DB.Create(collection).Error; err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(collection)
}

// GetCollection godoc
// @Summary Get collection details.
// @Description Returns a collection with all its recipes.
// @Tags collections
// @Accept */*
// @Produce json
// @Param id path string true "Collection ID"
// @Success 200 {object} domain.Collection
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/collections/{id} [get]
func GetCollection(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid collection id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	var collection domain.Collection
	if err := database.DB.Preload("Recipes").First(&collection, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("collection not found")
		}
		return err
	}

	if collection.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("collection does not belong to your household")
	}

	return c.JSON(collection)
}

type UpdateCollectionForm struct {
	Name        *string     `json:"name"`
	Description *string     `json:"description"`
	RecipeIDs   []uuid.UUID `json:"recipe_ids"`
}

// UpdateCollection godoc
// @Summary Update collection.
// @Description Rename collection or manage its recipes.
// @Tags collections
// @Accept json
// @Produce json
// @Param id path string true "Collection ID"
// @Param collection body UpdateCollectionForm true "Collection update data"
// @Success 200 {object} domain.Collection
// @Failure 400 {object} errors.Error
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/collections/{id} [patch]
func UpdateCollection(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid collection id")
	}

	var form UpdateCollectionForm
	if err := c.BodyParser(&form); err != nil {
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

	var collection domain.Collection
	if err := database.DB.First(&collection, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("collection not found")
		}
		return err
	}

	if collection.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("collection does not belong to your household")
	}

	if form.Name != nil {
		collection.Name = *form.Name
	}
	if form.Description != nil {
		collection.Description = *form.Description
	}

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&collection).Error; err != nil {
			return err
		}

		if form.RecipeIDs != nil {
			var recipes []domain.Recipe
			if len(form.RecipeIDs) > 0 {
				if err := tx.Where("id IN ?", form.RecipeIDs).Find(&recipes).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&collection).Association("Recipes").Replace(recipes); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return c.JSON(collection)
}

// DeleteCollection godoc
// @Summary Delete collection.
// @Description Delete a collection.
// @Tags collections
// @Accept */*
// @Produce json
// @Param id path string true "Collection ID"
// @Success 204
// @Failure 401 {object} errors.Error
// @Failure 403 {object} errors.Error
// @Failure 404 {object} errors.Error
// @Security ApiKeyAuth
// @Router /api/collections/{id} [delete]
func DeleteCollection(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return sErrors.BadRequest("invalid collection id")
	}

	tokenData, err := utils.ExtractTokenMetadata(c)
	if err != nil {
		return err
	}

	var user domain.User
	if err := database.DB.First(&user, tokenData.ID).Error; err != nil {
		return err
	}

	var collection domain.Collection
	if err := database.DB.First(&collection, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sErrors.NotFound("collection not found")
		}
		return err
	}

	if collection.HouseholdID != user.HouseholdID {
		return sErrors.Forbidden("collection does not belong to your household")
	}

	if err := database.DB.Delete(&collection).Error; err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}
