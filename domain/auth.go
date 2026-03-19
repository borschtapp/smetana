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
	Register(name, email, password, inviteCode string) (*User, error)
	// ForgotPassword generates a reset token and sends it to the given email address.
	ForgotPassword(email string) error
	ResetPassword(rawToken, newPassword string) error
	IssueTokens(user User) (*AuthTokens, error)
	IssueAccessToken(user User) (string, error)
	RotateRefreshToken(tokenStr string) (*User, *AuthTokens, error)
	Logout(tokenStr string) error
}

type EmailService interface {
	SendPasswordReset(to, rawToken string) error
	SendHouseholdInvite(to, code string) error
}

type OIDCService interface {
	LoginURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, *oidc.IDToken, error)
	Authorize(email, name string) (*User, error)
}
