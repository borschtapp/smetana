package database

import (
	"borscht.app/smetana/pkg/utils"
)

type DatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string // For Postgres
}

func LoadConfig() DatabaseConfig {
	return DatabaseConfig{
		Type:     utils.Getenv("DB_TYPE", "sqlite"),
		Host:     utils.Getenv("DB_HOST", "localhost"),
		Port:     utils.GetenvInt("DB_PORT", 3306),
		Name:     utils.Getenv("DB_NAME", "./data/borscht.db"),
		User:     utils.Getenv("DB_USER", ""),
		Password: utils.Getenv("DB_PASSWORD", ""),
		SSLMode:  utils.Getenv("DB_SSLMODE", "disable"),
	}
}
