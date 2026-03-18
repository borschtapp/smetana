package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Author struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Name        string        `gorm:"uniqueIndex:idx_recipe_author_url,sort:desc" json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Url         *string       `gorm:"uniqueIndex:idx_recipe_author_url,sort:desc" json:"url,omitempty"`
	ImagePath   *storage.Path `json:"image_url,omitempty"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	Recipes []*Recipe `gorm:"foreignKey:AuthorID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"recipes,omitempty"`
	Images  []*Image  `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (a *Author) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		var err error
		a.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type AuthorRepository interface {
	FindOrCreate(author *Author) error
}

type AuthorService interface {
	FindOrCreate(ctx context.Context, author *Author) error
}
