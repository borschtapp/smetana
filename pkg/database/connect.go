package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/pkg/configs"
	"borscht.app/smetana/pkg/utils"
)

func Migrate() error {
	err := DB.AutoMigrate(
		&domain.User{},
		&domain.UserToken{},
		&domain.Recipe{},
		&domain.RecipeImage{},
		&domain.RecipeInstruction{},
		&domain.RecipeIngredient{},
		&domain.Publisher{},
		&domain.PublisherTag{},
		&domain.Unit{},
		&domain.UnitTag{},
		&domain.Food{},
		&domain.FoodTag{},
		&domain.FoodCategory{},
	)
	return err
}

func Connect() (err error) {
	conf := configs.GormConfig()
	DB, err = gorm.Open(selectDialect(), &conf)
	return
}

func selectDialect() gorm.Dialector {
	dbType := utils.Getenv("DB_TYPE", "sqlite")

	if dbType == "mysql" || dbType == "mariadb" {
		return dialectMysql()
	}

	return dialectSqlLite()
}

func dialectSqlLite() gorm.Dialector {
	return sqlite.Open("tmp/smetana.db")
}

func dialectMysql() gorm.Dialector {
	dbHost := utils.Getenv("DB_HOST", "localhost")
	dbPort := utils.GetenvInt("DB_PORT", 3306)
	dbUser := utils.Getenv("DB_USER", "smetana")
	dbPassword := utils.Getenv("DB_PASSWORD", "smetana")
	dbName := utils.Getenv("DB_NAME", "smetana")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	return mysql.Open(dsn)
}
