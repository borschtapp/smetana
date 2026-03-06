package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Feed struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	Active      bool      `json:"active"`
	PublisherID uuid.UUID `gorm:"type:char(36)" json:"-"`
	Url         string    `gorm:"uniqueIndex" json:"url"`
	Name        string    `json:"name"`
	ErrorCount  int       `json:"error_count"`
	Retrieved   time.Time `json:"retrieved"` // last successful retrieval time
	Created     time.Time `gorm:"autoCreateTime" json:"created"`
	Updated     time.Time `gorm:"autoUpdateTime" json:"updated"`

	Publisher  *Publisher   `json:"publisher,omitempty"`
	Households []*Household `gorm:"many2many:feed_subscriptions;" json:"households,omitempty"`
	Recipes    []*Recipe    `json:"recipes,omitempty"`
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
	ByUrl(url string) (*Feed, error)
	List(householdID uuid.UUID, offset, limit int) ([]Feed, int64, error)
	ListActive() ([]Feed, error)
	Create(recipe *Feed) error
	Update(recipe *Feed) error
	Delete(id uuid.UUID) error

	AddFeed(householdID uuid.UUID, feed *Feed) error
	DeleteFeed(householdID uuid.UUID, feedID uuid.UUID) error

	Stream(householdID uuid.UUID, offset, limit int) ([]Recipe, int64, error)
}

type FeedService interface {
	List(householdID uuid.UUID, offset, limit int) ([]Feed, int64, error)
	Subscribe(householdID uuid.UUID, url string) (*Feed, error)
	Unsubscribe(householdID uuid.UUID, feedID uuid.UUID) error

	Stream(householdID uuid.UUID, offset, limit int) ([]Recipe, int64, error)
	FetchUpdates() error
}
