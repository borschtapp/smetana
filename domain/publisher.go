package domain

import (
	"strings"
	"time"

	"github.com/borschtapp/krip"
	"gorm.io/gorm"
)

type Publisher struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Url         string         `json:"url,omitempty"`
	Image       string         `json:"image,omitempty"`
	Updated     time.Time      `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time      `gorm:"autoCreateTime" json:"-"`
	Deleted     gorm.DeletedAt `gorm:"index" json:"-"`

	Recipes []Recipe `json:"-"`
}

func (r *Publisher) FilePath() (string, string) {
	return "publisher", strings.ReplaceAll(strings.ToLower(r.Name), " ", "_")
}

func FromKripPublisher(org *krip.Organization) *Publisher {
	model := &Publisher{}
	model.Name = org.Name
	model.Description = org.Description
	model.Url = org.Url
	model.Image = org.Logo
	return model
}
