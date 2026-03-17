package domain

import (
	"context"
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Equipment struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	Slug        string        `gorm:"uniqueIndex" json:"slug"`
	Name        string        `json:"name"`
	Description *string       `json:"description,omitempty"`
	ImagePath   *storage.Path `json:"image_url,omitempty"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`

	Images []*Image `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
}

func (e *Equipment) BeforeCreate(_ *gorm.DB) error {
	if e.ID == uuid.Nil {
		var err error
		e.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type EquipmentRepository interface {
	FindOrCreate(equipment *Equipment) error
	Search(query string, offset, limit int) ([]Equipment, int64, error)
}

type EquipmentService interface {
	FindOrCreate(ctx context.Context, equipment *Equipment) error
	Search(query string, offset, limit int) ([]Equipment, int64, error)
}
