package services

import (
	"borscht.app/smetana/domain"
	"github.com/gofiber/fiber/v3/log"
)

type PublisherService struct {
	repo         domain.PublisherRepository
	imageService domain.ImageService
}

func NewPublisherService(repo domain.PublisherRepository, imageService domain.ImageService) *PublisherService {
	return &PublisherService{repo: repo, imageService: imageService}
}

func (s *PublisherService) List(offset, limit int) ([]domain.Publisher, int64, error) {
	return s.repo.List(offset, limit)
}

func (s *PublisherService) FindOrCreate(pub *domain.Publisher) error {
	// Download and store publisher image before persisting
	if pub.RemoteImage != nil && len(*pub.RemoteImage) != 0 {
		path := pub.FilePath()
		if storedImage, err := s.imageService.DownloadAndSaveImage(*pub.RemoteImage, path); err != nil {
			log.Warnf("en error on download publisher image %v: %s", pub, err.Error())
		} else {
			pub.Image = &storedImage.Path
		}
	}

	return s.repo.FindOrCreate(pub)
}
