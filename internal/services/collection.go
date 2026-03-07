package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

type CollectionService struct {
	repo       domain.CollectionRepository
	recipeRepo domain.RecipeRepository
}

func NewCollectionService(repo domain.CollectionRepository, recipeRepo domain.RecipeRepository) domain.CollectionService {
	return &CollectionService{repo: repo, recipeRepo: recipeRepo}
}

func (s *CollectionService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *CollectionService) ByIDWithRecipes(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *CollectionService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Collection, int64, error) {
	return s.repo.Search(householdID, opts)
}

func (s *CollectionService) Create(collection *domain.Collection, userID uuid.UUID, householdID uuid.UUID) error {
	collection.HouseholdID = householdID
	collection.UserID = userID
	return s.repo.Create(collection)
}

func (s *CollectionService) Update(collection *domain.Collection, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(collection.ID)
	if err != nil {
		return err
	}
	if existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Update(collection)
}

func (s *CollectionService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Delete(id)
}

func (s *CollectionService) ListRecipes(collectionID uuid.UUID, userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	existing, err := s.repo.ByID(collectionID)
	if err != nil {
		return nil, 0, err
	}
	if existing.HouseholdID != householdID {
		return nil, 0, sentinels.ErrForbidden
	}

	return s.recipeRepo.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts, CollectionID: collectionID})
}

func (s *CollectionService) AddRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	// Validate permission to access the recipe
	recipe, err := s.recipeRepo.ByID(recipeID)
	if err != nil {
		return err
	}
	if recipe.HouseholdID != nil && *recipe.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.AddRecipe(collection, recipeID)
}

func (s *CollectionService) RemoveRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.RemoveRecipe(collection, recipeID)
}
