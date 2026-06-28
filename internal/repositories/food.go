package repositories

import (
	"fmt"

	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/utils"
)

type foodRepository struct {
	db *gorm.DB
}

func NewFoodRepository(db *gorm.DB) domain.FoodRepository {
	return &foodRepository{db: db}
}

func (r *foodRepository) ByID(id uuid.UUID) (*domain.Food, error) {
	var food domain.Food
	if err := r.db.First(&food, id).Error; err != nil {
		return nil, fmt.Errorf("food by id %s: %w", id, mapErr(err))
	}
	return &food, nil
}

func (r *foodRepository) ByIDs(ids []uuid.UUID) (map[uuid.UUID]*domain.Food, error) {
	if len(ids) == 0 {
		return make(map[uuid.UUID]*domain.Food), nil
	}

	var foods []domain.Food
	if err := r.db.Where("id IN ?", ids).Find(&foods).Error; err != nil {
		return nil, fmt.Errorf("foods by ids: %w", mapErr(err))
	}

	result := make(map[uuid.UUID]*domain.Food, len(foods))
	for i := range foods {
		result[foods[i].ID] = &foods[i]
	}
	return result, nil
}

func (r *foodRepository) FindOrCreate(food *domain.Food) error {
	if food.Slug == "" {
		food.Slug = utils.CreateTag(food.Name)
	}

	if err := r.db.First(food, "slug = ?", food.Slug).Error; err == nil {
		return r.resolveAlias(food)
	}

	if err := r.db.First(food, "lower(name) = lower(?)", food.Name).Error; err == nil {
		return r.resolveAlias(food)
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(food)
	if result.Error != nil {
		return fmt.Errorf("create food: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		if err := r.db.First(food, "slug = ?", food.Slug).Error; err != nil {
			return fmt.Errorf("find food after conflict: %w", mapErr(err))
		}
		return r.resolveAlias(food)
	}

	return nil
}

// resolveAlias replaces food with its canonical target when the row is an alias.
func (r *foodRepository) resolveAlias(food *domain.Food) error {
	if food.CanonicalFoodID == nil {
		return nil
	}
	var canonical domain.Food
	if err := r.db.First(&canonical, *food.CanonicalFoodID).Error; err != nil {
		return fmt.Errorf("resolve food alias: %w", mapErr(err))
	}
	*food = canonical
	return nil
}

func (r *foodRepository) Search(query string, offset, limit int) ([]domain.Food, int64, error) {
	db := r.db.Model(&domain.Food{}).Scopes(IsCanonicalFood)
	if query != "" {
		db = db.Where("name LIKE ? OR slug LIKE ?", "%"+query+"%", "%"+query+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("search count foods: %w", mapErr(err))
	}

	var foods []domain.Food
	if err := db.Offset(offset).Limit(limit).Find(&foods).Error; err != nil {
		return nil, 0, fmt.Errorf("search find foods: %w", mapErr(err))
	}
	return foods, total, nil
}

func (r *foodRepository) Merge(keepID, mergeID uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var keep, merge domain.Food
		if err := tx.Scopes(IsCanonicalFood).First(&keep, keepID).Error; err != nil {
			return fmt.Errorf("keep food: %w", mapErr(err))
		}
		if err := tx.Scopes(IsCanonicalFood).First(&merge, mergeID).Error; err != nil {
			return fmt.Errorf("merge food: %w", mapErr(err))
		}

		if err := tx.Model(&domain.RecipeIngredient{}).Where("food_id = ?", mergeID).Update("food_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign ingredients: %w", mapErr(err))
		}
		if err := tx.Model(&domain.ShoppingItem{}).Where("food_id = ?", mergeID).Update("food_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign shopping items: %w", mapErr(err))
		}
		if err := tx.Model(&domain.FoodPrice{}).Where("food_id = ?", mergeID).Update("food_id", keepID).Error; err != nil {
			return fmt.Errorf("reassign food prices: %w", mapErr(err))
		}

		var existingKeep []domain.FoodUnitConversion
		if err := tx.Where("food_id = ?", keepID).Find(&existingKeep).Error; err != nil {
			return fmt.Errorf("fetch keep conversions: %w", mapErr(err))
		}
		keepUnitIDs := make(map[uuid.UUID]bool)
		for _, c := range existingKeep {
			keepUnitIDs[c.UnitID] = true
		}

		var mergeConversions []domain.FoodUnitConversion
		if err := tx.Where("food_id = ?", mergeID).Find(&mergeConversions).Error; err != nil {
			return fmt.Errorf("fetch merge conversions: %w", mapErr(err))
		}
		var deleteIDs, reassignIDs []uuid.UUID
		for _, c := range mergeConversions {
			if keepUnitIDs[c.UnitID] {
				deleteIDs = append(deleteIDs, c.ID)
			} else {
				reassignIDs = append(reassignIDs, c.ID)
			}
		}
		if len(deleteIDs) > 0 {
			if err := tx.Delete(&domain.FoodUnitConversion{}, "id IN ?", deleteIDs).Error; err != nil {
				return fmt.Errorf("delete duplicate conversions: %w", mapErr(err))
			}
		}
		if len(reassignIDs) > 0 {
			if err := tx.Model(&domain.FoodUnitConversion{}).Where("id IN ?", reassignIDs).Update("food_id", keepID).Error; err != nil {
				return fmt.Errorf("reassign conversions: %w", mapErr(err))
			}
		}

		// Preserve fields from the discarded food if the kept food lacks them.
		inherited := map[string]any{}
		if keep.ImagePath == nil && merge.ImagePath != nil {
			inherited["image_path"] = merge.ImagePath
		}
		if keep.DefaultUnitID == nil && merge.DefaultUnitID != nil {
			inherited["default_unit_id"] = merge.DefaultUnitID
		}
		if !keep.Pantry && merge.Pantry {
			inherited["pantry"] = true
		}
		if len(inherited) > 0 {
			if err := tx.Model(&domain.Food{}).Where("id = ?", keepID).Updates(inherited).Error; err != nil {
				return fmt.Errorf("propagate fields to keep food: %w", mapErr(err))
			}
		}

		if err := tx.Model(&domain.Food{}).Where("id = ?", mergeID).Update("canonical_food_id", keepID).Error; err != nil {
			return fmt.Errorf("set food alias: %w", mapErr(err))
		}
		return nil
	})
}

func (r *foodRepository) Update(food *domain.Food) error {
	if err := r.db.Model(food).Select("name", "description", "default_unit_id", "pantry").Updates(food).Error; err != nil {
		return fmt.Errorf("update food %s: %w", food.ID, mapErr(err))
	}
	return nil
}

func (r *foodRepository) AddTaxonomy(foodID uuid.UUID, taxonomy *domain.Taxonomy) error {
	if err := r.db.Model(&domain.Food{ID: foodID}).Association("Taxonomies").Append(taxonomy); err != nil {
		return fmt.Errorf("add taxonomy %s to food %s: %w", taxonomy.ID, foodID, mapErr(err))
	}
	return nil
}

func (r *foodRepository) CreatePrice(price *domain.FoodPrice) error {
	if err := r.db.Create(price).Error; err != nil {
		return fmt.Errorf("create food price: %w", mapErr(err))
	}
	return nil
}

func (r *foodRepository) LatestPrices(householdID uuid.UUID, foodIDs []uuid.UUID) (map[uuid.UUID]*domain.FoodPrice, error) {
	if len(foodIDs) == 0 {
		return map[uuid.UUID]*domain.FoodPrice{}, nil
	}

	// Subquery: max created per food within the household.
	sub := r.db.Model(&domain.FoodPrice{}).
		Select("food_id, MAX(created) AS max_created").
		Where("household_id = ? AND food_id IN ?", householdID, foodIDs).
		Group("food_id")

	var prices []domain.FoodPrice
	err := r.db.
		Joins("JOIN (?) AS latest ON food_prices.food_id = latest.food_id AND food_prices.created = latest.max_created", sub).
		Where("food_prices.household_id = ?", householdID).
		Preload("Unit").
		Find(&prices).Error
	if err != nil {
		return nil, fmt.Errorf("find latest prices for household %s: %w", householdID, mapErr(err))
	}

	result := make(map[uuid.UUID]*domain.FoodPrice, len(prices))
	for i := range prices {
		result[prices[i].FoodID] = &prices[i]
	}
	return result, nil
}

func (r *foodRepository) ListPrices(householdID, foodID uuid.UUID, opts types.Pagination) ([]domain.FoodPrice, int64, error) {
	var prices []domain.FoodPrice
	var total int64

	q := r.db.Model(&domain.FoodPrice{}).
		Where("household_id = ? AND food_id = ?", householdID, foodID)

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count prices for food %s: %w", foodID, mapErr(err))
	}

	err := q.Order("created DESC").
		Limit(opts.Limit).
		Offset(opts.Offset).
		Preload("Unit").
		Find(&prices).Error

	if err != nil {
		return nil, 0, fmt.Errorf("find prices for food %s: %w", foodID, mapErr(err))
	}

	return prices, total, nil
}

func (r *foodRepository) DeletePrice(householdID, id uuid.UUID) error {
	if err := r.db.Delete(&domain.FoodPrice{}, "id = ? AND household_id = ?", id, householdID).Error; err != nil {
		return fmt.Errorf("delete food price %s: %w", id, mapErr(err))
	}
	return nil
}

func (r *foodRepository) CreateConversion(c *domain.FoodUnitConversion) error {
	if err := r.db.Create(c).Error; err != nil {
		return fmt.Errorf("create conversion: %w", mapErr(err))
	}
	return nil
}

func (r *foodRepository) ListConversions(foodID uuid.UUID) ([]domain.FoodUnitConversion, error) {
	var list []domain.FoodUnitConversion
	if err := r.db.Where("food_id = ?", foodID).Preload("TargetUnit").Preload("Unit").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list conversions: %w", mapErr(err))
	}
	return list, nil
}

func (r *foodRepository) UpdateConversion(c *domain.FoodUnitConversion) error {
	result := r.db.Model(c).Select("target_amount", "target_unit_id").Updates(c)
	if result.Error != nil {
		return fmt.Errorf("update conversion: %w", mapErr(result.Error))
	}
	if result.RowsAffected == 0 {
		return sentinels.ErrNotFound
	}
	return nil
}

func (r *foodRepository) DeleteConversion(foodID, id uuid.UUID) error {
	if err := r.db.Delete(&domain.FoodUnitConversion{}, "id = ? AND food_id = ?", id, foodID).Error; err != nil {
		return fmt.Errorf("delete conversion: %w", mapErr(err))
	}
	return nil
}

func (r *foodRepository) ConversionsByFoodIDs(foodIDs []uuid.UUID) (map[uuid.UUID][]domain.FoodUnitConversion, error) {
	if len(foodIDs) == 0 {
		return map[uuid.UUID][]domain.FoodUnitConversion{}, nil
	}
	var list []domain.FoodUnitConversion
	if err := r.db.Where("food_id IN ?", foodIDs).Find(&list).Error; err != nil {
		return nil, fmt.Errorf("conversions by food ids: %w", mapErr(err))
	}

	result := make(map[uuid.UUID][]domain.FoodUnitConversion)
	for i := range list {
		fid := list[i].FoodID
		result[fid] = append(result[fid], list[i])
	}
	return result, nil
}
