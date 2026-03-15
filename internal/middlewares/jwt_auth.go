package middlewares

import (
	"errors"

	jwtware "github.com/gofiber/contrib/v3/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/internal/configs"
	"borscht.app/smetana/internal/sentinels"
)

// Protected func for specify routes group with JWT authentication.
// See: https://github.com/gofiber/jwt
func Protected() fiber.Handler {
	secretKey := configs.JwtSecretKey()
	if len(secretKey) == 0 {
		log.Fatalw("JWT_SECRET_KEY environment variable is not set")
	}

	// Create config for JWT authentication middlewares.
	return jwtware.New(jwtware.Config{
		SigningKey:   jwtware.SigningKey{Key: secretKey},
		Extractor:    extractors.FromAuthHeader("Bearer"),
		ErrorHandler: jwtError,
	})
}

func jwtError(c fiber.Ctx, err error) error {
	if errors.Is(err, jwtware.ErrMissingToken) {
		return c.Status(fiber.StatusBadRequest).JSON(sentinels.BadRequest(err.Error()))
	}

	return c.Status(fiber.StatusUnauthorized).JSON(sentinels.Unauthorized("Invalid or expired JWT"))
}
