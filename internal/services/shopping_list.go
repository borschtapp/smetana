package services

import (
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
)

type ShoppingListService struct {
	repo domain.ShoppingListRepository
}

func NewShoppingListService(repo domain.ShoppingListRepository) *ShoppingListService {
	return &ShoppingListService{repo: repo}
}

func (s *ShoppingListService) ByID(id uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	item, err := s.repo.ByID(id)
	if err != nil {
		return nil, err
	}
	if item.HouseholdID != householdID {
		return nil, domain.ErrForbidden
	}
	return item, nil
}

func (s *ShoppingListService) List(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	return s.repo.List(householdID, offset, limit)
}

func (s *ShoppingListService) Create(item *domain.ShoppingList, householdID uuid.UUID) error {
	item.HouseholdID = householdID
	if err := s.repo.Create(item); err != nil {
		return err
	}
	if item.UnitID != nil {
		if fetched, err := s.repo.ByID(item.ID); err != nil {
			log.Warnf("failed to reload shopping list item %s after write: %v", item.ID, err)
		} else {
			item.Unit = fetched.Unit
		}
	}
	return nil
}

func (s *ShoppingListService) Update(item *domain.ShoppingList, householdID uuid.UUID) error {
	existing, err := s.repo.ByID(item.ID)
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
		if fetched, err := s.repo.ByID(item.ID); err != nil {
			log.Warnf("failed to reload shopping list item %s after write: %v", item.ID, err)
		} else {
			item.Unit = fetched.Unit
		}
	}
	return nil
}

func (s *ShoppingListService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	item, err := s.repo.ByID(id)
	if err != nil {
		return err
	}
	if item.HouseholdID != householdID {
		return domain.ErrForbidden
	}
	return s.repo.Delete(id)
}
