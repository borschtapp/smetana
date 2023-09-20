package domain

import (
	"strings"
	"time"

	"github.com/borschtapp/krip"
	"gorm.io/gorm"

	"borscht.app/smetana/pkg/utils"
)

type PublisherTag struct {
	PublisherID uint
	Tag         string `gorm:"primaryKey"`
}

type Publisher struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `json:"name,omitempty"`
	Description *string        `json:"description,omitempty"`
	Url         string         `json:"url,omitempty"`
	Image       *string        `json:"image,omitempty"`
	Updated     time.Time      `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time      `gorm:"autoCreateTime" json:"-"`
	Deleted     gorm.DeletedAt `gorm:"index" json:"-"`

	Recipes []Recipe       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"-"`
	Tags    []PublisherTag `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (p *Publisher) FilePath() string {
	return "publisher/" + strings.ReplaceAll(utils.CreateTag(p.Name), " ", "_")
}

func (p *Publisher) AfterCreate(tx *gorm.DB) (err error) {
	var tags = []PublisherTag{
		{PublisherID: p.ID, Tag: utils.CreateTag(p.Name)},
		{PublisherID: p.ID, Tag: utils.CreateHostnameTag(p.Url)},
	}

	tx.Create(&tags)
	return
}

func NewPublisherFromKrip(org *krip.Organization) *Publisher {
	model := &Publisher{}
	model.Name = org.Name
	if len(org.Description) != 0 {
		model.Description = &org.Description
	}
	if len(org.Url) != 0 {
		model.Url = org.Url
	}
	if len(org.Logo) != 0 {
		model.Image = &org.Logo
	}
	return model
}
