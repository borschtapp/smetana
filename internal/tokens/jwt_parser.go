package tokens

import (
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
		return nil, sentinels.Unauthorized("Missing or malformed JWT")
	}

	// Setting and checking token and credentials.
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		expires := int64(claims["exp"].(float64))

		userID, err := uuid.Parse(claims["id"].(string))
		if err != nil {
			return nil, err
		}

		householdID, err := uuid.Parse(claims["hid"].(string))
		if err != nil {
			return nil, err
		}

		return &TokenMetadata{
			ID:          userID,
			HouseholdID: householdID,
			Expires:     expires,
		}, nil
	}

	return nil, sentinels.Unauthorized("Token expired or invalid")
}
