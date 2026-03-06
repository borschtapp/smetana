package database

import (
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/configs"
)

func Connect() (*gorm.DB, error) {
	gcfg, gdial := configs.GormConfig()
	db, err := gorm.Open(gdial, &gcfg)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&domain.Household{},
		&domain.User{},
		&domain.UserToken{},
		&domain.Publisher{},
		&domain.Unit{},
		&domain.Food{},
		&domain.Taxonomy{},
		&domain.Recipe{},
		&domain.RecipeImage{},
		&domain.RecipeInstruction{},
		&domain.RecipeIngredient{},
		&domain.RecipeSaved{},
		&domain.MealPlan{},
		&domain.Collection{},
		&domain.ShoppingList{},
		&domain.Feed{},
	)
	return err
}
