package domain

import (
	"strings"
	"time"

	"github.com/borschtapp/krip"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/storage"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type Publisher struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Name        string        `gorm:"uniqueIndex:idx_publisher_name,sort:desc" json:"name,omitempty"`
	Description *string       `json:"description,omitempty"`
	Url         string        `gorm:"uniqueIndex:idx_publisher_url,sort:desc" json:"url,omitempty"`
	Image       *storage.Path `json:"image,omitempty"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	RemoteImage *string `json:"-" gorm:"-"`

	TotalRecipes *int64    `gorm:"->;-:migration" json:"total_recipes,omitempty"`
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

func (p *Publisher) FilePath() string {
	return "publisher/" + strings.ReplaceAll(utils.CreateTag(p.Name), " ", "_")
}

func FromKripPublisher(org *krip.Organization) *Publisher {
	model := &Publisher{}
	model.Name = org.Name
	if len(org.Description) != 0 {
		model.Description = &org.Description
	}
	if len(org.Url) != 0 {
		model.Url = org.Url
	}
	if len(org.Logo) != 0 {
		model.RemoteImage = &org.Logo
	}
	return model
}

type PublisherRepository interface {
	Search(opts types.SearchOptions) ([]Publisher, int64, error)
	FindOrCreate(pub *Publisher) error
}

type PublisherService interface {
	Search(opts types.SearchOptions) ([]Publisher, int64, error)
	FindOrCreate(pub *Publisher) error
}
