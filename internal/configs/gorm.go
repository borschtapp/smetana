package configs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3/log"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"borscht.app/smetana/internal/utils"
)

type databaseEnvVars struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string // For Postgres
}

var dialectRegistry = map[string]func(databaseEnvVars) gorm.Dialector{
	"mysql": func(cfg databaseEnvVars) gorm.Dialector {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)
		return mysql.Open(dsn)
	},
	"postgres": func(cfg databaseEnvVars) gorm.Dialector {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=UTC",
			cfg.Host, cfg.User, cfg.Password, cfg.Name, cfg.Port, cfg.SSLMode)
		return postgres.Open(dsn)
	},
	"sqlite": func(cfg databaseEnvVars) gorm.Dialector {
		_ = os.MkdirAll(filepath.Dir(cfg.Name), 0700)
		return sqlite.Open(cfg.Name)
	},
}

func GormConfig() (gorm.Config, gorm.Dialector) {
	enableLogger := utils.GetenvBool("GORM_ENABLE_LOGGER", false)
	envVars := databaseEnvVars{
		Type:     utils.Getenv("DB_TYPE", "sqlite"),
		Host:     utils.Getenv("DB_HOST", "localhost"),
		Port:     utils.GetenvInt("DB_PORT", 3306),
		Name:     utils.Getenv("DB_NAME", "./data/borscht.db"),
		User:     utils.Getenv("DB_USER", ""),
		Password: utils.Getenv("DB_PASSWORD", ""),
		SSLMode:  utils.Getenv("DB_SSLMODE", "disable"),
	}

	config := gorm.Config{
		TranslateError: true,
	}
	if enableLogger {
		config.Logger = logger.Default.LogMode(logger.Info)
	}

	factory, ok := dialectRegistry[envVars.Type]
	if !ok {
		log.Fatalw("unsupported DB_TYPE", "type", envVars.Type, "supported", "sqlite, mysql, postgres")
	}
	log.Infow("Database initialized", "type", envVars.Type, "host", envVars.Host, "name", envVars.Name)
	return config, factory(envVars)
}
