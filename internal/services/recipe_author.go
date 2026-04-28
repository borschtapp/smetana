package services

import (
	"context"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
	"borscht.app/smetana/internal/utils"
)

type authorService struct {
	repo         domain.AuthorRepository
	imageService domain.ImageService
}

func NewAuthorService(repo domain.AuthorRepository, imageService domain.ImageService) domain.AuthorService {
	return &authorService{repo: repo, imageService: imageService}
}

func (s *authorService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Author, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *authorService) FindOrCreate(ctx context.Context, author *domain.Author) error {
	if author != nil && author.Url != nil && *author.Url != "" {
		author.Url = new(utils.NormalizeURL(*author.Url))
	}
	if err := s.repo.FindOrCreate(author); err != nil {
		return err
	}

	if author != nil && author.ImagePath == nil && len(author.Images) > 0 {
		path, err := s.imageService.PersistRemoteAsDefault(ctx, author.Images[0], "recipe_authors", author.ID, "")
		if err != nil {
			log.Warnw("unable to process recipe author image, skipping", "author_id", author.ID, "image", author.Images[0], "error", err.Error())
		}
		author.ImagePath = path
	}
	return nil
}
