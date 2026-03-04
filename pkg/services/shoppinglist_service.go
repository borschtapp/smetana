package services

import (
	"borscht.app/smetana/domain"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type ShoppingListService struct {
	repo domain.ShoppingListRepository
}

func NewShoppingListService(repo domain.ShoppingListRepository) *ShoppingListService {
	return &ShoppingListService{repo: repo}
}

func (s *ShoppingListService) ById(id uuid.UUID) (*domain.ShoppingList, error) {
	return s.repo.ById(id)
}

func (s *ShoppingListService) List(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *ShoppingListService) Create(item *domain.ShoppingList) error {
	if err := s.repo.Create(item); err != nil {
		return err
	}
	if item.UnitID != nil {
		if fetched, err := s.repo.ById(item.ID); err != nil {
			log.Warnf("failed to reload shopping list item %s after write: %v", item.ID, err)
		} else {
			item.Unit = fetched.Unit
		}
	}
	return nil
}

func (s *ShoppingListService) Update(item *domain.ShoppingList, householdID uuid.UUID) error {
	existing, err := s.repo.ById(item.ID)
	if err != nil {
		return err
	}
	if existing.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	if err := s.repo.Update(item); err != nil {
		return err
	}
	if item.UnitID != nil {
		if fetched, err := s.repo.ById(item.ID); err != nil {
			log.Warnf("failed to reload shopping list item %s after write: %v", item.ID, err)
		} else {
			item.Unit = fetched.Unit
		}
	}
	return nil
}

func (s *ShoppingListService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	item, err := s.repo.ById(id)
	if err != nil {
		return err
	}
	if item.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(id)
}
