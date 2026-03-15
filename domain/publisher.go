package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/types"
)

type Publisher struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Name        string        `json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Url         string        `gorm:"uniqueIndex:idx_publisher_url,sort:desc" json:"url,omitempty"`
	ImagePath   *storage.Path `json:"image_url,omitempty"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	// Transient: remote image URL from import, not persisted.
	RemoteImage *string `gorm:"-" json:"-"`

	TotalRecipes *int64    `gorm:"->;-:migration" json:"total_recipes,omitempty"`
	Images       []*Image  `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
	Recipes      []*Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Feeds        []*Feed   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (p *Publisher) BeforeCreate(_ *gorm.DB) error {
	if p.ID == uuid.Nil {
		var err error
		p.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type PublisherRepository interface {
	Search(opts types.SearchOptions) ([]Publisher, int64, error)
	FindOrCreate(pub *Publisher) error
}

type PublisherService interface {
	Search(opts types.SearchOptions) ([]Publisher, int64, error)
	FindOrCreate(ctx context.Context, pub *Publisher) error
}
