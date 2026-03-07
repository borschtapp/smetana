package services

import (
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type PublisherService struct {
	repo         domain.PublisherRepository
	imageService domain.ImageService
}

func NewPublisherService(repo domain.PublisherRepository, imageService domain.ImageService) domain.PublisherService {
	return &PublisherService{repo: repo, imageService: imageService}
}

func (s *PublisherService) Search(opts types.SearchOptions) ([]domain.Publisher, int64, error) {
	return s.repo.Search(opts)
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
