package repositories

import (
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
)

type RecipeRepository struct {
	db *gorm.DB
}

func NewRecipeRepository(db *gorm.DB) *RecipeRepository {
	return &RecipeRepository{db: db}
}

func (r *RecipeRepository) ByID(id uuid.UUID) (*domain.Recipe, error) {
	var recipe domain.Recipe
	if err := r.db.
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipe, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &recipe, nil
}

func (r *RecipeRepository) ByUrl(url string) (*domain.Recipe, error) {
	var recipe domain.Recipe
	if err := r.db.Where(&domain.Recipe{IsBasedOn: &url}).
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipe).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrRecordNotFound
		}
		return nil, err
	}
	return &recipe, nil
}

func (r *RecipeRepository) Create(recipe *domain.Recipe) error {
	return r.db.Create(recipe).Error
}

func (r *RecipeRepository) Import(recipe *domain.Recipe) error {
	return r.db.Omit("Publisher", "Images", "Ingredients.Food", "Ingredients.Unit").Create(recipe).Error
}

func (r *RecipeRepository) CreateImages(images []*domain.RecipeImage) error {
	return r.db.Create(images).Error
}

func (r *RecipeRepository) UpdateImage(img *domain.RecipeImage) error {
	return r.db.Model(img).Updates(img).Error
}

func (r *RecipeRepository) Update(recipe *domain.Recipe) error {
	return r.db.Model(recipe).Updates(recipe).Error
}

func (r *RecipeRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.Recipe{}, id).Error
}

func (r *RecipeRepository) UserSave(userID uuid.UUID, recipeID uuid.UUID, householdID uuid.UUID) error {
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.RecipeSaved{
		UserID:      userID,
		RecipeID:    recipeID,
		HouseholdID: householdID,
	}).Error
}

func (r *RecipeRepository) ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID) ([]domain.Recipe, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	var recipes []domain.Recipe
	err := r.db.
		Where("parent_id IN ? AND household_id = ?", parentIDs, householdID).
		Find(&recipes).Error
	if err != nil {
		return nil, err
	}
	return recipes, nil
}

func (r *RecipeRepository) UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error {
	return r.db.Delete(&domain.RecipeSaved{}, "user_id = ? AND recipe_id = ?", userID, recipeID).Error
}

func (r *RecipeRepository) UserSearch(userID uuid.UUID, householdID uuid.UUID, q string, taxonomies []string, cuisine string, offset, limit int) ([]domain.Recipe, int64, error) {
	var recipes []domain.Recipe

	baseQuery := r.db.Model(&domain.Recipe{}).
		Joins("JOIN recipe_saved ON recipe_saved.recipe_id = recipes.id").
		Where("recipe_saved.household_id = ?", householdID)

	if q != "" {
		baseQuery = baseQuery.Where("recipes.name LIKE ? OR recipes.description LIKE ?", "%"+q+"%", "%"+q+"%")
	}

	if cuisine != "" {
		baseQuery = baseQuery.Joins("Left JOIN recipe_taxonomies as rt_cuisine ON rt_cuisine.recipe_id = recipes.id").
			Joins("Left JOIN taxonomies as t_cuisine ON t_cuisine.id = rt_cuisine.taxonomy_id").
			Where("t_cuisine.type = ? AND t_cuisine.slug = ?", "cuisine", cuisine)
	}

	if len(taxonomies) > 0 {
		baseQuery = baseQuery.Joins("Left JOIN recipe_taxonomies as rt_tax ON rt_tax.recipe_id = recipes.id").
			Joins("Left JOIN taxonomies as t_tax ON t_tax.id = rt_tax.taxonomy_id").
			Where("t_tax.slug IN ?", taxonomies)
	}

	var total int64
	if err := baseQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := baseQuery.
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		Order("recipes.updated DESC").
		Offset(offset).
		Limit(limit).
		Find(&recipes).Error; err != nil {
		return nil, 0, err
	}
	return recipes, total, nil
}

func (r *RecipeRepository) CreateIngredient(ingredient *domain.RecipeIngredient) error {
	return r.db.Create(ingredient).Error
}

func (r *RecipeRepository) UpdateIngredient(ingredient *domain.RecipeIngredient) error {
	return r.db.Model(ingredient).Updates(ingredient).Error
}

func (r *RecipeRepository) DeleteIngredient(id uuid.UUID) error {
	return r.db.Delete(&domain.RecipeIngredient{}, id).Error
}

func (r *RecipeRepository) CreateInstruction(instruction *domain.RecipeInstruction) error {
	return r.db.Create(instruction).Error
}

func (r *RecipeRepository) UpdateInstruction(instruction *domain.RecipeInstruction) error {
	return r.db.Model(instruction).Updates(instruction).Error
}

func (r *RecipeRepository) DeleteInstruction(id uuid.UUID) error {
	return r.db.Delete(&domain.RecipeInstruction{}, id).Error
}

func (r *RecipeRepository) Transaction(fn func(txRepo domain.RecipeRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := NewRecipeRepository(tx)
		return fn(txRepo)
	})
}

func (r *RecipeRepository) ReplaceRecipePointers(oldRecipeID, newRecipeID, householdID uuid.UUID) error {
	// 1. RecipeSaved
	if err := r.db.Table("recipe_saved").Where("recipe_id = ? AND household_id = ?", oldRecipeID, householdID).Update("recipe_id", newRecipeID).Error; err != nil {
		return err
	}
	// 2. MealPlan
	if err := r.db.Table("meal_plans").Where("recipe_id = ? AND household_id = ?", oldRecipeID, householdID).Update("recipe_id", newRecipeID).Error; err != nil {
		return err
	}
	// 3. CollectionRecipes
	if err := r.db.Table("collection_recipes").
		Where("recipe_id = ? AND collection_id IN (SELECT id FROM collections WHERE household_id = ?)", oldRecipeID, householdID).
		Update("recipe_id", newRecipeID).Error; err != nil {
		return err
	}
	return nil
}
