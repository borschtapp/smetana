package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Name            string         `json:"name"`
	Email           string         `gorm:"unique;unique_index;not null" json:"email"`
	EmailVisibility bool           `gorm:"default:false" json:"-"`
	EmailVerified   bool           `gorm:"default:false" json:"-"`
	Password        string         `gorm:"not null" json:"-"`
	Image           string         `json:"image,omitempty"`
	Updated         time.Time      `gorm:"autoUpdateTime" json:"updated"`
	Created         time.Time      `gorm:"autoCreateTime" json:"created"`
	Deleted         gorm.DeletedAt `gorm:"index" json:"-"`

	Tokens  []UserToken `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Recipes []Recipe    `gorm:"many2many:user_recipes;"`
}
