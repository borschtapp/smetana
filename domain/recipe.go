package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Recipe struct {
	ID          uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	ParentID    *uuid.UUID      `gorm:"type:char(36);index" json:"parent_id,omitempty"`
	HouseholdID *uuid.UUID      `gorm:"type:char(36);index" json:"household_id,omitempty"`
	UserID      *uuid.UUID      `gorm:"type:char(36);index" json:"user_id,omitempty"`
	Url         *string         `gorm:"-" json:"url,omitempty"`
	IsBasedOn   *string         `gorm:"index" json:"is_based_on,omitempty"`
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

	IsSaved      *bool                `gorm:"->;-:migration" json:"is_saved,omitempty"`
	Publisher    *Publisher           `json:"publisher,omitempty"`
	Feed         *Feed                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"feed,omitempty"`
	Images       []*RecipeImage       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Ingredients  []*RecipeIngredient  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"ingredients,omitempty"`
	Instructions []*RecipeInstruction `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instructions,omitempty"`
	Taxonomies   []*Taxonomy          `gorm:"many2many:recipe_taxonomies;" json:"taxonomies,omitempty"`
	Collections  []*Collection        `gorm:"many2many:collection_recipes;" json:"collections,omitempty"`
}

type Nutrition struct {
	ServingSize string   `json:"serving_size,omitempty" example:"1 plate"` // The serving size, in terms of the number of volume or mass.
	Calories    *float64 `json:"calories,omitempty" example:"450.5"`       // The number of calories.
	Fats        *float64 `json:"fat,omitempty" example:"15.2"`             // The number of grams of fat.
	FatSat      *float64 `json:"fat_saturated,omitempty" example:"5.1"`    // The number of grams of saturated fat.
	FatTrans    *float64 `json:"fat_trans,omitempty" example:"0.1"`        // The number of grams of trans fat.
	Cholesterol *float64 `json:"cholesterol,omitempty" example:"35.0"`     // The number of milligrams of cholesterol.
	Sodium      *float64 `json:"sodium,omitempty" example:"250.0"`         // The number of milligrams of sodium.
	Carbs       *float64 `json:"carbs,omitempty" example:"60.0"`           // The number of grams of carbohydrates.
	CarbSugar   *float64 `json:"carbs_sugar,omitempty" example:"10.0"`     // The number of grams of sugar.
	CarbFiber   *float64 `json:"carbs_fiber,omitempty" example:"4.5"`      // The number of grams of fiber.
	Protein     *float64 `json:"protein,omitempty" example:"22.0"`         // The number of grams of protein.
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

func (r *Recipe) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		var err error
		r.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type RecipeSearchOptions struct {
	types.SearchOptions

	// below are mutually exclusive options:
	// - if FromFeeds is true, only search recipes from feeds the household is subscribed to
	// - if CollectionID is provided, only search recipes from that collection (assume the caller has already validated access to the collection)
	// - otherwise, search all recipes saved by the household
	FromFeeds    bool
	CollectionID uuid.UUID
}

type RecipeRepository interface {
	ByID(id uuid.UUID) (*Recipe, error)
	ByUrl(url string) (*Recipe, error)
	ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID) ([]Recipe, error)
	Search(userID uuid.UUID, householdID uuid.UUID, opts RecipeSearchOptions) ([]Recipe, int64, error)
	Create(recipe *Recipe) error
	Import(recipe *Recipe) error
	Update(recipe *Recipe) error
	Delete(id uuid.UUID) error

	UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error
	UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error

	CreateImages(images []*RecipeImage) error
	UpdateImage(img *RecipeImage) error

	CreateIngredient(ingredient *RecipeIngredient) error
	UpdateIngredient(ingredient *RecipeIngredient) error
	DeleteIngredient(id uuid.UUID, recipeID uuid.UUID) error

	CreateInstruction(instruction *RecipeInstruction) error
	UpdateInstruction(instruction *RecipeInstruction) error
	DeleteInstruction(id uuid.UUID, recipeID uuid.UUID) error

	Transaction(fn func(txRepo RecipeRepository) error) error
	ReplaceRecipePointers(oldRecipeID, newRecipeID, householdID uuid.UUID) error
}

type RecipeService interface {
	ByID(id uuid.UUID, householdID uuid.UUID) (*Recipe, error)
	Search(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]Recipe, int64, error)
	Create(recipe *Recipe, userID uuid.UUID, householdID uuid.UUID) error
	Update(recipe *Recipe, userID uuid.UUID, householdID uuid.UUID) error
	Delete(id uuid.UUID, householdID uuid.UUID) error

	UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error
	UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error

	CreateIngredient(ingredient *RecipeIngredient, householdID uuid.UUID) error
	UpdateIngredient(ingredient *RecipeIngredient, householdID uuid.UUID) error
	DeleteIngredient(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error

	CreateInstruction(instruction *RecipeInstruction, householdID uuid.UUID) error
	UpdateInstruction(instruction *RecipeInstruction, householdID uuid.UUID) error
	DeleteInstruction(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error

	ImportFromURL(ctx context.Context, url string, forceUpdate bool, userID uuid.UUID, householdID uuid.UUID) (*Recipe, error)
	ImportRecipe(ctx context.Context, recipe *Recipe) (*Recipe, error)
}
