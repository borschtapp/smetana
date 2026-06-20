package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/borschtapp/kapusta"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type shoppingListService struct {
	repo        domain.ShoppingListRepository
	parser      IngredientParser
	foodService domain.FoodService
	unitService domain.UnitService
}

func NewShoppingListService(repo domain.ShoppingListRepository, parser IngredientParser, foodService domain.FoodService, unitService domain.UnitService) domain.ShoppingListService {
	return &shoppingListService{repo: repo, parser: parser, foodService: foodService, unitService: unitService}
}

// ensureOwned fetches a list by ID and verifies household ownership.
func (s *shoppingListService) ensureOwned(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	list, err := s.repo.ByID(listID)
	if err != nil {
		return nil, fmt.Errorf("ensure owned (fetch list): %w", err)
	}
	if list.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return list, nil
}

func (s *shoppingListService) Lists(householdID uuid.UUID, offset, limit int) ([]domain.ShoppingList, int64, error) {
	lists, total, err := s.repo.ListByHousehold(householdID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("lists: %w", err)
	}
	return lists, total, nil
}

func (s *shoppingListService) GetList(listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingList, error) {
	return s.ensureOwned(listID, householdID)
}

func (s *shoppingListService) CreateList(list *domain.ShoppingList, householdID uuid.UUID) error {
	defaultList, err := s.repo.DefaultForHousehold(householdID)
	if err != nil && !errors.Is(err, sentinels.ErrNotFound) {
		return fmt.Errorf("create list (check default): %w", err)
	}

	list.HouseholdID = householdID
	list.IsDefault = defaultList == nil // first list for the household becomes the default
	if err := s.repo.CreateList(list); err != nil {
		return fmt.Errorf("create list (persist): %w", err)
	}
	return nil
}

func (s *shoppingListService) DeleteList(listID uuid.UUID, householdID uuid.UUID) error {
	list, err := s.ensureOwned(listID, householdID)
	if err != nil {
		return err
	}
	if list.IsDefault {
		return sentinels.Unprocessable("cannot delete the default shopping list")
	}
	if err := s.repo.DeleteList(listID); err != nil {
		return fmt.Errorf("delete list (persist): %w", err)
	}
	return nil
}

func (s *shoppingListService) Items(listID uuid.UUID, householdID uuid.UUID, offset, limit int) ([]domain.ShoppingItem, int64, error) {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return nil, 0, fmt.Errorf("items (check permission): %w", err)
	}
	items, total, err := s.repo.ListItems(listID, offset, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("items (fetch): %w", err)
	}
	return items, total, nil
}

func (s *shoppingListService) AddItems(ctx context.Context, items []*domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return fmt.Errorf("add items (check permission): %w", err)
	}

	// Pre-resolve text items so we know all food IDs before querying the DB.
	for _, item := range items {
		if item.FoodID == nil && item.Text != "" {
			s.parseItemText(ctx, item)
		}
	}

	var foodIDs []uuid.UUID
	for _, item := range items {
		if item.FoodID != nil {
			foodIDs = append(foodIDs, *item.FoodID)
		}
	}

	existingItems, err := s.repo.FindItemsByFoodIDs(listID, foodIDs)
	if err != nil {
		return fmt.Errorf("add items (fetch existing): %w", err)
	}

	byFood := make(map[uuid.UUID]*domain.ShoppingItem, len(existingItems))
	for i := range existingItems {
		byFood[*existingItems[i].FoodID] = &existingItems[i]
	}

	var toCreate []*domain.ShoppingItem
	for _, item := range items {
		item.ShoppingListID = listID

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
					return fmt.Errorf("add items (update match %s): %w", match.ID, err)
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
	if err := s.repo.CreateItems(toCreate); err != nil {
		return fmt.Errorf("add items (persist new): %w", err)
	}
	return nil
}

// parseItemText uses kapusta to extract amount, food, and unit from raw text.
func (s *shoppingListService) parseItemText(ctx context.Context, item *domain.ShoppingItem) {
	parsed, err := s.parser.ParseIngredient(item.Text, kapusta.IngredientOptions{})
	if err != nil {
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
	if parsed.Name != "" {
		item.Text = parsed.Name
		if parsed.Description != "" {
			item.Text += ", " + parsed.Description
		}
	}
}

func (s *shoppingListService) GetItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingItem, error) {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return nil, err
	}
	item, err := s.repo.ItemByID(itemID)
	if err != nil {
		return nil, fmt.Errorf("get item (fetch): %w", err)
	}
	if item.ShoppingListID != listID {
		return nil, sentinels.ErrForbidden
	}
	return item, nil
}

func (s *shoppingListService) UpdateItem(item *domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) (*domain.ShoppingItem, error) {
	if _, err := s.GetItem(item.ID, listID, householdID); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateItem(item); err != nil {
		return nil, fmt.Errorf("update item (persist): %w", err)
	}
	item, err := s.repo.ItemByID(item.ID)
	if err != nil {
		return nil, fmt.Errorf("update item (refetch): %w", err)
	}
	return item, nil
}

func (s *shoppingListService) DeleteItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID) error {
	if _, err := s.GetItem(itemID, listID, householdID); err != nil {
		return fmt.Errorf("delete item (check permission): %w", err)
	}
	if err := s.repo.DeleteItem(itemID); err != nil {
		return fmt.Errorf("delete item (persist): %w", err)
	}
	return nil
}
