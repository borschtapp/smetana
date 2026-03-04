package services

import (
	"borscht.app/smetana/domain"
	"github.com/google/uuid"
)

type CollectionService struct {
	repo domain.CollectionRepository
}

func NewCollectionService(repo domain.CollectionRepository) *CollectionService {
	return &CollectionService{repo: repo}
}

func (s *CollectionService) ById(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ById(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, domain.ErrForbidden
	}
	return collection, nil
}

func (s *CollectionService) ByIdWithRecipes(id uuid.UUID, householdID uuid.UUID) (*domain.Collection, error) {
	collection, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return nil, err
	}
	if collection.HouseholdID != householdID {
		return nil, domain.ErrForbidden
	}
	return collection, nil
}

func (s *CollectionService) List(householdID uuid.UUID, offset, limit int) ([]domain.Collection, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *CollectionService) Create(collection *domain.Collection) error {
	return s.repo.Create(collection)
}

func (s *CollectionService) Update(collection *domain.Collection, householdID uuid.UUID) error {
	existing, err := s.repo.ById(collection.ID)
	if err != nil {
		return err
	}
	if existing.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.Update(collection)
}

func (s *CollectionService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ById(id)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(id)
}

func (s *CollectionService) AddRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ById(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.AddRecipe(collection, recipeID)
}

func (s *CollectionService) RemoveRecipe(collectionID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	collection, err := s.repo.ById(collectionID)
	if err != nil {
		return err
	}
	if collection.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.RemoveRecipe(collection, recipeID)
}
