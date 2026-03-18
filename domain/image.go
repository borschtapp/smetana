package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/internal/storage"
)

// Image is a polymorphic storage record owned by any entity via EntityType + EntityID.
type Image struct {
	ID          uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	EntityType  string        `gorm:"index:idx_entity;not null" json:"-"`
	EntityID    uuid.UUID     `gorm:"type:char(36);index:idx_entity;not null" json:"-"`
	Path        *storage.Path `json:"url"`
	Width       *int          `json:"width,omitempty"`
	Height      *int          `json:"height,omitempty"`
	ContentType *string       `json:"content_type,omitempty"`
	Size        *int64        `json:"size,omitempty"`
	Caption     *string       `json:"caption,omitempty"`
	SourceURL   string        `gorm:"index" json:"-"`
	IsDefault   bool          `gorm:"default:false;index" json:"-"`
	Order       int           `gorm:"default:0" json:"order"`
	Updated     time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created     time.Time     `gorm:"autoCreateTime" json:"-"`
}

func (i *Image) BeforeCreate(_ *gorm.DB) error {
	if i.ID == uuid.Nil {
		var err error
		i.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

// UploadedImage is returned by ImageService.PersistUploaded for user uploads not yet tied to a specific entity.
type UploadedImage struct {
	Path        storage.Path `json:"url"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
	ContentType string       `json:"content_type"`
	Size        int64        `json:"size"`
}

type ImageRepository interface {
	Create(image *Image) error
	Update(image *Image) error
	// SetDefault also updates image_path of the owning entity table via EntityType reference.
	SetDefault(image *Image) error
	FindByID(id uuid.UUID) (*Image, error)
	FindByEntity(entityType string, entityID uuid.UUID) ([]*Image, error)
	FindDefault(entityType string, entityID uuid.UUID) (*Image, error)
	FindBySourceURL(sourceURL string) (*Image, error)
	Delete(id uuid.UUID) error
}

type ImageService interface {
	// PersistRemote fetches a remote URL, saves the file, and creates an Image record.
	// pathPrefix sets the storage subdirectory; if empty, it is resolved as image.EntityType + "/" + image.EntityID.
	// Deduplicates by SourceURL: if the URL was already downloaded, the existing record is returned.
	PersistRemote(ctx context.Context, image *Image, pathPrefix string) error
	// PersistRemoteAsDefault downloads and persists the image if it has a SourceURL and hasn't been saved yet.
	PersistRemoteAsDefault(ctx context.Context, image *Image, entityType string, entityID uuid.UUID, pathPrefix string) (*storage.Path, error)
	// PersistUploaded stores bytes to storage (no DB record).
	PersistUploaded(ctx context.Context, data []byte, contentType string) (*UploadedImage, error)
	// SetDefault marks the image as default in the DB and returns its storage path.
	// Callers should assign the returned path to entity.ImagePath for in-memory use.
	SetDefault(image *Image) error
	// Delete removes the Image record and its backing storage file.
	Delete(imageID uuid.UUID) error
}
