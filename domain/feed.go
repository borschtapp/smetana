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
