package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"borscht.app/smetana/domain"
	"borscht.app/smetana/internal/utils"
)

type AuthService struct {
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	userService  domain.UserService
}

func NewAuthService(userService domain.UserService) (*AuthService, error) {
	providerURL := utils.Getenv("OIDC_PROVIDER", "")
	clientID := utils.Getenv("OIDC_CLIENT_ID", "")
	clientSecret := utils.Getenv("OIDC_CLIENT_SECRET", "")
	redirectURL := utils.Getenv("OIDC_REDIRECT_URL", "")

	if providerURL == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("OIDC configuration missing")
	}

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query provider: %v", err)
	}

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})

	return &AuthService{
		provider:     provider,
		oauth2Config: conf,
		verifier:     verifier,
		userService:  userService,
	}, nil
}

func (s *AuthService) LoginURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state)
}

func (s *AuthService) Exchange(ctx context.Context, code string) (*oauth2.Token, *oidc.IDToken, error) {
	oauth2Token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to exchange token: %v", err)
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		return nil, nil, fmt.Errorf("no id_token field in oauth2 token")
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify ID Token: %v", err)
	}

	return oauth2Token, idToken, nil
}

// FindOrRegisterOIDCUser finds a user by email (with Household preloaded) or creates one via JIT provisioning.
func (s *AuthService) FindOrRegisterOIDCUser(email, name string) (*domain.User, error) {
	user, err := s.userService.ByEmailWithHousehold(email)
	if err == nil {
		return user, nil
	}

	if !errors.Is(err, domain.ErrRecordNotFound) {
		return nil, err
	}

	newUser := domain.User{
		ID:      uuid.New(),
		Email:   email,
		Name:    name,
		Created: time.Now(),
	}
	if newUser.Name == "" {
		newUser.Name = strings.Split(email, "@")[0]
	}

	if err := s.userService.Create(&newUser); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return s.userService.ByEmailWithHousehold(email)
		}
		return nil, err
	}
	return &newUser, nil
}
