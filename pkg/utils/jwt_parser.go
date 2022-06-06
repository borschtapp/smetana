package utils

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// TokenMetadata struct to describe metadata in JWT.
type TokenMetadata struct {
	ID      uint
	Expires int64
}

// ExtractTokenMetadata func to extract metadata from JWT.
func ExtractTokenMetadata(c *fiber.Ctx) (*TokenMetadata, error) {
	token := c.Locals("jwt").(*jwt.Token)

	// Setting and checking token and credentials.
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		// Expires time.
		id := uint(claims["id"].(float64))
		expires := int64(claims["exp"].(float64))

		return &TokenMetadata{
			ID:      id,
			Expires: expires,
		}, nil
	}

	return nil, errors.New("token invalid")
}
