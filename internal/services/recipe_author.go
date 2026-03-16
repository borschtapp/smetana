package services

import (
	"context"

	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/domain"
)

type recipeAuthorService struct {
	repo         domain.RecipeAuthorRepository
	imageService domain.ImageService
}

func NewRecipeAuthorService(repo domain.RecipeAuthorRepository, imageService domain.ImageService) domain.RecipeAuthorService {
	return &recipeAuthorService{repo: repo, imageService: imageService}
}

func (s *recipeAuthorService) FindOrCreate(ctx context.Context, author *domain.RecipeAuthor) error {
	if err := s.repo.FindOrCreate(author); err != nil {
		return err
	}

	if author.RemoteImage == nil || *author.RemoteImage == "" {
		return nil
	}

	image := &domain.Image{
		EntityType: "recipe_authors",
		EntityID:   author.ID,
		SourceURL:  *author.RemoteImage,
	}

	if err := s.imageService.PersistRemote(ctx, image, ""); err != nil {
		log.Warnw("unable to download recipe author image, skipping", "author_id", author.ID, "url", *author.RemoteImage, "error", err)
		return nil
	}

	if err := s.imageService.SetDefault(image); err != nil {
		log.Warnw("unable to set recipe author default image", "author_id", author.ID, "error", err)
		return nil
	}

	author.ImagePath = image.Path
	return nil
}
