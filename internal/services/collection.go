package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type collectionService struct {
	repo          domain.CollectionRepository
	recipeService domain.RecipeService
}

func NewCollectionService(repo domain.CollectionRepository, recipeService domain.RecipeService) domain.CollectionService {
	return &collectionService{repo: repo, recipeService: recipeService}
}

func (s *collectionService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *collectionService) ByIDWithRecipes(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *collectionService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Collection, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *collectionService) Create(collection *domain.Collection, userID uuid.UUID, householdID uuid.UUID) error {
	collection.HouseholdID = householdID
	collection.UserID = userID
	return s.repo.Create(collection)
}

func (s *collectionService) Update(collection *domain.Collection, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(collection.ID)
	if err != nil {
		return err
	}
	if existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Update(collection)
}

func (s *collectionService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Delete(id)
}

func (s *collectionService) ListRecipes(collectionID uuid.UUID, userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	existing, err := s.repo.ByID(collectionID)
	if err != nil {
		return nil, 0, err
	}
	if existing.HouseholdID != householdID {
		return nil, 0, sentinels.ErrForbidden
	}

	return s.recipeService.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts, CollectionID: collectionID})
}

func (s *collectionService) AddRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	// Validate permission to access the recipe
	_, err = s.recipeService.ByID(recipeID, householdID)
	if err != nil {
		return err
	}
	return s.repo.AddRecipe(collection, recipeID)
}

func (s *collectionService) RemoveRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.RemoveRecipe(collection, recipeID)
}
