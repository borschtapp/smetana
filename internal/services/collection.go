package services

import (
	"fmt"

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
		return nil, fmt.Errorf("by id: %w", err)
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *collectionService) ByIDWithRecipes(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return nil, fmt.Errorf("by id with recipes: %w", err)
	}
	if collection.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return collection, nil
}

func (s *collectionService) Search(householdID uuid.UUID, opts types.SearchOptions) ([]domain.Collection, int64, error) {
	collections, total, err := s.repo.Search(householdID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("search: %w", err)
	}
	return collections, total, nil
}

func (s *collectionService) Create(collection *domain.Collection, userID uuid.UUID, householdID uuid.UUID) error {
	collection.HouseholdID = householdID
	collection.UserID = userID
	if err := s.repo.Create(collection); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	return nil
}

func (s *collectionService) Update(collection *domain.Collection, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(collection.ID)
	if err != nil {
		return fmt.Errorf("update (fetch existing): %w", err)
	}
	if existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	if err := s.repo.Update(collection); err != nil {
		return fmt.Errorf("update (persist): %w", err)
	}
	return nil
}

func (s *collectionService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(id)
	if err != nil {
		return fmt.Errorf("delete (fetch existing): %w", err)
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("delete (persist): %w", err)
	}
	return nil
}

func (s *collectionService) ListRecipes(collectionID uuid.UUID, userID uuid.UUID, householdID uuid.UUID, opts types.SearchOptions) ([]domain.Recipe, int64, error) {
	existing, err := s.repo.ByID(collectionID)
	if err != nil {
		return nil, 0, fmt.Errorf("list recipes (fetch collection): %w", err)
	}
	if existing.HouseholdID != householdID {
		return nil, 0, sentinels.ErrForbidden
	}

	recipes, total, err := s.recipeService.Search(userID, householdID, domain.RecipeSearchOptions{SearchOptions: opts, CollectionID: collectionID})
	if err != nil {
		return nil, 0, fmt.Errorf("list recipes (recipe search): %w", err)
	}
	return recipes, total, nil
}

func (s *collectionService) AddRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return fmt.Errorf("add recipe (fetch collection): %w", err)
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	// Validate permission to access the recipe
	_, err = s.recipeService.ByID(recipeID, householdID)
	if err != nil {
		return fmt.Errorf("add recipe (fetch recipe): %w", err)
	}
	if err := s.repo.AddRecipe(collection, recipeID); err != nil {
		return fmt.Errorf("add recipe (persist): %w", err)
	}
	return nil
}

func (s *collectionService) RemoveRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ByID(collectionID)
	if err != nil {
		return fmt.Errorf("remove recipe (fetch collection): %w", err)
	}
	if collection.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	if err := s.repo.RemoveRecipe(collection, recipeID); err != nil {
		return fmt.Errorf("remove recipe (persist): %w", err)
	}
	return nil
}
