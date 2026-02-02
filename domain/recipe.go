package domain

import (
	"time"

	"borscht.app/smetana/pkg/types"
	"borscht.app/smetana/pkg/utils"
	"github.com/borschtapp/krip"
	kUtils "github.com/borschtapp/krip/utils"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Recipe struct {
	ID          uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	IsBasedOn   *string         `gorm:"uniqueIndex" json:"is_based_on,omitempty"`
	Name        *string         `json:"name,omitempty" example:"Spaghetti Carbonara"`
	Description *string         `json:"description,omitempty" example:"A classic Italian pasta dish made with eggs, cheese, pancetta, and pepper."`
	Language    *string         `json:"language,omitempty" example:"en"`
	Author      *Author         `gorm:"embedded;embeddedPrefix:author_" json:"author,omitempty"`
	PublisherID *uuid.UUID      `gorm:"type:char(36)" json:"-"`
	FeedID      *uuid.UUID      `gorm:"type:char(36)" json:"feed_id,omitempty"`
	Text        *string         `json:"text,omitempty"`
	PrepTime    *types.Duration `json:"prep_time,omitempty" swaggertype:"integer" example:"900"`
	CookTime    *types.Duration `json:"cook_time,omitempty" swaggertype:"integer" example:"1200"`
	TotalTime   *types.Duration `json:"total_time,omitempty" swaggertype:"integer" example:"2100"`
	Difficulty  *string         `json:"difficulty,omitempty" example:"Medium"`
	Method      *string         `json:"method,omitempty" example:"Stovetop"`
	Yield       *int            `json:"yield,omitempty" example:"4"`
	Equipment   *[]string       `gorm:"serializer:json" json:"equipment,omitempty" example:"[\"Large pot\", \"Frying pan\"]"`
	Nutrition   *Nutrition      `gorm:"embedded;embeddedPrefix:nutrition_" json:"nutrition,omitempty"`
	Rating      *Rating         `gorm:"embedded;embeddedPrefix:rating_" json:"rating,omitempty"`
	Video       *Video          `gorm:"embedded;embeddedPrefix:video_" json:"video,omitempty"`
	Published   *time.Time      `json:"published,omitempty" swaggertype:"string" format:"date-time"`
	Updated     time.Time       `gorm:"autoUpdateTime" json:"updated" swaggertype:"string" format:"date-time"`
	Created     time.Time       `gorm:"autoCreateTime" json:"created" swaggertype:"string" format:"date-time"`

	Publisher    *Publisher           `json:"publisher,omitempty"`
	Feed         *Feed                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"feed,omitempty"`
	Images       []*RecipeImage       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Ingredients  []*RecipeIngredient  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"ingredients,omitempty"`
	Instructions []*RecipeInstruction `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instructions,omitempty"`
	Taxonomies   []*Taxonomy          `gorm:"many2many:recipe_taxonomies;" json:"taxonomies,omitempty"`
	Collections  []*Collection        `gorm:"many2many:collection_recipes;" json:"collections,omitempty"`
}

type Nutrition struct {
	Calories    float64 `json:"calories,omitempty" example:"450.5"`       // The number of calories.
	ServingSize string  `json:"serving_size,omitempty" example:"1 plate"` // The serving size, in terms of the number of volume or mass.

	Fats        float64 `json:"fat,omitempty" example:"15.2"`          // The number of grams of fat.
	FatSat      float64 `json:"fat_saturated,omitempty" example:"5.1"` // The number of grams of saturated fat.
	FatTrans    float64 `json:"fat_trans,omitempty" example:"0.1"`     // The number of grams of trans fat.
	Cholesterol float64 `json:"cholesterol,omitempty" example:"35.0"`  // The number of milligrams of cholesterol.
	Sodium      float64 `json:"sodium,omitempty" example:"250.0"`      // The number of milligrams of sodium.
	Carbs       float64 `json:"carbs,omitempty" example:"60.0"`        // The number of grams of carbohydrates.
	CarbSugar   float64 `json:"carbs_sugar,omitempty" example:"10.0"`  // The number of grams of sugar.
	CarbFiber   float64 `json:"carbs_fiber,omitempty" example:"4.5"`   // The number of grams of fiber.
	Protein     float64 `json:"protein,omitempty" example:"22.0"`      // The number of grams of protein.
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
	recipe.IsBasedOn = &kripRecipe.Url
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
		if d, err := types.DurationFromISO8601(kripRecipe.PrepTime); err == nil {
			recipe.PrepTime = &d
		}
	}
	if len(kripRecipe.CookTime) != 0 {
		if d, err := types.DurationFromISO8601(kripRecipe.CookTime); err == nil {
			recipe.CookTime = &d
		}
	}
	if len(kripRecipe.TotalTime) != 0 {
		if d, err := types.DurationFromISO8601(kripRecipe.TotalTime); err == nil {
			recipe.TotalTime = &d
		}
	}
	if len(kripRecipe.Difficulty) > 0 {
		recipe.Difficulty = &kripRecipe.Difficulty
	}
	if len(kripRecipe.CookingMethod) > 0 {
		recipe.Method = &kripRecipe.CookingMethod
	}
	if len(kripRecipe.Diets) > 0 {
		for _, diet := range kripRecipe.Diets {
			recipe.Taxonomies = append(recipe.Taxonomies, &Taxonomy{Type: "diet", Label: diet, Slug: utils.CreateTag(diet)})
		}
	}
	if len(kripRecipe.Categories) > 0 {
		for _, cat := range kripRecipe.Categories {
			recipe.Taxonomies = append(recipe.Taxonomies, &Taxonomy{Type: "category", Label: cat, Slug: utils.CreateTag(cat)})
		}
	}
	if len(kripRecipe.Cuisines) > 0 {
		for _, cuisine := range kripRecipe.Cuisines {
			recipe.Taxonomies = append(recipe.Taxonomies, &Taxonomy{Type: "cuisine", Label: cuisine, Slug: utils.CreateTag(cuisine)})
		}
	}
	if len(kripRecipe.Keywords) > 0 {
		for _, keyword := range kripRecipe.Keywords {
			recipe.Taxonomies = append(recipe.Taxonomies, &Taxonomy{Type: "keyword", Label: keyword, Slug: utils.CreateTag(keyword)})
		}
	}
	if kripRecipe.Yield > 0 {
		recipe.Yield = &kripRecipe.Yield
	}
	if len(kripRecipe.Ingredients) > 0 {
		for _, str := range kripRecipe.Ingredients {
			textCopy := str
			recipe.Ingredients = append(recipe.Ingredients, &RecipeIngredient{RawText: textCopy})
		}
	}
	if len(kripRecipe.Equipment) > 0 {
		recipe.Equipment = &kripRecipe.Equipment
	}
	if kripRecipe.Nutrition != nil {
		// TODO: make krip return value in float64 or int
		recipe.Nutrition = &Nutrition{
			Calories:    float64(kUtils.FindNumber(kripRecipe.Nutrition.Calories)),
			ServingSize: kripRecipe.Nutrition.ServingSize,
			Carbs:       float64(kUtils.ParseToMillis(kripRecipe.Nutrition.CarbohydrateContent)),
			CarbFiber:   float64(kUtils.ParseToMillis(kripRecipe.Nutrition.FiberContent)),
			CarbSugar:   float64(kUtils.ParseToMillis(kripRecipe.Nutrition.SugarContent)),
			Cholesterol: float64(kUtils.ParseToMillis(kripRecipe.Nutrition.CholesterolContent, 1)),
			Sodium:      float64(kUtils.ParseToMillis(kripRecipe.Nutrition.SodiumContent, 1)),
			Fats:        float64(kUtils.ParseToMillis(kripRecipe.Nutrition.FatContent)),
			FatSat:      float64(kUtils.ParseToMillis(kripRecipe.Nutrition.SaturatedFatContent)),
			FatTrans:    float64(kUtils.ParseToMillis(kripRecipe.Nutrition.TransFatContent)),
			Protein:     float64(kUtils.ParseToMillis(kripRecipe.Nutrition.ProteinContent)),
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
	if len(kripRecipe.Instructions) > 0 {
		for i := range kripRecipe.Instructions {
			instruction := FromKripHowToStep(&kripRecipe.Instructions[i].HowToStep)
			recipe.Instructions = append(recipe.Instructions, instruction)

			if len(kripRecipe.Instructions[i].Steps) != 0 {
				for _, step := range kripRecipe.Instructions[i].Steps {
					instruction := FromKripHowToStep(step)
					recipe.Instructions = append(recipe.Instructions, instruction)
				}
			}
		}
	}

	return recipe
}

func (r *Recipe) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		var err error
		r.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
