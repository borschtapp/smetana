package domain

import (
	"math"
	"time"

	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/utils"
	"github.com/sosodev/duration"
	"gorm.io/gorm"
)

type Recipe struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	Url         *string        `json:"url,omitempty"`
	Name        *string        `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Language    *string        `json:"language,omitempty"`
	Author      *Author        `gorm:"embedded;embeddedPrefix:author_" json:"author,omitempty"`
	PublisherID *uint          `json:"-"`
	Text        *string        `json:"text,omitempty"`
	PrepTime    *int           `json:"prep_time,omitempty"`
	CookTime    *int           `json:"cook_time,omitempty"`
	TotalTime   *int           `json:"total_time,omitempty"`
	Difficulty  *string        `json:"difficulty,omitempty"`
	Method      *string        `json:"method,omitempty"`
	Diets       *[]string      `gorm:"serializer:json" json:"diets,omitempty"`
	Categories  *[]string      `gorm:"serializer:json" json:"categories,omitempty"`
	Cuisines    *[]string      `gorm:"serializer:json" json:"cuisines,omitempty"`
	Keywords    *[]string      `gorm:"serializer:json" json:"keywords,omitempty"`
	Yield       *int           `json:"yield,omitempty"`
	Equipment   *[]string      `gorm:"serializer:json" json:"equipment,omitempty"`
	Nutrition   *Nutrition     `gorm:"embedded;embeddedPrefix:nutrition_" json:"nutrition,omitempty"`
	Rating      *Rating        `gorm:"embedded;embeddedPrefix:rating_" json:"rating,omitempty"`
	Video       *Video         `gorm:"embedded;embeddedPrefix:video_" json:"video,omitempty"`
	Published   *time.Time     `json:"published,omitempty"`
	Updated     time.Time      `gorm:"autoUpdateTime" json:"updated"`
	Created     time.Time      `gorm:"autoCreateTime" json:"created"`
	Deleted     gorm.DeletedAt `gorm:"index" json:"-"`

	Publisher    *Publisher           `json:"publisher,omitempty"`
	Images       []*RecipeImage       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Ingredients  []*RecipeIngredient  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"ingredients,omitempty"`
	Instructions []*RecipeInstruction `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instructions,omitempty"`
}

type Nutrition struct {
	Calories    uint   `json:"calories,omitempty"`     // The number of calories.
	ServingSize string `json:"serving_size,omitempty"` // The serving size, in terms of the number of volume or mass.

	Fats        uint `json:"fat,omitempty"`           // The number of grams of fat.
	FatSat      uint `json:"fat_saturated,omitempty"` // The number of grams of saturated fat.
	FatTrans    uint `json:"fat_trans,omitempty"`     // The number of grams of trans fat.
	Cholesterol uint `json:"cholesterol,omitempty"`   // The number of milligrams of cholesterol.
	Sodium      uint `json:"sodium,omitempty"`        // The number of milligrams of sodium.
	Carbs       uint `json:"carbs,omitempty"`         // The number of grams of carbohydrates.
	CarbSugar   uint `json:"carbs_sugar,omitempty"`   // The number of grams of sugar.
	CarbFiber   uint `json:"carbs_fiber,omitempty"`   // The number of grams of fiber.
	Protein     uint `json:"protein,omitempty"`       // The number of grams of protein.
}

type Rating struct {
	Reviews int     `json:"reviews,omitempty"`
	Count   int     `json:"count,omitempty"`
	Value   float64 `json:"value,omitempty"`
}

type Author struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Image       string `json:"image,omitempty"`
}

type Video struct {
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	EmbedUrl     string `json:"embed_url,omitempty"`
	ContentUrl   string `json:"content_url,omitempty"`
	ThumbnailUrl string `json:"thumbnail_url,omitempty"`
}

func FromKripAuthor(person *krip.Person) *Author {
	model := &Author{}
	model.Name = person.Name
	model.Description = person.Description
	model.Url = person.Url
	model.Image = person.Image
	return model
}

func FromKripRecipe(kripRecipe *krip.Recipe) *Recipe {
	recipe := &Recipe{}
	recipe.Url = &kripRecipe.Url
	if len(kripRecipe.Name) > 0 {
		recipe.Name = &kripRecipe.Name
	}
	if len(kripRecipe.Description) > 0 {
		recipe.Description = &kripRecipe.Description
	}
	if len(kripRecipe.Language) > 0 {
		recipe.Language = &kripRecipe.Language
	}
	if len(kripRecipe.Images) > 0 {
		for _, image := range kripRecipe.Images {
			imageModel := FromKripImage(image)
			recipe.Images = append(recipe.Images, imageModel)
		}
	}
	if kripRecipe.Author != nil {
		recipe.Author = FromKripAuthor(kripRecipe.Author)
	}
	if len(kripRecipe.Text) > 0 {
		recipe.Text = &kripRecipe.Text
	}
	if len(kripRecipe.PrepTime) != 0 {
		if d, err := duration.Parse(kripRecipe.PrepTime); err == nil {
			val := int(math.Round(d.ToTimeDuration().Minutes()))
			recipe.PrepTime = &val
		}
	}
	if len(kripRecipe.CookTime) != 0 {
		if d, err := duration.Parse(kripRecipe.CookTime); err == nil {
			val := int(math.Round(d.ToTimeDuration().Minutes()))
			recipe.CookTime = &val
		}
	}
	if len(kripRecipe.TotalTime) != 0 {
		if d, err := duration.Parse(kripRecipe.TotalTime); err == nil {
			val := int(math.Round(d.ToTimeDuration().Minutes()))
			recipe.TotalTime = &val
		}
	}
	if len(kripRecipe.Difficulty) > 0 {
		recipe.Difficulty = &kripRecipe.Difficulty
	}
	if len(kripRecipe.CookingMethod) > 0 {
		recipe.Method = &kripRecipe.CookingMethod
	}
	if kripRecipe.Diets != nil && len(kripRecipe.Diets) > 0 {
		recipe.Diets = &kripRecipe.Diets
	}
	if kripRecipe.Categories != nil && len(kripRecipe.Categories) > 0 {
		recipe.Categories = &kripRecipe.Categories
	}
	if kripRecipe.Cuisines != nil && len(kripRecipe.Cuisines) > 0 {
		recipe.Cuisines = &kripRecipe.Cuisines
	}
	if kripRecipe.Keywords != nil && len(kripRecipe.Keywords) > 0 {
		recipe.Keywords = &kripRecipe.Keywords
	}
	if kripRecipe.Yield > 0 {
		recipe.Yield = &kripRecipe.Yield
	}
	if kripRecipe.Ingredients != nil && len(kripRecipe.Ingredients) > 0 {
		for _, str := range kripRecipe.Ingredients {
			textCopy := str
			recipe.Ingredients = append(recipe.Ingredients, &RecipeIngredient{Text: &textCopy})
		}
	}
	if kripRecipe.Equipment != nil && len(kripRecipe.Equipment) > 0 {
		recipe.Equipment = &kripRecipe.Equipment
	}
	if kripRecipe.Nutrition != nil {
		recipe.Nutrition = &Nutrition{
			Calories:    uint(utils.FindNumber(kripRecipe.Nutrition.Calories)),
			ServingSize: kripRecipe.Nutrition.ServingSize,
			Carbs:       utils.ParseToMillis(kripRecipe.Nutrition.CarbohydrateContent),
			CarbFiber:   utils.ParseToMillis(kripRecipe.Nutrition.FiberContent),
			CarbSugar:   utils.ParseToMillis(kripRecipe.Nutrition.SugarContent),
			Cholesterol: utils.ParseToMillis(kripRecipe.Nutrition.CholesterolContent, 1),
			Sodium:      utils.ParseToMillis(kripRecipe.Nutrition.SodiumContent, 1),
			Fats:        utils.ParseToMillis(kripRecipe.Nutrition.FatContent),
			FatSat:      utils.ParseToMillis(kripRecipe.Nutrition.SaturatedFatContent),
			FatTrans:    utils.ParseToMillis(kripRecipe.Nutrition.TransFatContent),
			Protein:     utils.ParseToMillis(kripRecipe.Nutrition.ProteinContent),
		}
	}
	if kripRecipe.Rating != nil {
		recipe.Rating = &Rating{
			Reviews: kripRecipe.Rating.ReviewCount,
			Count:   kripRecipe.Rating.RatingCount,
			Value:   kripRecipe.Rating.RatingValue,
		}
	}
	if kripRecipe.Video != nil {
		recipe.Video = &Video{
			Name:         kripRecipe.Video.Name,
			Description:  kripRecipe.Video.Description,
			EmbedUrl:     kripRecipe.Video.EmbedUrl,
			ContentUrl:   kripRecipe.Video.ContentUrl,
			ThumbnailUrl: kripRecipe.Video.ThumbnailUrl,
		}
	}
	if kripRecipe.Publisher != nil {
		recipe.Publisher = NewPublisherFromKrip(kripRecipe.Publisher)
	}
	if kripRecipe.DateModified != nil {
		recipe.Published = kripRecipe.DateModified
	} else if kripRecipe.DatePublished != nil {
		recipe.Published = kripRecipe.DatePublished
	}
	if kripRecipe.Instructions != nil && len(kripRecipe.Instructions) > 0 {
		for _, section := range kripRecipe.Instructions {
			instruction := FromKripHowToStep(&section.HowToStep)
			recipe.Instructions = append(recipe.Instructions, instruction)

			if section.Steps != nil && len(section.Steps) != 0 {
				for _, step := range section.Steps {
					instruction := FromKripHowToStep(step)
					instruction.Parent = &instruction.Order
					recipe.Instructions = append(recipe.Instructions, instruction)
				}
			}
		}
	}

	return recipe
}
