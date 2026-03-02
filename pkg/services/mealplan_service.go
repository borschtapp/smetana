package services

import (
	"time"

	"borscht.app/smetana/domain"
	"github.com/gofiber/fiber/v3/log"
	"github.com/google/uuid"
)

type MealPlanService struct {
	repo domain.MealPlanRepository
}

func NewMealPlanService(repo domain.MealPlanRepository) *MealPlanService {
	return &MealPlanService{repo: repo}
}

func (s *MealPlanService) ByIdWithRecipes(id uuid.UUID) (*domain.MealPlan, error) {
	return s.repo.ByIdWithRecipes(id)
}

func (s *MealPlanService) List(householdID uuid.UUID, from, to *time.Time, offset, limit int) ([]domain.MealPlan, int64, error) {
	return s.repo.List(householdID, from, to, offset, limit)
}

func (s *MealPlanService) Create(mealPlan *domain.MealPlan) error {
	if err := s.repo.Create(mealPlan); err != nil {
		return err
	}

	if mealPlan.RecipeID != nil {
		if fetched, err := s.repo.ByIdWithRecipes(mealPlan.ID); err != nil {
			log.Warnf("failed to reload meal plan %s after write: %v", mealPlan.ID, err)
		} else {
			mealPlan.Recipe = fetched.Recipe
		}
	}

	return nil
}

func (s *MealPlanService) Update(mealPlan *domain.MealPlan) error {
	if err := s.repo.Update(mealPlan); err != nil {
		return err
	}

	if mealPlan.RecipeID != nil {
		if fetched, err := s.repo.ByIdWithRecipes(mealPlan.ID); err != nil {
			log.Warnf("failed to reload meal plan %s after write: %v", mealPlan.ID, err)
		} else {
			mealPlan.Recipe = fetched.Recipe
		}
	}

	return nil
}

func (s *MealPlanService) Delete(id uuid.UUID) error {
	return s.repo.Delete(id)
}
