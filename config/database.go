package config

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"borscht.app/smetana/model"
)

var (
	err error
	DB  *gorm.DB
)

func MigrateDB() error {
	err := DB.AutoMigrate(
		&model.User{},
		&model.Book{},
	)
	return err
}

func ConnectSqlLite() error {
	DB, err = gorm.Open(sqlite.Open("tmp/smetana.db"), &gorm.Config{})
	if err != nil {
		return err
	}

	return nil
}

func ConnectMysql() error {
	DSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		Env.Database.Username, Env.Database.Password, Env.Database.Host, Env.Database.Port, Env.Database.Name)
	DB, err = gorm.Open(mysql.Open(DSN), &gorm.Config{})
	if err != nil {
		return err
	}

	return nil
}
