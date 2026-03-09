package domain

import (
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeImage struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"-"`
	RecipeID    uuid.UUID     `gorm:"type:char(36);index" json:"-"`
	Width       int           `json:"width,omitempty"`
	Height      int           `json:"height,omitempty"`
	Caption     string        `json:"caption,omitempty"`
	RemoteUrl   string        `json:"-"`
	DownloadUrl *storage.Path `json:"url"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	Recipe *Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (i *RecipeImage) BeforeCreate(_ *gorm.DB) error {
	if i.ID == uuid.Nil {
		var err error
		i.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
