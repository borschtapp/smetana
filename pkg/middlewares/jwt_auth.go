package middlewares

import (
	"os"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"

	"borscht.app/smetana/pkg/errors"
)

// Protected func for specify routes group with JWT authentication.
// See: https://github.com/gofiber/jwt
func Protected() fiber.Handler {
	secretKey := os.Getenv("JWT_SECRET_KEY")

	// Create config for JWT authentication middlewares.
	return jwtware.New(jwtware.Config{
		SigningKey:   []byte(secretKey),
		ContextKey:   "jwt", // used in private routes
		ErrorHandler: jwtError,
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		return c.Status(fiber.StatusBadRequest).JSON(errors.BadRequest("Authorization header missing"))
	}

	return c.Status(fiber.StatusUnauthorized).JSON(errors.Unauthorized(err.Error()))
}
