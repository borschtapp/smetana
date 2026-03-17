package services

import (
	"context"
	"errors"

	"github.com/borschtapp/kapusta"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type shoppingListService struct {
	repo        domain.ShoppingListRepository
	foodService domain.FoodService
	unitService domain.UnitService
}

func NewShoppingListService(repo domain.ShoppingListRepository, foodService domain.FoodService, unitService domain.UnitService) domain.ShoppingListService {
	return &shoppingListService{repo: repo, foodService: foodService, unitService: unitService}
}

// ensureOwned fetches a list by ID and verifies household ownership.
func (s *shoppingListService) ensureOwned(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	list, err := s.repo.ByID(listID)
	if err != nil {
		return nil, err
	}
	if list.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return list, nil
}

func (s *shoppingListService) Lists(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	return s.repo.ListByHousehold(householdID, offset, limit)
}

func (s *shoppingListService) GetList(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	return s.ensureOwned(listID, householdID)
}

func (s *shoppingListService) CreateList(list *domain.ShoppingList, householdID uuid.UUID) error {
	defaultList, err := s.repo.DefaultForHousehold(householdID)
	if err != nil && !errors.Is(err, sentinels.ErrNotFound) {
		return err
	}

	list.HouseholdID = householdID
	list.IsDefault = defaultList == nil // first list for the household becomes the default
	return s.repo.CreateList(list)
}

func (s *shoppingListService) DeleteList(listID uuid.UUID, householdID uuid.UUID) error {
	list, err := s.ensureOwned(listID, householdID)
	if err != nil {
		return err
	}
	if list.IsDefault {
		return sentinels.Unprocessable("cannot delete the default shopping list")
	}
	return s.repo.DeleteList(listID)
}

func (s *shoppingListService) Items(listID uuid.UUID, householdID uuid.UUID, offset, limit int) ([]domain.ShoppingItem, int64, error) {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return nil, 0, err
	}
	return s.repo.ListItems(listID, offset, limit)
}

func (s *shoppingListService) AddItems(ctx context.Context, items []*domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return err
	}

	existingItems, _, err := s.repo.ListItems(listID, 0, -1)
	if err != nil {
		return err
	}

	byFood := make(map[uuid.UUID]*domain.ShoppingItem, len(existingItems))
	for i := range existingItems {
		if existingItems[i].FoodID != nil {
			byFood[*existingItems[i].FoodID] = &existingItems[i]
		}
	}

	var toCreate []*domain.ShoppingItem
	for _, item := range items {
		item.ShoppingListID = listID
		if item.FoodID == nil && item.Text != "" {
			s.parseItemText(ctx, item)
		}

		if item.FoodID != nil {
			if match, ok := byFood[*item.FoodID]; ok {
				if match.IsBought {
					// Restore a previously bought item: uncheck and replace amount.
					match.IsBought = false
					match.Amount = item.Amount
				} else if item.Amount != nil {
					// Accumulate into an existing unbought item.
					if match.Amount != nil {
						match.Amount = new(*match.Amount + *item.Amount)
					} else {
						match.Amount = item.Amount
					}
				}
				if err := s.repo.UpdateItem(match); err != nil {
					return err
				}
				*item = *match
				continue
			}
		}
		toCreate = append(toCreate, item)
	}

	if len(toCreate) == 0 {
		return nil
	}
	return s.repo.CreateItems(toCreate)
}

// parseItemText uses kapusta to extract amount, food, and unit from raw text.
func (s *shoppingListService) parseItemText(ctx context.Context, item *domain.ShoppingItem) {
	parsed, err := kapusta.ParseIngredient(item.Text, "")
	if err != nil || parsed == nil {
		return
	}
	if parsed.Amount != 0 && item.Amount == nil {
		item.Amount = &parsed.Amount
	}
	if parsed.Name != "" {
		food := &domain.Food{Name: parsed.Name, Slug: utils.CreateTag(parsed.Name)}
		if err := s.foodService.FindOrCreate(ctx, food); err == nil {
			item.FoodID = &food.ID
			item.Food = food
		}
	}
	if parsed.Unit != "" {
		unit := &domain.Unit{Name: parsed.Unit, Slug: utils.CreateTag(parsed.UnitCode)}
		if err := s.unitService.FindOrCreate(unit); err == nil {
			item.UnitID = &unit.ID
			item.Unit = unit
		}
	}
}

func (s *shoppingListService) UpdateItem(item *domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error {
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

func (s *shoppingListService) DeleteItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID) error {
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
