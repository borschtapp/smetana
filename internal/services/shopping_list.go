package services

import (
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type ShoppingListService struct {
	repo domain.ShoppingListRepository
}

func NewShoppingListService(repo domain.ShoppingListRepository) domain.ShoppingListService {
	return &ShoppingListService{repo: repo}
}

func (s *ShoppingListService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	item, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	if item.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return item, nil
}

func (s *ShoppingListService) List(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *ShoppingListService) Create(item *domain.ShoppingList, householdID uuid.UUID) error {
	item.HouseholdID = householdID
	return s.repo.Create(item)
}

func (s *ShoppingListService) Update(item *domain.ShoppingList, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(item.ID)
	if err != nil {
		return err
	}
	if existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Update(item)
}

func (s *ShoppingListService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	item, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if item.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Delete(id)
}
