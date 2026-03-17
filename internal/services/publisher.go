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

	if pub != nil && pub.ImagePath == nil && len(pub.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, pub.Images[0], "publishers", pub.ID, "")
		if err != nil {
			log.Warnw("unable to process publisher image, skipping", "publisher_id", pub.ID, "image", pub.Images[0], "error", err)
		}
		pub.ImagePath = path
	}
	return nil
}
