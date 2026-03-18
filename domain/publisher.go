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
	Name        string        `json:"name"`
	Description *string       `json:"description,omitempty"`
	Url         *string       `gorm:"uniqueIndex:idx_publisher_url,sort:desc" json:"url,omitempty"`
	ImagePath   *storage.Path `json:"image_url,omitempty"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	TotalRecipes *int64    `gorm:"->;-:migration" json:"total_recipes,omitempty"`
	Recipes      []*Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"recipes,omitempty"`
	Feeds        []*Feed   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"feeds,omitempty"`
	Images       []*Image  `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
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
