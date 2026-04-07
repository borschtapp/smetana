package services

import (
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/sentinels"
)

type mealPlanService struct {
	repo domain.MealPlanRepository
}

func NewMealPlanService(repo domain.MealPlanRepository) domain.MealPlanService {
	return &mealPlanService{repo: repo}
}

func (s *mealPlanService) ByIDWithRecipes(id uuid.UUID, householdID uuid.UUID) (*domain.MealPlan, error) {
	mealPlan, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return nil, err
	}
	if mealPlan.HouseholdID != householdID {
		return nil, sentinels.ErrForbidden
	}
	return mealPlan, nil
}

func (s *mealPlanService) List(householdID uuid.UUID, from, to *time.Time, offset, limit int) ([]domain.MealPlan, int64, error) {
	return s.repo.List(householdID, from, to, offset, limit)
}

func (s *mealPlanService) Create(mealPlan *domain.MealPlan, householdID uuid.UUID) error {
	mealPlan.HouseholdID = householdID
	if err := s.repo.Create(mealPlan); err != nil {
		return err
	}

	if mealPlan.RecipeID != nil {
		if fetched, err := s.repo.ByIdWithRecipes(mealPlan.ID); err != nil {
			log.Warnw("failed to reload meal plan", "meal_plan", mealPlan.ID, "error", err.Error())
		} else {
			mealPlan.Recipe = fetched.Recipe
		}
	}

	return nil
}

func (s *mealPlanService) Update(mealPlan *domain.MealPlan, householdID uuid.UUID) error {
	existing, err := s.repo.ByIdWithRecipes(mealPlan.ID)
	if err != nil {
		return err
	}

	if existing.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}

	if err := s.repo.Update(mealPlan); err != nil {
		return err
	}

	if mealPlan.RecipeID != nil {
		if fetched, err := s.repo.ByIdWithRecipes(mealPlan.ID); err != nil {
			log.Warnw("failed to reload meal plan after update", "meal_plan", mealPlan.ID, "error", err.Error())
		} else {
			mealPlan.Recipe = fetched.Recipe
		}
	}

	return nil
}

func (s *mealPlanService) Delete(id uuid.UUID, householdID uuid.UUID) error {
	mealPlan, err := s.repo.ByIdWithRecipes(id)
	if err != nil {
		return err
	}
	if mealPlan.HouseholdID != householdID {
		return sentinels.ErrForbidden
	}
	return s.repo.Delete(id)
}
