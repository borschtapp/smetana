package domain

import (
	"time"

	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Recipe struct {
	ID          uuid.UUID       `gorm:"type:char(36);primaryKey" json:"id"`
	ParentID    *uuid.UUID      `gorm:"type:char(36);index" json:"-"`
	HouseholdID *uuid.UUID      `gorm:"type:char(36);index" json:"-"`
	UserID      *uuid.UUID      `gorm:"type:char(36);index" json:"user_id,omitempty"`
	SourceUrl   *string         `gorm:"index" json:"source_url,omitempty" validate:"omitempty,url"`
	Name        *string         `json:"name,omitempty" example:"Spaghetti Carbonara" validate:"required,min=2,max=255"`
	ImagePath   *storage.Path   `json:"image_url,omitempty"`
	Description *string         `json:"description,omitempty" example:"A classic Italian pasta dish made with eggs, cheese, pancetta, and pepper." validate:"omitempty,max=10000"`
	Language    *string         `json:"language,omitempty" example:"en" validate:"omitempty,len=2"`
	AuthorID    *uuid.UUID      `gorm:"type:char(36);index" json:"author_id,omitempty"`
	PublisherID *uuid.UUID      `gorm:"type:char(36);index" json:"publisher_id,omitempty"`
	FeedID      *uuid.UUID      `gorm:"type:char(36);index" json:"feed_id,omitempty"`
	Text        *string         `json:"text,omitempty"`
	PrepTime    *types.Duration `json:"prep_time,omitempty" swaggertype:"integer" example:"900" validate:"omitempty,gt=0"`
	CookTime    *types.Duration `json:"cook_time,omitempty" swaggertype:"integer" example:"1200" validate:"omitempty,gt=0"`
	TotalTime   *types.Duration `json:"total_time,omitempty" swaggertype:"integer" example:"2100" validate:"omitempty,gt=0"`
	Difficulty  *string         `json:"difficulty,omitempty" example:"Medium"`
	Method      *string         `json:"method,omitempty" example:"Stovetop"`
	Yield       *int            `json:"yield,omitempty" example:"4" validate:"omitempty,gt=0"`
	Rating      *Rating         `gorm:"embedded;embeddedPrefix:rating_" json:"rating,omitempty"`
	Video       *Video          `gorm:"serializer:json" json:"video,omitempty"`
	Published   *time.Time      `json:"published,omitempty" swaggertype:"string" format:"date-time"`
	Updated     time.Time       `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time       `gorm:"autoCreateTime" json:"-"`

	IsSaved      *bool                `gorm:"->;-:migration" json:"is_saved,omitempty"`
	Parent       *Recipe              `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Author       *Author              `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"author,omitempty"`
	Publisher    *Publisher           `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"publisher,omitempty"`
	Feed         *Feed                `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Nutrition    *RecipeNutrition     `gorm:"foreignKey:RecipeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"nutrition,omitempty"`
	Images       []*Image             `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Ingredients  []*RecipeIngredient  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"ingredients,omitempty"`
	Instructions []*RecipeInstruction `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"instructions,omitempty"`
	Equipment    []*Equipment         `gorm:"many2many:recipe_equipment;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"equipment,omitempty"`
	Taxonomies   []*Taxonomy          `gorm:"many2many:recipe_taxonomies;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"taxonomies,omitempty"`
	Collections  []*Collection        `gorm:"many2many:collection_recipes;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"collections,omitempty"`
}

type Rating struct {
	Reviews *int     `json:"reviews,omitempty" validate:"omitempty,min=0"`
	Count   *int     `json:"count,omitempty" validate:"omitempty,min=0"`
	Value   *float64 `json:"value,omitempty" validate:"omitempty,min=0,max=5"`
}

type Video struct {
	Name         *string `json:"name,omitempty" validate:"omitempty,min=2"`
	Description  *string `json:"description,omitempty" validate:"omitempty,max=2000"`
	EmbedUrl     *string `json:"embed_url,omitempty" validate:"omitempty,url"`
	ContentUrl   *string `json:"content_url,omitempty" validate:"omitempty,url"`
	ThumbnailUrl *string `json:"thumbnail_url,omitempty" validate:"omitempty,url"`
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

	// CollectionID, when non-nil, restricts results to a specific collection.
	CollectionID uuid.UUID
}

type RecipeRepository interface {
	ByID(id uuid.UUID) (*Recipe, error)
	ByIDPreload(id, userID, householdID uuid.UUID, preload types.PreloadOptions) (*Recipe, error)
	ByUrl(url string) (*Recipe, error)
	ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]Recipe, error)
	Search(userID uuid.UUID, householdID uuid.UUID, opts RecipeSearchOptions) ([]Recipe, int64, error)
	Create(recipe *Recipe) error
	Import(recipe *Recipe) error
	Update(recipe *Recipe) error
	Delete(id uuid.UUID) error

	UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error
	UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error

	CreateIngredient(ingredient *RecipeIngredient) error
	UpdateIngredient(ingredient *RecipeIngredient) error
	DeleteIngredient(id uuid.UUID, recipeID uuid.UUID) error

	AddEquipment(recipeID uuid.UUID, equipmentID uuid.UUID) error
	RemoveEquipment(recipeID uuid.UUID, equipmentID uuid.UUID) error

	CreateInstruction(instruction *RecipeInstruction) error
	UpdateInstruction(instruction *RecipeInstruction) error
	DeleteInstruction(id uuid.UUID, recipeID uuid.UUID) error

	Transaction(fn func(txRepo RecipeRepository) error) error
	ReplaceRecipePointers(oldRecipeID, newRecipeID, householdID uuid.UUID) error
}

// RecipePriceEstimate is a computed (never stored) cost breakdown for a recipe.
type RecipePriceEstimate struct {
	Total         float64     `json:"total"`
	PerServing    *float64    `json:"per_serving,omitempty"`    // nil if recipe has no yield
	MissingPrices []uuid.UUID `json:"missing_prices,omitempty"` // food IDs with no recorded price
}

type RecipeService interface {
	ByID(id uuid.UUID, householdID uuid.UUID) (*Recipe, error)
	ByIDPreload(id, userID, householdID uuid.UUID, preload types.PreloadOptions) (*Recipe, error)
	ByUrl(url string, householdID uuid.UUID) (*Recipe, error)
	ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]Recipe, error)
	Search(userID uuid.UUID, householdID uuid.UUID, opts RecipeSearchOptions) ([]Recipe, int64, error)
	Create(recipe *Recipe, userID uuid.UUID, householdID uuid.UUID) error
	Import(recipe *Recipe) error
	SetFeedID(recipeID, feedID uuid.UUID) error
	Update(recipe *Recipe, userID uuid.UUID, householdID uuid.UUID) error
	Delete(id uuid.UUID, householdID uuid.UUID) error

	UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error
	UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error

	CreateIngredient(ingredient *RecipeIngredient, householdID uuid.UUID) error
	UpdateIngredient(ingredient *RecipeIngredient, householdID uuid.UUID) error
	DeleteIngredient(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error

	AddEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error
	RemoveEquipment(recipeID uuid.UUID, equipmentID uuid.UUID, householdID uuid.UUID) error

	CreateInstruction(instruction *RecipeInstruction, householdID uuid.UUID) error
	UpdateInstruction(instruction *RecipeInstruction, householdID uuid.UUID) error
	DeleteInstruction(id uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error

	EstimatePrice(recipeID uuid.UUID, householdID uuid.UUID) (*RecipePriceEstimate, error)
}
