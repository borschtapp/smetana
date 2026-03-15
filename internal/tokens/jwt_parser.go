package tokens

import (
	"errors"

	"borscht.app/smetana/internal/sentinels"
	jwtware "github.com/gofiber/contrib/v3/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/google/uuid"
)

// TokenMetadata struct to describe metadata in JWT.
type TokenMetadata struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	Expires     int64
}

// ParseJwtClaims func to extract metadata from JWT.
func ParseJwtClaims(c fiber.Ctx) (*TokenMetadata, error) {
	token := jwtware.FromContext(c)
	if token == nil {
		return nil, sentinels.Unauthorized("Missing or invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, sentinels.Unauthorized("Expired or invalid token")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		return nil, sentinels.Unauthorized("Invalid token")
	}

	userID, err := parseUUIDClaim(claims, "id")
	if err != nil {
		return nil, sentinels.Unauthorized("Invalid token")
	}

	householdID, err := parseUUIDClaim(claims, "hid")
	if err != nil {
		return nil, sentinels.Unauthorized("Invalid token")
	}

	return &TokenMetadata{
		ID:          userID,
		HouseholdID: householdID,
		Expires:     int64(exp),
	}, nil
}

func parseUUIDClaim(claims jwt.MapClaims, key string) (uuid.UUID, error) {
	val, ok := claims[key].(string)
	if !ok {
		return uuid.UUID{}, errors.New("Missing claim " + key)
	}
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.UUID{}, errors.New("Invalid UUID " + key)
	}
	return id, nil
}
