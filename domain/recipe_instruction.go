package domain

import (
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeInstruction struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	RecipeID    uuid.UUID     `gorm:"type:char(36);index:idx_recipe_order" json:"-"`
	ParentID    *uuid.UUID    `gorm:"type:char(36);index" json:"parent_id,omitempty"`
	Order       uint8         `gorm:"index:idx_recipe_order" json:"order,omitempty"`
	Title       *string       `json:"title,omitempty"`
	Text        string        `json:"text,omitempty"`
	Url         *string       `json:"url,omitempty"`
	Image       *string       `json:"-"`               // remote URL, not stored locally
	DownloadUrl *storage.Path `json:"image,omitempty"` // local URL of downloaded image
	Video       *string       `json:"video,omitempty"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	Recipe *Recipe            `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Parent *RecipeInstruction `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"parent,omitempty"`
}

func (ri *RecipeInstruction) BeforeCreate(_ *gorm.DB) error {
	if ri.ID == uuid.Nil {
		var err error
		ri.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
