package services

import (
	"fmt"
	"net/smtp"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/configs"
	"github.com/gofiber/fiber/v3/log"
)

type EmailService struct {
	cfg configs.EmailConfig
}

func NewEmailService() (domain.EmailService, error) {
	cfg, err := configs.NewEmail()
	if err != nil {
		return nil, err
	}

	log.Tracew("EmailService initialized", "emailHost", cfg.Host, "emailFrom", cfg.From, "baseURL", cfg.BaseURL)
	return &EmailService{cfg: cfg}, nil
}

func (s *EmailService) sendMail(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.cfg.From, to, subject, body)
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
	return smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg))
}

func (s *EmailService) SendPasswordReset(to, rawToken string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.BaseURL, rawToken)
	body := fmt.Sprintf("Click the link below to reset your password (valid for 1 hour):\n\n%s\n\nIf you did not request a password reset, you can ignore this email.", resetURL)
	return s.sendMail(to, "Password Reset Request", body)
}

func (s *EmailService) SendHouseholdInvite(to, code string) error {
	joinURL := fmt.Sprintf("%s/join?code=%s", s.cfg.BaseURL, code)
	body := fmt.Sprintf("You've been invited to join a household!\n\nClick the link below to accept (valid for 7 days):\n\n%s\n\nIf you did not expect this invitation, you can ignore this email.", joinURL)
	return s.sendMail(to, "Household Invitation", body)
}
