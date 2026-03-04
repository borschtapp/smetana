package domain

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type AuthService interface {
	LoginURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, *oidc.IDToken, error)
	FindOrRegisterOIDCUser(email, name string) (*User, error)
}
