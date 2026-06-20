package repositories

import (
	"fmt"

	"borscht.app/smetana/internal/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
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
