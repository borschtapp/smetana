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

func (s *CollectionService) ById(id uuid.UUID) (*domain.Collection, error) {
	return s.repo.ById(id)
}

func (s *CollectionService) ByIdWithRecipes(id uuid.UUID) (*domain.Collection, error) {
	return s.repo.ByIdWithRecipes(id)
}

func (s *CollectionService) List(householdID uuid.UUID, offset, limit int) ([]domain.Collection, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *CollectionService) Create(collection *domain.Collection) error {
	return s.repo.Create(collection)
}

func (s *CollectionService) Update(collection *domain.Collection) error {
	return s.repo.Update(collection)
}

func (s *CollectionService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}

func (s *CollectionService) AddRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	return s.repo.AddRecipe(collection, recipeID)
}

func (s *CollectionService) RemoveRecipe(collection *domain.Collection, recipeID uuid.UUID) error {
	return s.repo.RemoveRecipe(collection, recipeID)
}
