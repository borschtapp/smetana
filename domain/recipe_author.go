package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeAuthor struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Name        string        `gorm:"uniqueIndex:idx_recipe_author_url,sort:desc" json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Url         string        `gorm:"uniqueIndex:idx_recipe_author_url,sort:desc" json:"url,omitempty"`
	ImagePath   *storage.Path `json:"image_url,omitempty"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	// Transient: remote image URL from import, not persisted.
	RemoteImage *string `gorm:"-" json:"-"`

	Images  []*Image  `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Recipes []*Recipe `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"recipes,omitempty"`
}

func (a *RecipeAuthor) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		var err error
		a.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type RecipeAuthorRepository interface {
	FindOrCreate(author *RecipeAuthor) error
}

type RecipeAuthorService interface {
	FindOrCreate(ctx context.Context, author *RecipeAuthor) error
}
