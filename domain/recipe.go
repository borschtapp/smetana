package domain

import (
	"encoding/json"
	"math"
	"time"

	"github.com/borschtapp/krip"
	"github.com/sosodev/duration"
	"gorm.io/gorm"

	"borscht.app/smetana/pkg/database/types"
)

type Recipe struct {
	ID            uint                     `gorm:"primaryKey" json:"id"`
	Url           *string                  `gorm:"url" json:"url,omitempty"`
	Name          *string                  `gorm:"name" json:"name,omitempty"`
	Description   *string                  `gorm:"description" json:"description,omitempty"`
	Language      *string                  `gorm:"language" json:"inLanguage,omitempty"`
	Images        []*RecipeImage           `json:"image,omitempty"`
	Author        *Author                  `gorm:"embedded;embeddedPrefix:author_" json:"author,omitempty"`
	PublisherID   uint                     `json:"-"`
	Publisher     *Publisher               `gorm:"publisher" json:"publisher,omitempty"`
	Text          *string                  `gorm:"text" json:"text,omitempty"`
	PrepTime      *int                     `gorm:"prep_time" json:"prepTime,omitempty"`
	CookTime      *int                     `gorm:"cook_time" json:"cookTime,omitempty"`
	TotalTime     *int                     `gorm:"total_time" json:"totalTime,omitempty"`
	Difficulty    *string                  `gorm:"difficulty" json:"educationalLevel,omitempty"`
	CookingMethod *string                  `gorm:"cookingMethod" json:"cookingMethod,omitempty"`
	Diets         *types.JsonArray[string] `gorm:"suitableForDiet" json:"suitableForDiet,omitempty"`
	Categories    *types.JsonArray[string] `gorm:"recipeCategory" json:"recipeCategory,omitempty"`
	Cuisines      *types.JsonArray[string] `gorm:"recipeCuisine" json:"recipeCuisine,omitempty"`
	Keywords      *types.JsonArray[string] `gorm:"keywords" json:"keywords,omitempty"`
	Yield         *int                     `gorm:"recipeYield" json:"recipeYield,omitempty"`               // alias `yield`
	Ingredients   *types.JsonArray[string] `gorm:"recipeIngredient" json:"recipeIngredient,omitempty"`     // alias `supply`
	Equipment     *types.JsonArray[string] `gorm:"recipeEquipment" json:"tool,omitempty"`                  // FIXME: `recipeEquipment` is not a part of Recipe schema https://github.com/schemaorg/schemaorg/issues/3132
	Instructions  *types.JsonRaw           `gorm:"recipeInstructions" json:"recipeInstructions,omitempty"` // alias `step`
	Notes         *types.JsonArray[string] `gorm:"notes" json:"correction,omitempty"`                      // some notes or advices for the recipe
	Nutrition     *Nutrition               `gorm:"embedded;embeddedPrefix:facts_" json:"nutrition,omitempty"`
	Rating        *Rating                  `gorm:"embedded;embeddedPrefix:rating_" json:"aggregateRating,omitempty"`
	CommentCount  *int                     `gorm:"commentCount" json:"commentCount,omitempty"`
	Video         *Video                   `gorm:"embedded;embeddedPrefix:video_" json:"video,omitempty"`
	Links         *types.JsonArray[string] `gorm:"links" json:"citation,omitempty"` // maybe not the cleanest name, but we can store additional links here
	DateModified  *time.Time               `gorm:"dateModified" json:"dateModified,omitempty"`
	DatePublished *time.Time               `gorm:"datePublished" json:"datePublished,omitempty"`
	Updated       time.Time                `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time                `gorm:"autoCreateTime" json:"-"`
	Deleted       gorm.DeletedAt           `gorm:"index" json:"-"`
}

type Nutrition struct {
	Calories              string `json:"calories,omitempty"`              // The number of calories.
	ServingSize           string `json:"servingSize,omitempty"`           // The serving size, in terms of the number of volume or mass.
	CarbohydrateContent   string `json:"carbohydrateContent,omitempty"`   // The number of grams of carbohydrates.
	CholesterolContent    string `json:"cholesterolContent,omitempty"`    // The number of milligrams of cholesterol.
	FatContent            string `json:"fatContent,omitempty"`            // The number of grams of fat.
	FiberContent          string `json:"fiberContent,omitempty"`          // The number of grams of fiber.
	ProteinContent        string `json:"proteinContent,omitempty"`        // The number of grams of protein.
	SaturatedFatContent   string `json:"saturatedFatContent,omitempty"`   // The number of grams of saturated fat.
	SodiumContent         string `json:"sodiumContent,omitempty"`         // The number of milligrams of sodium.
	SugarContent          string `json:"sugarContent,omitempty"`          // The number of grams of sugar.
	TransFatContent       string `json:"transFatContent,omitempty"`       // The number of grams of trans fat.
	UnsaturatedFatContent string `json:"unsaturatedFatContent,omitempty"` // The number of grams of unsaturated fat.
}

type Rating struct {
	ReviewCount int     `json:"reviewCount,omitempty"`
	RatingCount int     `json:"ratingCount,omitempty"`
	RatingValue float64 `json:"ratingValue,omitempty"`
	BestRating  int     `json:"bestRating,omitempty"`
	WorstRating int     `json:"worstRating,omitempty"`
}

type Author struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Url         string `json:"url,omitempty"`
	Image       string `json:"image,omitempty"`
}

type Video struct {
	Name         string     `json:"name,omitempty"`
	Description  string     `json:"description,omitempty"`
	Duration     string     `json:"duration,omitempty"`
	EmbedUrl     string     `json:"embedUrl,omitempty"`
	ContentUrl   string     `json:"contentUrl,omitempty"`
	ThumbnailUrl string     `json:"thumbnailUrl,omitempty"`
	UploadDate   *time.Time `json:"uploadDate,omitempty"`
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
			imageModel.RecipeID = recipe.ID
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
		recipe.CookingMethod = &kripRecipe.CookingMethod
	}
	if kripRecipe.Diets != nil && len(kripRecipe.Diets) > 0 {
		recipe.Diets = types.ToJsonArray(kripRecipe.Diets)
	}
	if kripRecipe.Categories != nil && len(kripRecipe.Categories) > 0 {
		recipe.Categories = types.ToJsonArray(kripRecipe.Categories)
	}
	if kripRecipe.Cuisines != nil && len(kripRecipe.Cuisines) > 0 {
		recipe.Cuisines = types.ToJsonArray(kripRecipe.Cuisines)
	}
	if kripRecipe.Keywords != nil && len(kripRecipe.Keywords) > 0 {
		recipe.Keywords = types.ToJsonArray(kripRecipe.Keywords)
	}
	if kripRecipe.Yield > 0 {
		recipe.Yield = &kripRecipe.Yield
	}
	if kripRecipe.Ingredients != nil && len(kripRecipe.Ingredients) > 0 {
		recipe.Ingredients = types.ToJsonArray(kripRecipe.Ingredients)
	}
	if kripRecipe.Equipment != nil && len(kripRecipe.Equipment) > 0 {
		recipe.Equipment = types.ToJsonArray(kripRecipe.Equipment)
	}
	if kripRecipe.Notes != nil && len(kripRecipe.Notes) > 0 {
		recipe.Notes = types.ToJsonArray(kripRecipe.Notes)
	}
	if kripRecipe.Nutrition != nil {
		recipe.Nutrition = &Nutrition{
			Calories:              kripRecipe.Nutrition.Calories,
			ServingSize:           kripRecipe.Nutrition.ServingSize,
			CarbohydrateContent:   kripRecipe.Nutrition.CarbohydrateContent,
			CholesterolContent:    kripRecipe.Nutrition.CholesterolContent,
			FatContent:            kripRecipe.Nutrition.FatContent,
			FiberContent:          kripRecipe.Nutrition.FiberContent,
			ProteinContent:        kripRecipe.Nutrition.ProteinContent,
			SaturatedFatContent:   kripRecipe.Nutrition.SaturatedFatContent,
			SodiumContent:         kripRecipe.Nutrition.SodiumContent,
			SugarContent:          kripRecipe.Nutrition.SugarContent,
			TransFatContent:       kripRecipe.Nutrition.TransFatContent,
			UnsaturatedFatContent: kripRecipe.Nutrition.UnsaturatedFatContent,
		}
	}
	if kripRecipe.Rating != nil {
		recipe.Rating = &Rating{
			ReviewCount: kripRecipe.Rating.ReviewCount,
			RatingCount: kripRecipe.Rating.RatingCount,
			RatingValue: kripRecipe.Rating.RatingValue,
			BestRating:  kripRecipe.Rating.BestRating,
			WorstRating: kripRecipe.Rating.WorstRating,
		}
	}
	if kripRecipe.CommentCount > 0 {
		recipe.CommentCount = &kripRecipe.CommentCount
	}
	if kripRecipe.Video != nil {
		recipe.Video = &Video{
			Name:         kripRecipe.Video.Name,
			Description:  kripRecipe.Video.Description,
			Duration:     kripRecipe.Video.Duration,
			EmbedUrl:     kripRecipe.Video.EmbedUrl,
			ContentUrl:   kripRecipe.Video.ContentUrl,
			ThumbnailUrl: kripRecipe.Video.ThumbnailUrl,
			UploadDate:   kripRecipe.Video.UploadDate,
		}
	}
	if kripRecipe.Links != nil && len(kripRecipe.Links) > 0 {
		recipe.Links = types.ToJsonArray(kripRecipe.Links)
	}
	if kripRecipe.Publisher != nil {
		recipe.Publisher = FromKripPublisher(kripRecipe.Publisher)
	}
	if kripRecipe.DateModified != nil {
		recipe.DateModified = kripRecipe.DateModified
	}
	if kripRecipe.DatePublished != nil {
		recipe.DatePublished = kripRecipe.DatePublished
	}
	if kripRecipe.Instructions != nil && len(kripRecipe.Instructions) > 0 {
		if data, err := json.Marshal(kripRecipe.Instructions); err == nil {
			var val types.JsonRaw
			val = data
			recipe.Instructions = &val
		}
	}

	return recipe
}
