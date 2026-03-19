package tokens

import (
	"errors"
	"time"

	"borscht.app/smetana/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/utils"
)

func GenerateAccess(id uuid.UUID, householdID uuid.UUID) (string, error) {
	return generateNewAccessToken(id, householdID)
}

// GenerateNew func for generate a new Access & Refresh tokens.
func GenerateNew(id uuid.UUID, householdId uuid.UUID) (*domain.AuthTokens, error) {
	accessToken, err := generateNewAccessToken(id, householdId)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateNewRefreshToken()
	if err != nil {
		return nil, err
	}

	return &domain.AuthTokens{
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}

func generateNewAccessToken(id uuid.UUID, householdId uuid.UUID) (string, error) {
	expiresIn := time.Minute * time.Duration(configs.JwtSecretExpireMinutes())
	claims := jwt.MapClaims{
		"id":  id.String(),
		"hid": householdId.String(),
		"exp": time.Now().Add(expiresIn).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString(configs.JwtSecretKey())
	if err != nil {
		// Return error, if JWT token generation failed.
		return "", err
	}

	return t, nil
}

func generateNewRefreshToken() (string, error) {
	token := utils.GenerateRandomString(32)
	if token == "" {
		return "", errors.New("failed to generate refresh token")
	}
	return token, nil
}
