package api

import (
	"errors"
	"net/http"
	"sync"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/database/dao"
	"borscht.app/smetana/pkg/utils"
)

func Scrape(c *fiber.Ctx) error {
	url := c.Query("url")
	update := c.QueryBool("update", false)

	var recipeByUrl domain.Recipe
	if err := database.DB.Where(&domain.Recipe{Url: &url}).
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipeByUrl).Error; err == nil {
		if !update {
			return c.JSON(recipeByUrl)
		} else {
			if err := database.DB.Delete(&recipeByUrl).Error; err != nil {
				return err
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return err
	}

	// convert Krip's recipe model into Recipe record
	recipe := domain.FromKripRecipe(kripRecipe)

	if recipe.Publisher != nil {
		if err := dao.FindOrCreatePublisher(recipe.Publisher); err != nil {
			log.Warnf("en error on create publisher %v: %s", recipe.Publisher, err.Error())
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	if len(recipe.Ingredients) != 0 {
		for _, ingredient := range recipe.Ingredients {
			if parsed, err := kapusta.ParseIngredient(*ingredient.Text, kripRecipe.Language); err == nil || parsed != nil {
				ingredient.Amount = parsed.Quantity
				if len(parsed.Annotation) != 0 {
					ingredient.Note = &parsed.Annotation
				}
				if len(parsed.Ingredient) != 0 {
					food := &domain.Food{Name: parsed.Ingredient}
					if err := dao.FindOrCreateFood(food); err != nil {
						log.Warnf("en error on create food %v: %s", food, err.Error())
					} else {
						ingredient.Food = food
						ingredient.FoodID = &food.ID
					}
				}
				if len(parsed.Unit) != 0 {
					unit := &domain.Unit{Name: parsed.Unit}
					if err := dao.FindOrCreateUnit(unit); err != nil {
						log.Warnf("en error on create unit %v: %s", unit, err.Error())
					} else {
						ingredient.Unit = unit
						ingredient.UnitID = &unit.ID
					}
				}
			}
		}
	}

	if err := database.DB.Omit("Publisher", "Images", "Ingredients.Food", "Ingredients.Unit").Create(&recipe).Error; err != nil {
		return err
	}

	if len(recipe.Images) > 0 {
		var wg sync.WaitGroup

		for _, image := range recipe.Images {
			wg.Add(1)
			image.RecipeID = recipe.ID
			go processImage(&wg, image)
		}

		wg.Wait()
		if err := database.DB.Create(recipe.Images).Error; err != nil {
			log.Warnf("failed to save image: %v", err)
		}
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}

func processImage(wg *sync.WaitGroup, image *domain.RecipeImage) {
	defer wg.Done()

	path := image.FilePath()
	if info, err := utils.DownloadAndPutImage(image.RemoteUrl, path); err != nil {
		log.Warnf("failed to download image: %v", err)
	} else {
		image.DownloadUrl = info.Path
		image.Width = info.Width
		image.Height = info.Height
	}
}
