package services

import (
	"borscht.app/smetana/domain"
)

type TokenService struct {
	repo domain.UserRepository
}

func NewTokenService(repo domain.UserRepository) *TokenService {
	return &TokenService{repo: repo}
}

// CreateRefreshToken persists a new refresh token for a user.
func (s *TokenService) CreateRefreshToken(token *domain.UserToken) error {
	return s.repo.CreateRefreshToken(token)
}

// ByRefreshToken retrieves a refresh token with its associated user.
func (s *TokenService) ByRefreshToken(tokenStr string) (*domain.UserToken, error) {
	return s.repo.FindRefreshToken(tokenStr)
}

// DeleteRefreshToken permanently removes a refresh token.
func (s *TokenService) DeleteRefreshToken(tokenStr string) error {
	return s.repo.DeleteRefreshToken(tokenStr)
}
