package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Feed struct {
	ID            uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Url           string    `gorm:"uniqueIndex" json:"url"`
	Name          string    `json:"name"`
	WebsiteUrl    string    `json:"website_url"`
	Description   string    `json:"description"`
	LastFetchedAt time.Time `json:"last_fetched_at"`
	ErrorCount    int       `json:"error_count"`
	Active        bool      `json:"active"`
	Created       time.Time `gorm:"autoCreateTime" json:"created"`
	Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`

	Users   []*User   `gorm:"many2many:feed_subscriptions;" json:"users,omitempty"`
	Recipes []*Recipe `json:"recipes,omitempty"`
}

func (f *Feed) BeforeCreate(tx *gorm.DB) error {
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
	ByUrl(url string) (*Feed, error)
	List(userID uuid.UUID, offset, limit int) ([]Feed, int64, error)
	ListActive() ([]Feed, error)
	Create(recipe *Feed) error
	Update(recipe *Feed) error
	Delete(id uuid.UUID) error

	Stream(userID uuid.UUID, offset, limit int) ([]Recipe, int64, error)
	AddFeed(userID uuid.UUID, feed *Feed) error
	DeleteFeed(userID uuid.UUID, feedID uuid.UUID) error
}

type FeedService interface {
	Subscribe(userID uuid.UUID, url string) (*Feed, error)
	Unsubscribe(userID uuid.UUID, feedID uuid.UUID) error
	List(userID uuid.UUID, offset, limit int) ([]Feed, int64, error)
	Stream(userID uuid.UUID, offset, limit int) ([]Recipe, int64, error)
	FetchUpdates() error
}
