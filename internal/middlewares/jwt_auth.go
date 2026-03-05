package middlewares

import (
	"os"

	jwtware "github.com/gofiber/contrib/v3/jwt"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"

	"borscht.app/smetana/internal/sentinels"
)

// Protected func for specify routes group with JWT authentication.
// See: https://github.com/gofiber/jwt
func Protected() fiber.Handler {
	secretKey := os.Getenv("JWT_SECRET_KEY")
	if secretKey == "" {
		log.Fatal("JWT_SECRET_KEY environment variable is not set")
	}

	// Create config for JWT authentication middlewares.
	return jwtware.New(jwtware.Config{
		SigningKey:   jwtware.SigningKey{Key: []byte(secretKey)},
		ErrorHandler: jwtError,
	})
}

func jwtError(c fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		return c.Status(fiber.StatusBadRequest).JSON(sentinels.BadRequest("Authorization header missing"))
	}

	return c.Status(fiber.StatusUnauthorized).JSON(sentinels.Unauthorized(err.Error()))
}
