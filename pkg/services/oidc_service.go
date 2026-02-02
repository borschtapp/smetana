package services

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"borscht.app/smetana/pkg/utils"
)

type OIDCService struct {
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

func NewOIDCService() (*OIDCService, error) {
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

	return &OIDCService{
		provider:     provider,
		oauth2Config: conf,
		verifier:     verifier,
	}, nil
}

func (s *OIDCService) GetLoginURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state)
}

func (s *OIDCService) Exchange(ctx context.Context, code string) (*oauth2.Token, *oidc.IDToken, error) {
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
