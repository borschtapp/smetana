package services

import (
	"errors"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
	"gorm.io/gorm/clause"
)

type PublisherService struct {
	imageService *ImageService
}

func NewPublisherService(imageService *ImageService) *PublisherService {
	return &PublisherService{imageService: imageService}
}

func (s *PublisherService) FindPublisher(pub *domain.Publisher) error {
	if pub.ID != uuid.Nil {
		return nil // current publisher is already existing
	}
	if len(pub.Url) > 0 {
		var existing domain.Publisher
		if err := database.DB.First(&existing, "url = ?", pub.Url).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	if len(pub.Name) > 0 {
		var existing domain.Publisher
		if err := database.DB.First(&existing, "name = ?", pub.Name).Error; err == nil {
			*pub = existing
			return nil
		}
	}
	return errors.New("publisher not found")
}

func (s *PublisherService) FindOrCreatePublisher(pub *domain.Publisher) error {
	if err := s.FindPublisher(pub); err == nil {
		return nil
	}

	if pub.RemoteImage != nil && len(*pub.RemoteImage) != 0 {
		path := pub.FilePath()
		if storedImage, err := s.imageService.DownloadAndPutImage(*pub.RemoteImage, path); err != nil {
			log.Warnf("en error on download publisher image %v: %s", pub, err.Error())
		} else {
			pub.Image = &storedImage.Path
		}
	}

	if err := database.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&pub).Error; err != nil {
		return err
	}

	if pub.ID == uuid.Nil { // fallback for conflict scenario
		return s.FindPublisher(pub)
	}

	return nil
}
