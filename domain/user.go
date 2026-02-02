package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID            uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID   uuid.UUID `gorm:"type:char(36);index" json:"household_id"`
	Name          string    `json:"name"`
	Email         string    `gorm:"uniqueIndex;not null" json:"email"`
	EmailVerified bool      `gorm:"default:false" json:"-"`
	Password      string    `json:"-"`
	Image         string    `json:"image,omitempty"`
	Updated       time.Time `gorm:"autoUpdateTime" json:"updated"`
	Created       time.Time `gorm:"autoCreateTime" json:"created"`

	Household *Household   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"household,omitempty"`
	Tokens    []*UserToken `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Feeds     []*Feed      `gorm:"many2many:feed_subscriptions;" json:"feeds,omitempty"`
	Recipes   []*Recipe    `gorm:"many2many:recipe_saved;" json:"recipes,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		var err error
		u.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
