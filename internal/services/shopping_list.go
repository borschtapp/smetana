package services

import (
	"errors"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"github.com/google/uuid"
)

type ShoppingListService struct {
	repo domain.ShoppingListRepository
}

func NewShoppingListService(repo domain.ShoppingListRepository) domain.ShoppingListService {
	return &ShoppingListService{repo: repo}
}

// ensureOwned fetches a list by ID and verifies household ownership.
func (s *ShoppingListService) ensureOwned(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	list, err := s.repo.ByID(listID)
	if err != nil {
		return nil, err
	}
	if list.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return list, nil
}

func (s *ShoppingListService) Lists(householdID uuid.UUID) ([]domain.ShoppingList, error) {
	return s.repo.ListByHousehold(householdID)
}

func (s *ShoppingListService) GetList(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	return s.ensureOwned(listID, householdID)
}

func (s *ShoppingListService) CreateList(list *domain.ShoppingList, householdID uuid.UUID) error {
	defaultList, err := s.repo.DefaultForHousehold(householdID)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return err
	}

	list.HouseholdID = householdID
	list.IsDefault = defaultList == nil // first list for the household becomes the default
	return s.repo.CreateList(list)
}

func (s *ShoppingListService) DeleteList(listID uuid.UUID, householdID uuid.UUID) error {
	list, err := s.ensureOwned(listID, householdID)
	if err != nil {
		return err
	}
	if list.IsDefault {
		return sentinels.Unprocessable("cannot delete the default shopping list")
	}
	return s.repo.DeleteList(listID)
}

func (s *ShoppingListService) Items(listID uuid.UUID, householdID uuid.UUID, offset, limit int) ([]domain.ShoppingItem, int64, error) {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListItems(listID, offset, limit)
}

func (s *ShoppingListService) AddItem(item *domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return err
	}
	item.ShoppingListID = listID
	return s.repo.CreateItem(item)
}

func (s *ShoppingListService) UpdateItem(item *domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return err
	}
	// Confirm the item actually belongs to the given list (prevents cross-list item hijacking).
	existing, err := s.repo.ItemByID(item.ID)
	if err != nil {
		return err
	}
	if existing.ShoppingListID != listID {
		return sentinels.ErrForbidden
	}
	return s.repo.UpdateItem(item)
}

func (s *ShoppingListService) DeleteItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return err
	}
	existing, err := s.repo.ItemByID(itemID)
	if err != nil {
		return err
	}
	if existing.ShoppingListID != listID {
		return sentinels.ErrForbidden
	}
	return s.repo.DeleteItem(itemID)
}
