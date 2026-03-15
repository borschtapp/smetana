package domain

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type AuthTokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token,omitempty"`
}

type AuthService interface {
	Login(email, password string) (*User, error)
	Register(name, email, password string) (*User, error)
	IssueTokens(user User) (*AuthTokens, error)
	RotateRefreshToken(tokenStr string) (*User, *AuthTokens, error)
	Logout(tokenStr string) error
}

type OIDCService interface {
	LoginURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, *oidc.IDToken, error)
	Authorize(email, name string) (*User, error)
}
