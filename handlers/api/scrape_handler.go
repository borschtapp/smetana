package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/borschtapp/krip"
	"github.com/gofiber/fiber/v2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"borscht.app/smetana/pkg/utils"
)

func Scrape(c *fiber.Ctx) error {
	url := c.Query("url")
	update := c.QueryBool("update", false)

	var recipeByUrl domain.Recipe
	if err := database.DB.Where(&domain.Recipe{Url: &url}).Preload("Images").First(&recipeByUrl).Error; err == nil {
		if !update {
			return c.JSON(recipeByUrl)
		} else {
			if err := database.DB.Delete(&recipeByUrl).Error; err != nil {
				return err
			}
		}
	}

	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return err
	}

	// convert Krip's recipe model into Recipe record
	recipe := domain.FromKripRecipe(kripRecipe)

	if recipe.Publisher != nil {
		var pub domain.Publisher
		if err := database.DB.Where(&domain.Publisher{Name: recipe.Publisher.Name}).First(&pub).Error; err == nil {
			recipe.Publisher = &pub
		} else {
			bucket, path := recipe.Publisher.FilePath()
			if newImage, err := utils.DownloadAndPutObject(recipe.Publisher.Image, bucket, path); err != nil {
				log.Println(err)
			} else {
				recipe.Publisher.Image = newImage
			}

			if err := database.DB.Create(&recipe.Publisher).Error; err != nil {
				return err
			}
		}
	}

	if err := database.DB.Create(&recipe).Error; err != nil {
		return err
	}

	if len(recipe.Images) > 0 {
		var wg sync.WaitGroup

		for _, image := range recipe.Images {
			wg.Add(1)
			go processImage(&wg, image)
		}

		wg.Wait()
	}

	return c.Status(http.StatusCreated).JSON(recipe)
}

func processImage(wg *sync.WaitGroup, image *domain.RecipeImage) {
	defer wg.Done()

	bucket, path := image.FilePath()
	if info, err := utils.DownloadAndPutImage(image.RemoteUrl, bucket, path); err != nil {
		log.Printf("failed to download image: %v", err)
	} else {
		image.DownloadUrl = info.Path
		image.Width = info.Width
		image.Height = info.Height

		if err := database.DB.Save(&image).Error; err != nil {
			log.Printf("failed to save image: %v", err)
		}
	}
}
