package services

import (
	"sync"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/database"

	"github.com/borschtapp/kapusta"
	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/model"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RecipeService struct {
	imageService     *ImageService
	publisherService *PublisherService
	foodService      *FoodService
	unitService      *UnitService
}

func NewRecipeService(imageService *ImageService, publisherService *PublisherService, foodService *FoodService, unitService *UnitService) *RecipeService {
	return &RecipeService{
		imageService:     imageService,
		publisherService: publisherService,
		foodService:      foodService,
		unitService:      unitService,
	}
}

func (s *RecipeService) GetUserRecipes(userID uuid.UUID, q string, taxonomies []string, cuisine string, offset, limit int) ([]domain.Recipe, int64, error) {
	var recipes []domain.Recipe

	baseQuery := database.DB.Model(&domain.Recipe{}).
		Joins("JOIN recipe_saved ON recipe_saved.recipe_id = recipes.id").
		Where("recipe_saved.user_id = ?", userID)

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

func (s *RecipeService) GetRecipe(id uuid.UUID) (*domain.Recipe, error) {
	var recipe domain.Recipe
	if err := database.DB.
		Preload(clause.Associations).
		Preload("Ingredients", func(db *gorm.DB) *gorm.DB {
			return db.Joins("Food").Joins("Unit")
		}).
		First(&recipe, id).Error; err != nil {
		return nil, err
	}
	return &recipe, nil
}

func (s *RecipeService) CreateRecipe(recipe *domain.Recipe) error {
	return database.DB.Create(recipe).Error
}

func (s *RecipeService) UpdateRecipe(recipe *domain.Recipe) error {
	return database.DB.Model(recipe).Updates(recipe).Error
}

func (s *RecipeService) DeleteRecipe(id uuid.UUID) error {
	return database.DB.Delete(&domain.Recipe{}, id).Error
}

func (s *RecipeService) CanUserAccessRecipe(userID uuid.UUID, recipeID uuid.UUID) (bool, error) {
	var count int64
	err := database.DB.Table("recipe_saved").
		Where("user_id = ? AND recipe_id = ?", userID, recipeID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *RecipeService) SaveRecipe(userID uuid.UUID, recipeID uuid.UUID) error {
	var user domain.User
	if err := database.DB.Select("household_id").First(&user, userID).Error; err != nil {
		return err
	}

	return database.DB.Create(&domain.RecipeSaved{
		UserID:      userID,
		RecipeID:    recipeID,
		HouseholdID: user.HouseholdID,
	}).Error
}

func (s *RecipeService) UnsaveRecipe(userID uuid.UUID, recipeID uuid.UUID) error {
	return database.DB.Delete(&domain.RecipeSaved{}, "user_id = ? AND recipe_id = ?", userID, recipeID).Error
}

func (s *RecipeService) CreateIngredient(ingredient *domain.RecipeIngredient) error {
	return database.DB.Create(ingredient).Error
}

func (s *RecipeService) UpdateIngredient(ingredient *domain.RecipeIngredient) error {
	return database.DB.Model(ingredient).Updates(ingredient).Error
}

func (s *RecipeService) DeleteIngredient(id uuid.UUID) error {
	return database.DB.Delete(&domain.RecipeIngredient{}, id).Error
}

func (s *RecipeService) CreateInstruction(instruction *domain.RecipeInstruction) error {
	return database.DB.Create(instruction).Error
}

func (s *RecipeService) UpdateInstruction(instruction *domain.RecipeInstruction) error {
	return database.DB.Model(instruction).Updates(instruction).Error
}

func (s *RecipeService) DeleteInstruction(id uuid.UUID) error {
	return database.DB.Delete(&domain.RecipeInstruction{}, id).Error
}

func (s *RecipeService) ImportFromUrl(url string) (*domain.Recipe, error) {
	kripRecipe, err := krip.ScrapeUrl(url)
	if err != nil {
		return nil, err
	}
	return s.ImportFromKripRecipe(kripRecipe, nil)
}

func (s *RecipeService) ImportFromKripRecipe(kripRecipe *model.Recipe, feedID *uuid.UUID) (*domain.Recipe, error) {
	recipe := domain.FromKripRecipe(kripRecipe)
	recipe.FeedID = feedID

	if recipe.Publisher != nil {
		if err := s.publisherService.FindOrCreatePublisher(recipe.Publisher); err != nil {
			log.Warnf("error creating publisher %v: %s", recipe.Publisher, err.Error())
		} else {
			recipe.PublisherID = &recipe.Publisher.ID
		}
	}

	s.ParseAndEnrichIngredients(recipe.Ingredients, kripRecipe.Language)

	if err := database.DB.Omit("Publisher", "Images", "Ingredients.Food", "Ingredients.Unit").Create(&recipe).Error; err != nil {
		return nil, err
	}

	s.processRecipeImages(recipe)

	return recipe, nil
}

func (s *RecipeService) ParseAndEnrichIngredients(ingredients []*domain.RecipeIngredient, language string) {
	for _, ingredient := range ingredients {
		parsed, err := kapusta.ParseIngredient(ingredient.RawText, language)
		if err != nil || parsed == nil {
			continue
		}

		ingredient.Amount = &parsed.Quantity
		if len(parsed.Annotation) != 0 {
			ingredient.Note = &parsed.Annotation
		}
		if len(parsed.Ingredient) != 0 {
			food := &domain.Food{Name: parsed.Ingredient}
			if err := s.foodService.FindOrCreateFood(food); err != nil {
				log.Warnf("error creating food %v: %s", food, err.Error())
			} else {
				ingredient.Food = food
				ingredient.FoodID = &food.ID
			}
		}
		if len(parsed.Unit) != 0 {
			unit := &domain.Unit{Name: parsed.Unit}
			if err := s.unitService.FindOrCreateUnit(unit); err != nil {
				log.Warnf("error creating unit %v: %s", unit, err.Error())
			} else {
				ingredient.Unit = unit
				ingredient.UnitID = &unit.ID
			}
		}
	}
}

func (s *RecipeService) processRecipeImages(recipe *domain.Recipe) {
	if len(recipe.Images) == 0 {
		return
	}

	var wg sync.WaitGroup
	// Limit concurrent downloads to 5 to avoid resource exhaustion
	sem := make(chan struct{}, 5)

	for _, image := range recipe.Images {
		wg.Add(1)
		image.RecipeID = recipe.ID
		go func(img *domain.RecipeImage) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire token
			defer func() { <-sem }() // Release token

			basePath := "recipe/" + img.RecipeID.String() + "/" + img.ID.String()
			if info, err := s.imageService.DownloadAndPutImage(img.RemoteUrl, basePath); err != nil {
				log.Warnf("failed to download image: %v", err)
			} else {
				img.DownloadUrl = &info.Path
				img.Width = info.Width
				img.Height = info.Height
			}
		}(image)
	}

	// Save initial image records
	if err := database.DB.Create(recipe.Images).Error; err != nil {
		log.Warnf("failed to save images: %v", err)
	}

	// Wait for downloads to finish so we update dimensions/path
	wg.Wait()

	// Update image records with download info
	for _, img := range recipe.Images {
		if img.DownloadUrl != nil {
			database.DB.Save(img)
		}
	}
}
