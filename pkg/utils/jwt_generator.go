package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Tokens struct to describe tokens object.
type Tokens struct {
	Type    string `json:"token_type"`
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token,omitempty"`
}

// GenerateNewTokens func for generate a new Access & Refresh tokens.
func GenerateNewTokens(id uint) (*Tokens, error) {
	// Generate JWT Access token.
	accessToken, err := generateNewAccessToken(id)
	if err != nil {
		// Return token generation error.
		return nil, err
	}

	// Generate JWT Refresh token.
	refreshToken, err := generateNewRefreshToken()
	if err != nil {
		// Return token generation error.
		return nil, err
	}

	return &Tokens{
		Type:    "Bearer",
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

func generateNewAccessToken(id uint) (string, error) {
	// Set secret key from .env file.
	secret := os.Getenv("JWT_SECRET_KEY")
	// Set expires in for secret key from .env file
	expiresIn := time.Minute * time.Duration(GetenvInt("JWT_SECRET_EXPIRE_MINUTES", 60))

	// Create a claims
	claims := jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(expiresIn).Unix(),
	}

	// Create a new JWT access token with claims.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate token.
	t, err := token.SignedString([]byte(secret))
	if err != nil {
		// Return error, it JWT token generation failed.
		return "", err
	}

	return t, nil
}

func generateNewRefreshToken() (string, error) {
	// Create a new SHA256 hash.
	hash := sha256.New()

	// Create a new now date and time string with salt.
	refresh := os.Getenv("JWT_REFRESH_KEY") + time.Now().String()

	// See: https://pkg.go.dev/io#Writer.Write
	_, err := hash.Write([]byte(refresh))
	if err != nil {
		// Return error, it refresh token generation failed.
		return "", err
	}

	// Create a new refresh token (sha256 string with salt + expire time).
	t := hex.EncodeToString(hash.Sum(nil))
	return t, nil
}
