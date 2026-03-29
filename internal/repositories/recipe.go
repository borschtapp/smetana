package repositories

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/types"
)

type recipeRepository struct {
	db *gorm.DB
}

func NewRecipeRepository(db *gorm.DB) domain.RecipeRepository {
	return &recipeRepository{db: db}
}

// applyPreloads requires userID and householdID for "collections" and "saved" preloads.
func applyPreloads(q *gorm.DB, preload types.PreloadOptions, userID, householdID uuid.UUID) *gorm.DB {
	if len(preload.Preload) == 0 {
		return q
	}

	if preload.HasAny("ingredients", "all") {
		q = q.Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		})
	}

	if preload.HasAny("collections", "all") && householdID != uuid.Nil {
		q = q.Preload("Collections", "household_id = ?", householdID)
	}

	if preload.HasAny("saved", "all") && userID != uuid.Nil {
		q = q.Select(`recipes.*, EXISTS(
				SELECT 1 FROM recipes_saved
				WHERE recipes_saved.recipe_id = recipes.id
				AND recipes_saved.user_id = ?
			) AS is_saved`, userID)
	}

	if preload.Has("all") {
		q = q.Preload(clause.Associations)
		return q
	}

	if preload.Has("publisher") {
		q = q.Preload("Publisher")
	}

	if preload.Has("author") {
		q = q.Preload("Author")
	}

	if preload.Has("feed") {
		q = q.Preload("Feed")
	}

	if preload.Has("images") {
		q = q.Preload("Images")
	}

	if preload.Has("equipment") {
		q = q.Preload("Equipment")
	}

	if preload.Has("instructions") {
		q = q.Preload("Instructions")
	}

	if preload.Has("nutrition") {
		q = q.Preload("Nutrition")
	}

	if preload.Has("taxonomies") {
		q = q.Preload("Taxonomies")
	}

	return q
}

func (r *recipeRepository) ByID(id uuid.UUID) (*domain.Recipe, error) {
	var recipe domain.Recipe
	if err := r.db.First(&recipe, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &recipe, nil
}

func (r *recipeRepository) ByIDPreload(id uuid.UUID, userID, householdID uuid.UUID, preload types.PreloadOptions) (*domain.Recipe, error) {
	var recipe domain.Recipe
	q := applyPreloads(r.db, preload, userID, householdID)
	if err := q.First(&recipe, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &recipe, nil
}

func (r *recipeRepository) ByUrl(url string) (*domain.Recipe, error) {
	var recipe domain.Recipe
	if err := r.db.Where(&domain.Recipe{SourceUrl: &url}).First(&recipe).Error; err != nil {
		return nil, mapErr(err)
	}
	return &recipe, nil
}

func (r *recipeRepository) Search(userID uuid.UUID, householdID uuid.UUID, opts domain.RecipeSearchOptions) ([]domain.Recipe, int64, error) {
	var recipes []domain.Recipe

	// base filter, only show recipes saved by someone from the household
	q := r.db.Model(&domain.Recipe{})

	if opts.CollectionID != uuid.Nil {
		q = q.Joins("JOIN collection_recipes ON collection_recipes.recipe_id = recipes.id").
			Where("collection_recipes.collection_id = ?", opts.CollectionID)
	} else if opts.FromFeeds {
		q = q.Joins("JOIN feeds ON feeds.id = recipes.feed_id").
			Joins("JOIN feed_subscriptions ON feed_subscriptions.feed_id = feeds.id").
			Where("feed_subscriptions.household_id = ?", householdID)
	} else {
		q = q.Joins("JOIN recipes_saved ON recipes_saved.recipe_id = recipes.id").
			Where("recipes_saved.household_id = ?", householdID)
	}

	// apply filters/search options
	if opts.SearchQuery != "" {
		q = q.Where("recipes.name LIKE ? OR recipes.description LIKE ?", "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
	}

	if len(opts.Taxonomies) > 0 {
		q = q.Joins("JOIN recipe_taxonomies ON recipe_taxonomies.recipe_id = recipes.id").
			Where("recipe_taxonomies.taxonomy_id IN ?", opts.Taxonomies)
	}

	if len(opts.Publishers) > 0 {
		q = q.Where("recipes.publisher_id IN ?", opts.Publishers)
	}

	if len(opts.Authors) > 0 {
		q = q.Where("recipes.author_id IN ?", opts.Authors)
	}

	if len(opts.Equipment) > 0 {
		q = q.Joins("JOIN recipe_equipment re_filter ON re_filter.recipe_id = recipes.id").
			Where("re_filter.equipment_id IN ?", opts.Equipment)
	}

	if opts.CookTimeMax != nil {
		q = q.Where("recipes.cook_time IS NOT NULL AND recipes.cook_time <= ?", int64(*opts.CookTimeMax))
	}

	if opts.TotalTimeMax != nil {
		q = q.Where("recipes.total_time IS NOT NULL AND recipes.total_time <= ?", int64(*opts.TotalTimeMax))
	}

	var total int64
	if err := q.Distinct("recipes.id").Count(&total).Error; err != nil {
		return nil, 0, mapErr(err)
	} else if total == 0 {
		return recipes, 0, nil
	}

	// is_saved is not a real DB column; use explicit SELECT to prevent GORM from emitting auto-generated column list.
	// The "saved" preload block below overrides this with the EXISTS subquery.
	// Distinct collapses duplicate rows produced by multi-value JOINs (taxonomies, equipment).
	q = q.Distinct().Select("recipes.*")

	// preload relations
	q = applyPreloads(q, opts.PreloadOptions, userID, householdID)

	// pagination
	q = q.Offset(opts.Offset).Limit(opts.Limit)

	// sorting
	q = q.Order(clause.OrderByColumn{
		Column: clause.Column{Table: "recipes", Name: opts.Sort},
		Desc:   strings.EqualFold(opts.Order, "DESC"),
	})

	if err := q.Find(&recipes).Error; err != nil {
		return nil, 0, mapErr(err)
	}

	return recipes, total, nil
}

func (r *recipeRepository) Create(recipe *domain.Recipe) error {
	return mapErr(r.db.Create(recipe).Error)
}

func (r *recipeRepository) Import(recipe *domain.Recipe) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(
			"Parent",
			"Author",
			"Publisher",
			"Feed",
			"Images",
			"Ingredients",
			"Instructions",
			"Equipment.*",
			"Taxonomies.*",
			"Collection.*",
		).Create(recipe).Error; err != nil {
			return mapErr(err)
		}

		if len(recipe.Ingredients) > 0 {
			for _, ing := range recipe.Ingredients {
				ing.RecipeID = recipe.ID
			}
			if err := tx.Omit(clause.Associations).Create(&recipe.Ingredients).Error; err != nil {
				return mapErr(err)
			}
		}

		if len(recipe.Instructions) > 0 {
			for _, inst := range recipe.Instructions {
				inst.RecipeID = recipe.ID
			}
			if err := tx.Omit(clause.Associations).Create(&recipe.Instructions).Error; err != nil {
				return mapErr(err)
			}
		}

		return nil
	})
}

func (r *recipeRepository) Update(recipe *domain.Recipe) error {
	return mapErr(r.db.Model(recipe).Updates(recipe).Error)
}

func (r *recipeRepository) Delete(id uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.Recipe{}, id).Error)
}

func (r *recipeRepository) UserSave(recipeID uuid.UUID, userID uuid.UUID, householdID uuid.UUID) error {
	return mapErr(r.db.Clauses(clause.OnConflict{DoNothing: true}).Omit(clause.Associations).Create(&domain.RecipeSaved{
		UserID:      userID,
		RecipeID:    recipeID,
		HouseholdID: householdID,
	}).Error)
}

func (r *recipeRepository) ByParentIDsAndHousehold(parentIDs []uuid.UUID, householdID uuid.UUID, preload types.PreloadOptions) ([]domain.Recipe, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	var recipes []domain.Recipe
	err := applyPreloads(r.db, preload, uuid.Nil, householdID).
		Where("parent_id IN ? AND household_id = ?", parentIDs, householdID).
		Find(&recipes).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return recipes, nil
}

func (r *recipeRepository) UserUnsave(recipeID uuid.UUID, userID uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.RecipeSaved{}, "user_id = ? AND recipe_id = ?", userID, recipeID).Error)
}

func (r *recipeRepository) CreateIngredient(ingredient *domain.RecipeIngredient) error {
	return mapErr(r.db.Create(ingredient).Error)
}

func (r *recipeRepository) UpdateIngredient(ingredient *domain.RecipeIngredient) error {
	return mapErr(r.db.Model(ingredient).Where("recipe_id = ?", ingredient.RecipeID).Updates(ingredient).Error)
}

func (r *recipeRepository) DeleteIngredient(id uuid.UUID, recipeID uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.RecipeIngredient{}, "id = ? AND recipe_id = ?", id, recipeID).Error)
}

func (r *recipeRepository) AddEquipment(recipeID uuid.UUID, equipmentID uuid.UUID) error {
	return mapErr(r.db.Model(&domain.Recipe{ID: recipeID}).Association("Equipment").Append(&domain.Equipment{ID: equipmentID}))
}

func (r *recipeRepository) RemoveEquipment(recipeID uuid.UUID, equipmentID uuid.UUID) error {
	return mapErr(r.db.Model(&domain.Recipe{ID: recipeID}).Association("Equipment").Delete(&domain.Equipment{ID: equipmentID}))
}

func (r *recipeRepository) CreateInstruction(instruction *domain.RecipeInstruction) error {
	return mapErr(r.db.Create(instruction).Error)
}

func (r *recipeRepository) UpdateInstruction(instruction *domain.RecipeInstruction) error {
	return mapErr(r.db.Model(instruction).Where("recipe_id = ?", instruction.RecipeID).Updates(instruction).Error)
}

func (r *recipeRepository) DeleteInstruction(id uuid.UUID, recipeID uuid.UUID) error {
	return mapErr(r.db.Delete(&domain.RecipeInstruction{}, "id = ? AND recipe_id = ?", id, recipeID).Error)
}

func (r *recipeRepository) Transaction(fn func(txRepo domain.RecipeRepository) error) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		txRepo := NewRecipeRepository(tx)
		return fn(txRepo)
	})
}

func (r *recipeRepository) ReplaceRecipePointers(oldRecipeID, newRecipeID, householdID uuid.UUID) error {
	// 1. RecipeSaved
	if err := r.db.Table("recipes_saved").Where("recipe_id = ? AND household_id = ?", oldRecipeID, householdID).Update("recipe_id", newRecipeID).Error; err != nil {
		return mapErr(err)
	}
	// 2. MealPlan
	if err := r.db.Table("meal_plans").Where("recipe_id = ? AND household_id = ?", oldRecipeID, householdID).Update("recipe_id", newRecipeID).Error; err != nil {
		return mapErr(err)
	}
	// 3. CollectionRecipes
	if err := r.db.Table("collection_recipes").
		Where("recipe_id = ? AND collection_id IN (SELECT id FROM collections WHERE household_id = ?)", oldRecipeID, householdID).
		Update("recipe_id", newRecipeID).Error; err != nil {
		return mapErr(err)
	}
	return nil
}
