package services

import (
	"context"

	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type publisherService struct {
	repo         domain.PublisherRepository
	imageService domain.ImageService
}

func NewPublisherService(repo domain.PublisherRepository, imageService domain.ImageService) domain.PublisherService {
	return &publisherService{repo: repo, imageService: imageService}
}

func (s *publisherService) Search(opts types.SearchOptions) ([]domain.Publisher, int64, error) {
	return s.repo.Search(opts)
}

func (s *publisherService) FindOrCreate(ctx context.Context, pub *domain.Publisher) error {
	if err := s.repo.FindOrCreate(pub); err != nil {
		return err
	}

	if pub.RemoteImage == nil || *pub.RemoteImage == "" {
		return nil
	}

	image := &domain.Image{
		EntityType: "publishers",
		EntityID:   pub.ID,
		SourceURL:  *pub.RemoteImage,
	}

	if err := s.imageService.PersistRemote(ctx, image, ""); err != nil {
		log.Warnw("unable to download publisher image, skipping", "publisher_id", pub.ID, "url", *pub.RemoteImage, "error", err)
		return nil
	}

	if err := s.imageService.SetDefault(image); err != nil {
		log.Warnw("unable to set publisher default image", "publisher_id", pub.ID, "error", err)
		return nil
	}

	pub.ImagePath = image.Path
	return nil
}
