package domain

import (
	"context"
	"time"

	"github.com/borschtapp/krip"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/types"
)

type Feed struct {
	ID              uuid.UUID            `gorm:"type:char(36);primaryKey" json:"id"`
	Active          bool                 `json:"active"`
	PublisherID     uuid.UUID            `gorm:"type:char(36);index" json:"publisher_id"`
	Url             string               `gorm:"uniqueIndex" json:"url"`
	Name            string               `json:"name"`
	Description     *string              `json:"description,omitempty"`
	ErrorCount      int                  `json:"-"`
	LastSyncAt      time.Time            `json:"last_sync_at"`
	LastSyncSuccess bool                 `json:"last_sync_success"`
	Discovered      *krip.DiscoveredFeed `gorm:"serializer:json" json:"-"`
	Updated         time.Time            `gorm:"autoUpdateTime" json:"-"`
	Created         time.Time            `gorm:"autoCreateTime" json:"-"`

	TotalRecipes *int64       `gorm:"->;-:migration" json:"total_recipes,omitempty"`
	Publisher    *Publisher   `json:"publisher,omitempty"`
	Recipes      []*Recipe    `json:"recipes,omitempty"`
	Households   []*Household `gorm:"many2many:feed_subscriptions;" json:"-"`
}

func (f *Feed) BeforeCreate(_ *gorm.DB) error {
	if f.ID == uuid.Nil {
		var err error
		f.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	f.Active = true
	return nil
}

type FeedRepository interface {
	ByIDForHousehold(id uuid.UUID, householdID uuid.UUID) (*Feed, error)
	ByUrl(url string) (*Feed, error)
	Search(householdID uuid.UUID, opts types.SearchOptions) ([]Feed, int64, error)
	ListActive() ([]Feed, error)
	Create(recipe *Feed) error
	Update(recipe *Feed) error
	Delete(id uuid.UUID) error

	AddFeed(householdID uuid.UUID, feed *Feed) error
	DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error
}

type FeedService interface {
	Search(householdID uuid.UUID, opts types.SearchOptions) ([]Feed, int64, error)
	Subscribe(ctx context.Context, householdID uuid.UUID, url string) (*Feed, error)
	Unsubscribe(householdID uuid.UUID, feedID uuid.UUID) error

	Stream(userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]Recipe, int64, error)
	FetchFeed(ctx context.Context, feed *Feed) (int, int, error)
	Sync(ctx context.Context, householdID uuid.UUID, feedID uuid.UUID) error
}
