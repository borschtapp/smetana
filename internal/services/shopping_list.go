package services

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

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

func (s *shoppingListService) AddItems(ctx context.Context, items []*domain.ShoppingItem, listID uuid.UUID, householdID uuid.UUID) ([]*domain.ShoppingItem, error) {
	if _, err := s.ensureOwned(listID, householdID); err != nil {
		return nil, fmt.Errorf("add items (check permission): %w", err)
	}

	// Pre-resolve text items so we know all food IDs before querying the DB.
	for _, item := range items {
		if item.FoodID == nil && item.Text != "" {
			s.parseItemText(ctx, item)
		}
	}

	// Client-supplied IDs may be aliases of merged foods or units; parsed ones are already canonical.
	var clientFoodIDs, clientUnitIDs []*uuid.UUID
	for _, item := range items {
		if item.Food == nil {
			clientFoodIDs = append(clientFoodIDs, item.FoodID)
		}
		if item.Unit == nil {
			clientUnitIDs = append(clientUnitIDs, item.UnitID)
		}
	}
	if err := s.foodService.Canonicalize(clientFoodIDs...); err != nil {
		return nil, fmt.Errorf("add items (resolve foods): %w", err)
	}
	if err := s.unitService.Canonicalize(clientUnitIDs...); err != nil {
		return nil, fmt.Errorf("add items (resolve units): %w", err)
	}

	// Pass on loaded units; the rest are fetched in one query, and only if a conversion needs them.
	converter := s.unitService.NewConverter()
	foodSet := make(map[uuid.UUID]struct{}, len(items))
	for _, item := range items {
		if item.FoodID != nil {
			foodSet[*item.FoodID] = struct{}{}
		}
		if item.Unit != nil {
			converter.Add(item.Unit)
		} else if item.UnitID != nil {
			converter.Expect(*item.UnitID)
		}
	}
	foodIDs := slices.Collect(maps.Keys(foodSet))

	conversions, err := s.foodService.ConversionsByFoodIDs(foodIDs)
	if err != nil {
		return nil, fmt.Errorf("add items (fetch conversions): %w", err)
	}
	converter.Expect(conversionUnitIDs(conversions)...)

	// Combine items that reference the same food so the batch itself never asks to create
	// (or double-accumulate into) two rows for the same food.
	items, err = mergeDuplicateFoodItems(items, conversions, converter)
	if err != nil {
		return nil, fmt.Errorf("add items (merge batch): %w", err)
	}

	err = s.repo.Transaction(func(txRepo domain.ShoppingListRepository) error {
		existingItems, err := txRepo.FindItemsByFoodIDs(listID, foodIDs)
		if err != nil {
			return fmt.Errorf("fetch existing: %w", err)
		}

		byFood := make(map[uuid.UUID]*domain.ShoppingItem, len(existingItems))
		for i := range existingItems {
			byFood[*existingItems[i].FoodID] = &existingItems[i]
			converter.Add(existingItems[i].Unit)
		}

		var toCreate []*domain.ShoppingItem
		for _, item := range items {
			item.ShoppingListID = listID

			var match *domain.ShoppingItem
			if item.FoodID != nil {
				match = byFood[*item.FoodID]
			}
			if match == nil {
				toCreate = append(toCreate, item)
				continue
			}

			if match.IsBought {
				reopenWith(match, item)
			} else if err := accumulateAmount(match, item, conversions[*item.FoodID], converter); err != nil {
				return fmt.Errorf("accumulate into %s: %w", match.ID, err)
			}
			if err := txRepo.UpdateItem(match); err != nil {
				return fmt.Errorf("update match %s: %w", match.ID, err)
			}
			*item = *match
		}

		if len(toCreate) == 0 {
			return nil
		}
		if err := txRepo.CreateItems(toCreate); err != nil {
			return fmt.Errorf("persist new: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("add items: %w", err)
	}
	return items, nil
}

// mergeDuplicateFoodItems combines items referencing the same food into the first
// occurrence, accumulating their amounts, so a single batch cannot create duplicate rows.
func mergeDuplicateFoodItems(items []*domain.ShoppingItem, conversions map[uuid.UUID][]domain.FoodUnitConversion, converter domain.UnitConverter) ([]*domain.ShoppingItem, error) {
	merged := make([]*domain.ShoppingItem, 0, len(items))
	byFood := make(map[uuid.UUID]*domain.ShoppingItem, len(items))
	for _, item := range items {
		if item.FoodID == nil {
			merged = append(merged, item)
			continue
		}
		if existing, ok := byFood[*item.FoodID]; ok {
			if err := accumulateAmount(existing, item, conversions[*item.FoodID], converter); err != nil {
				return nil, err
			}
			continue
		}
		byFood[*item.FoodID] = item
		merged = append(merged, item)
	}
	return merged, nil
}

// accumulateAmount adds incoming's amount to target, converted to target's unit (a missing unit means pieces).
// Returns a 422 if the units cannot be converted, rather than adding mismatched amounts.
func accumulateAmount(target, incoming *domain.ShoppingItem, conversions []domain.FoodUnitConversion, converter domain.UnitConverter) error {
	if incoming.Amount == nil {
		return nil
	}
	if target.Amount == nil {
		target.Amount = incoming.Amount
		setUnit(target, incoming.UnitID, incoming.Unit)
		return nil
	}

	amount, err := converter.Convert(*incoming.Amount, incoming.UnitID, target.UnitID, conversions)
	if errors.Is(err, sentinels.ErrUnitConversion) {
		return incompatibleUnitsError(target)
	}
	if err != nil {
		return fmt.Errorf("convert amount: %w", err)
	}
	sum := *target.Amount + amount
	target.Amount = &sum
	return nil
}

// reopenWith restores a bought item for incoming: it is unchecked and takes incoming's amount, unit and text.
func reopenWith(target, incoming *domain.ShoppingItem) {
	target.IsBought = false
	target.Amount = incoming.Amount
	setUnit(target, incoming.UnitID, incoming.Unit)
	if incoming.Text != "" {
		target.Text = incoming.Text
	}
}

// setUnit points item at unitID with its unit. When the ID is unchanged, the loaded Unit is kept,
// so UpdateItem does not reload it.
func setUnit(item *domain.ShoppingItem, unitID *uuid.UUID, unit *domain.Unit) {
	if unitID != nil && item.Unit != nil && item.Unit.ID == *unitID {
		return
	}
	item.UnitID, item.Unit = unitID, unit
}

func incompatibleUnitsError(target *domain.ShoppingItem) error {
	subject := "amount"
	if target.Text != "" {
		subject = fmt.Sprintf("amount for %q", target.Text)
	}
	return sentinels.Unprocessable(subject + " cannot be added: its unit cannot be converted to the unit already on the list")
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

// UpdateItem applies changes to the stored item, since the repository writes every updatable column.
func (s *shoppingListService) UpdateItem(itemID uuid.UUID, listID uuid.UUID, householdID uuid.UUID, apply func(*domain.ShoppingItem)) (*domain.ShoppingItem, error) {
	item, err := s.GetItem(itemID, listID, householdID)
	if err != nil {
		return nil, err
	}
	apply(item)
	if err := s.repo.UpdateItem(item); err != nil {
		return nil, fmt.Errorf("update item (persist): %w", err)
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
