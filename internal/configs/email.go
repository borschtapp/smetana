package configs

import (
	"fmt"
	"os"

	"borscht.app/smetana/internal/utils"
)

type EmailConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	BaseURL  string
}

func NewEmail() (EmailConfig, error) {
	host := os.Getenv("SMTP_HOST")
	if host == "" {
		return EmailConfig{}, fmt.Errorf("SMTP_HOST is not set")
	}

	return EmailConfig{
		Host:     host,
		Port:     utils.GetenvInt("SMTP_PORT", 587),
		User:     os.Getenv("SMTP_USER"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		BaseURL:  appBaseURL(),
	}, nil
}

func appBaseURL() string {
	if base := os.Getenv("BASE_URL"); base != "" {
		return base
	}
	return fmt.Sprintf("https://%s", utils.Getenv("SERVER_HOST", "localhost"))
}
