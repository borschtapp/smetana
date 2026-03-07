package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/types"
)

type Collection struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID uuid.UUID `gorm:"type:char(36);index" json:"household_id"`
	UserID      uuid.UUID `gorm:"type:char(36);index" json:"user_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time `gorm:"autoCreateTime" json:"-"`

	TotalRecipes *int64     `gorm:"->;-:migration" json:"total_recipes,omitempty"`
	Household    *Household `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"household,omitempty"`
	Recipes      []*Recipe  `gorm:"many2many:collection_recipes;" json:"recipes,omitempty"`
}

func (c *Collection) BeforeCreate(_ *gorm.DB) error {
	if c.ID == uuid.Nil {
		var err error
		c.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type CollectionRepository interface {
	ByID(id uuid.UUID) (*Collection, error)
	ByIdWithRecipes(id uuid.UUID) (*Collection, error)
	Search(householdID uuid.UUID, opts types.SearchOptions) ([]Collection, int64, error)
	Create(collection *Collection) error
	Update(collection *Collection) error
	Delete(id uuid.UUID) error

	AddRecipe(collection *Collection, recipeID uuid.UUID) error
	RemoveRecipe(collection *Collection, recipeID uuid.UUID) error
}

type CollectionService interface {
	ByID(id uuid.UUID, householdID uuid.UUID) (*Collection, error)
	ByIDWithRecipes(id uuid.UUID, householdID uuid.UUID) (*Collection, error)
	Search(householdID uuid.UUID, opts types.SearchOptions) ([]Collection, int64, error)
	Create(collection *Collection, userID uuid.UUID, householdID uuid.UUID) error
	Update(collection *Collection, householdID uuid.UUID) error
	Delete(id uuid.UUID, householdID uuid.UUID) error

	ListRecipes(collectionID uuid.UUID, userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]Recipe, int64, error)
	AddRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error
	RemoveRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error
}
