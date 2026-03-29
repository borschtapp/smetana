package repositories

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
)

type mealPlanRepository struct {
	db *gorm.DB
}

func NewMealPlanRepository(db *gorm.DB) domain.MealPlanRepository {
	return &mealPlanRepository{db: db}
}

func (r *mealPlanRepository) ByIdWithRecipes(id uuid.UUID) (*domain.MealPlan, error) {
	var mealPlan domain.MealPlan
	if err := r.db.Preload("Recipe").First(&mealPlan, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &mealPlan, nil
}

func (r *mealPlanRepository) List(householdID uuid.UUID, from, to *time.Time, offset, limit int) ([]domain.MealPlan, int64, error) {
	query := r.db.Preload("Recipe").Where("household_id = ?", householdID)

	if from != nil {
		query = query.Where("date >= ?", *from)
	}
	if to != nil {
		query = query.Where("date <= ?", *to)
	}

	var total int64
	if err := query.Model(&domain.MealPlan{}).Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
	}

	var mealPlans []domain.MealPlan
	if err := query.Offset(offset).Limit(limit).Find(&mealPlans).Error; err != nil {
		return nil, 0, mapErr(err)
	}
	return mealPlans, total, nil
}

func (r *mealPlanRepository) Create(mealPlan *domain.MealPlan) error {
	return mapErr(r.db.Create(mealPlan).Error)
}

func (r *mealPlanRepository) Update(mealPlan *domain.MealPlan) error {
	return mapErr(r.db.Model(mealPlan).Updates(mealPlan).Error)
}

func (r *mealPlanRepository) Delete(id uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.MealPlan{}, id).Error)
}
