package services

import (
	"sync"

	"borscht.app/smetana/domain"
	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type RecipeService struct {
	repo             domain.RecipeRepository
	imageService     domain.ImageService
	publisherService domain.PublisherService
	foodRepo         domain.FoodRepository
	unitRepo         domain.UnitRepository
	userRepo         domain.UserRepository
}

func NewRecipeService(repo domain.RecipeRepository, imageService domain.ImageService, publisherService domain.PublisherService, foodRepo domain.FoodRepository, unitRepo domain.UnitRepository, userRepo domain.UserRepository) *RecipeService {
	return &RecipeService{
		repo:             repo,
		imageService:     imageService,
		publisherService: publisherService,
		foodRepo:         foodRepo,
		unitRepo:         unitRepo,
		userRepo:         userRepo,
	}
}

func (s *RecipeService) ById(id uuid.UUID) (*domain.Recipe, error) {
	return s.repo.ById(id)
}

func (s *RecipeService) ByUrl(url string) (*domain.Recipe, error) {
	return s.repo.ByUrl(url)
}

func (s *RecipeService) Create(recipe *domain.Recipe) error {
	return s.repo.Create(recipe)
}

func (s *RecipeService) Update(recipe *domain.Recipe) error {
	return s.repo.Update(recipe)
}

func (s *RecipeService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *RecipeService) CanUserAccess(userID uuid.UUID, recipeID uuid.UUID) (bool, error) {
	return s.repo.IsUserSaved(userID, recipeID)
}

func (s *RecipeService) UserSave(userID uuid.UUID, recipeID uuid.UUID) error {
	user, err := s.userRepo.ById(userID)
	if err != nil {
		return err
	}
	return s.repo.UserSave(userID, recipeID, user.HouseholdID)
}

func (s *RecipeService) UserUnsave(userID uuid.UUID, recipeID uuid.UUID) error {
	return s.repo.UserUnsave(userID, recipeID)
}

func (s *RecipeService) UserSearch(userID uuid.UUID, q string, taxonomies []string, cuisine string, offset, limit int) ([]domain.Recipe, int64, error) {
	return s.repo.UserSearch(userID, q, taxonomies, cuisine, offset, limit)
}

func (s *RecipeService) CreateIngredient(ingredient *domain.RecipeIngredient) error {
	return s.repo.CreateIngredient(ingredient)
}

func (s *RecipeService) UpdateIngredient(ingredient *domain.RecipeIngredient) error {
	return s.repo.UpdateIngredient(ingredient)
}

func (s *RecipeService) DeleteIngredient(id uuid.UUID) error {
	return s.repo.DeleteIngredient(id)
}

func (s *RecipeService) CreateInstruction(instruction *domain.RecipeInstruction) error {
	return s.repo.CreateInstruction(instruction)
}

func (s *RecipeService) UpdateInstruction(instruction *domain.RecipeInstruction) error {
	return s.repo.UpdateInstruction(instruction)
}

func (s *RecipeService) DeleteInstruction(id uuid.UUID) error {
	return s.repo.DeleteInstruction(id)
}

func (s *RecipeService) ImportFromURL(url string) (*domain.Recipe, error) {
	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return nil, err
	}
	return s.ImportFromKripRecipe(kripRecipe, nil)
}

func (s *RecipeService) ImportFromKripRecipe(kripRecipe *model.Recipe, feedID *uuid.UUID) (*domain.Recipe, error) {
	recipe := domain.FromKripRecipe(kripRecipe)
	recipe.FeedID = feedID

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreate(recipe.Publisher); err != nil {
			log.Warnf("error creating publisher %v: %s", recipe.Publisher, err.Error())
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	s.ParseAndEnrichIngredients(recipe.Ingredients, kripRecipe.Language)

	if err := s.repo.Import(recipe); err != nil {
		return nil, err
	}

	s.processRecipeImages(recipe)

	return recipe, nil
}

func (s *RecipeService) ParseAndEnrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		parsed, err := kapusta.ParseIngredient(ingredient.RawText, language)
		if err != nil || parsed == nil {
			continue
		}

		ingredient.Amount = &parsed.Quantity
		if len(parsed.Annotation) != 0 {
			ingredient.Note = &parsed.Annotation
		}
		if len(parsed.Ingredient) != 0 {
			food := &domain.Food{Name: parsed.Ingredient}
			if err := s.foodRepo.FindOrCreate(food); err != nil {
				log.Warnf("error creating food %v: %s", food, err.Error())
			} else {
				ingredient.Food = food
				ingredient.FoodID = &food.ID
			}
		}
		if len(parsed.Unit) != 0 {
			unit := &domain.Unit{Name: parsed.Unit}
			if err := s.unitRepo.FindOrCreate(unit); err != nil {
				log.Warnf("error creating unit %v: %s", unit, err.Error())
			} else {
				ingredient.Unit = unit
				ingredient.UnitID = &unit.ID
			}
		}
	}
}

func (s *RecipeService) processRecipeImages(recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	for _, image := range recipe.Images {
		image.RecipeID = recipe.ID
	}

	if err := s.repo.CreateImages(recipe.Images); err != nil {
		log.Warnf("failed to save images: %v", err)
		return
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5)

	for _, image := range recipe.Images {
		wg.Add(1)
		go func(img *domain.RecipeImage) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			basePath := "recipe/" + img.RecipeID.String() + "/" + img.ID.String()
			if info, err := s.imageService.DownloadAndSaveImage(img.RemoteUrl, basePath); err != nil {
				log.Warnf("failed to download image: %v", err)
			} else {
				img.DownloadUrl = &info.Path
				img.Width = info.Width
				img.Height = info.Height
			}
		}(image)
	}

	wg.Wait()

	for _, img := range recipe.Images {
		if img.DownloadUrl != nil {
			s.repo.UpdateImage(img)
		}
	}
}
