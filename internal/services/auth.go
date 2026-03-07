package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/tokens"
	"borscht.app/smetana/internal/utils"
)

type AuthService struct {
	userRepo  domain.UserRepository
	dummyHash string
}

func NewAuthService(userRepo domain.UserRepository) domain.AuthService {
	// Pre-compute a hash so the login path always runs bcrypt regardless of
	// whether the email exists — prevents timing-based user enumeration.
	hash, _ := utils.HashPassword("dummy-password-for-timing-protection")
	return &AuthService{userRepo: userRepo, dummyHash: hash}
}

// Login validates credentials and returns the matching user.
func (s *AuthService) Login(email, password string) (*domain.User, error) {
	user, err := s.userRepo.ByEmail(email)
	if err != nil && !errors.Is(err, sentinels.ErrRecordNotFound) {
		return nil, err
	}

	hashToCheck := s.dummyHash
	if user != nil {
		hashToCheck = user.Password
	}

	if !utils.ValidatePassword(hashToCheck, password) || user == nil {
		return nil, sentinels.ErrUnauthorized
	}
	return user, nil
}

// Register creates a new user with a personal household.
func (s *AuthService) Register(name, email, password string) (*domain.User, error) {
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	user := &domain.User{
		Email:     email,
		Password:  hash,
		Name:      name,
		Household: &domain.Household{Name: name + "'s Household"},
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// IssueTokens generates a new access+refresh token pair and persists the refresh token.
func (s *AuthService) IssueTokens(user domain.User) (*domain.AuthTokens, error) {
	generatedTokens, err := tokens.GenerateNew(user.ID, user.HouseholdID)
	if err != nil {
		return nil, err
	}

	expiresIn := time.Minute * time.Duration(configs.JwtRefreshExpireMinutes())
	token := &domain.UserToken{
		UserID:  user.ID,
		Type:    "refresh",
		Token:   generatedTokens.Refresh,
		Expires: time.Now().Add(expiresIn),
	}

	if err := s.userRepo.CreateToken(token); err != nil {
		return nil, err
	}
	return generatedTokens, nil
}

// RotateRefreshToken validates a refresh token, invalidates it, and issues a new pair.
func (s *AuthService) RotateRefreshToken(tokenStr string) (*domain.User, *domain.AuthTokens, error) {
	userToken, err := s.userRepo.FindToken(tokenStr, "refresh")
	if err != nil {
		return nil, nil, sentinels.ErrUnauthorized
	}
	if !time.Now().Before(userToken.Expires) {
		return nil, nil, sentinels.ErrUnauthorized
	}
	user := userToken.User
	if user == nil {
		return nil, nil, sentinels.ErrUnauthorized
	}

	if err := s.userRepo.DeleteToken(userToken.Token); err != nil {
		return nil, nil, err
	}
	generatedTokens, err := s.IssueTokens(*user)
	if err != nil {
		return nil, nil, err
	}
	return user, generatedTokens, nil
}
