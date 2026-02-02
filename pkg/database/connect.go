package database

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/configs"
)

// DialectRegistry maps database type names to their dialector creators.
var DialectRegistry = map[string]func(DatabaseConfig) gorm.Dialector{
	"mysql": func(cfg DatabaseConfig) gorm.Dialector {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
		return mysql.Open(dsn)
	},
	"postgres": func(cfg DatabaseConfig) gorm.Dialector {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=UTC",
			cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port, cfg.SSLMode)
		return postgres.Open(dsn)
	},
	"sqlite": func(cfg DatabaseConfig) gorm.Dialector {
		return sqlite.Open(cfg.Name)
	},
}

func Migrate() error {
	err := DB.AutoMigrate(
		&domain.User{},
		&domain.UserToken{},
		&domain.Recipe{},
		&domain.RecipeImage{},
		&domain.RecipeInstruction{},
		&domain.RecipeIngredient{},
		&domain.Publisher{},
		&domain.Unit{},
		&domain.Food{},
		&domain.Taxonomy{},
		&domain.Household{},
		&domain.RecipeSaved{},
		&domain.MealPlan{},
		&domain.Collection{},
		&domain.ShoppingList{},
		&domain.Feed{},
	)
	return err
}

func Connect() (*gorm.DB, error) {
	cfg := LoadConfig()
	dialectCreator, ok := DialectRegistry[cfg.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	gcfg := configs.GormConfig()
	db, err := gorm.Open(dialectCreator(cfg), &gcfg)
	if err != nil {
		return nil, err
	}

	DB = db
	return db, nil
}
