package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"

	"borscht.app/smetana/pkg/config"
)

// Tokens struct to describe tokens object.
type Tokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
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
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

func generateNewAccessToken(id uint) (string, error) {
	// Set secret key from .env file.
	secret := config.Env.JwtSecretKey

	// Set expires minutes count for secret key from .env file
	minutesCount := config.Env.JwtTokenExpireMinutes

	// Create a claims
	claims := jwt.MapClaims{
		"id":  id,
		"exp": time.Now().Add(time.Minute * time.Duration(minutesCount)).Unix(),
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
	refresh := config.Env.JwtRefreshKey + time.Now().String()

	// See: https://pkg.go.dev/io#Writer.Write
	_, err := hash.Write([]byte(refresh))
	if err != nil {
		// Return error, it refresh token generation failed.
		return "", err
	}

	// Set expires hours count for refresh key from .env file.
	minutesCount := config.Env.JwtRefreshExpireMinutes

	// Set expiration time.
	expireTime := fmt.Sprint(time.Now().Add(time.Minute * time.Duration(minutesCount)).Unix())

	// Create a new refresh token (sha256 string with salt + expire time).
	t := hex.EncodeToString(hash.Sum(nil)) + "." + expireTime

	return t, nil
}

// ParseRefreshToken func for parse second argument from refresh token.
func ParseRefreshToken(refreshToken string) (int64, error) {
	return strconv.ParseInt(strings.Split(refreshToken, ".")[1], 0, 64)
}
