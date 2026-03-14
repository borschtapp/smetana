package repositories

import (
	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type ImageRepository struct {
	db *gorm.DB
}

func NewImageRepository(db *gorm.DB) domain.ImageRepository {
	return &ImageRepository{db: db}
}

func (r *ImageRepository) Create(image *domain.Image) error {
	return r.db.Create(image).Error
}

func (r *ImageRepository) FindByID(id uuid.UUID) (*domain.Image, error) {
	var image domain.Image
	if err := r.db.First(&image, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &image, nil
}

func (r *ImageRepository) FindByEntity(entityType string, entityID uuid.UUID) ([]*domain.Image, error) {
	var images []*domain.Image
	err := r.db.
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("`order` ASC, created ASC").
		Find(&images).Error
	return images, err
}

func (r *ImageRepository) FindDefault(entityType string, entityID uuid.UUID) (*domain.Image, error) {
	var image domain.Image
	err := r.db.
		Where("entity_type = ? AND entity_id = ? AND is_default = true", entityType, entityID).
		First(&image).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &image, nil
}

func (r *ImageRepository) FindBySourceURL(sourceURL string) (*domain.Image, error) {
	if sourceURL == "" {
		return nil, sentinels.ErrRecordNotFound
	}
	var image domain.Image
	if err := r.db.Where("source_url = ?", sourceURL).First(&image).Error; err != nil {
		return nil, mapErr(err)
	}
	return &image, nil
}

func (r *ImageRepository) SetDefault(image *domain.Image) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Image{}).
			Where("entity_type = ? AND entity_id = ?", image.EntityType, image.EntityID).
			Update("is_default", false).Error; err != nil {
			return err
		}

		if err := tx.Model(&domain.Image{}).
			Where("id = ?", image.ID).
			Update("is_default", true).Error; err != nil {
			return err
		}

		// Intentionally updates image_path on the owning entity table via the polymorphic EntityType string,
		// the same string GORM uses for its polymorphic tag, so it always matches the correct table.
		// Keeping this here ensures is_default and image_path stay in sync within a single transaction.
		if image.Path != nil {
			return tx.Table(image.EntityType).
				Where("id = ?", image.EntityID).
				Update("image_path", string(*image.Path)).Error
		}
		return nil
	})
}

func (r *ImageRepository) Update(image *domain.Image) error {
	return r.db.Model(image).Select("path", "content_type", "width", "height", "size", "caption", "order", "updated").Updates(image).Error
}

func (r *ImageRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Image{}, "id = ?", id).Error
}
