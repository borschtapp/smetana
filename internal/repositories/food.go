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

func (r *foodRepository) FindOrCreate(food *domain.Food) error {
	if food.Slug == "" {
		food.Slug = utils.CreateTag(food.Name)
	}

	if err := r.db.First(food, "slug = ?", food.Slug).Error; err == nil {
		return nil
	}

	if err := r.db.First(food, "lower(name) = lower(?)", food.Name).Error; err == nil {
		return nil
	}

	result := r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(food)
	if result.Error != nil {
		return fmt.Errorf("create food: %w", mapErr(result.Error))
	}

	if result.RowsAffected == 0 { // DoNothing triggered: conflict; BeforeCreate already assigned a stale ID
		if err := r.db.First(food, "slug = ?", food.Slug).Error; err != nil {
			return fmt.Errorf("find food after conflict: %w", mapErr(err))
		}
	}

	return nil
}

func (r *foodRepository) Update(food *domain.Food) error {
	if err := r.db.Model(food).Select("name", "image_path", "default_unit_id").Updates(food).Error; err != nil {
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
