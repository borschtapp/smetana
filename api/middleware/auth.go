package middleware

import (
	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"

	"borscht.app/smetana/api/presenter"
	"borscht.app/smetana/config"
)

// Protected func for specify routes group with JWT authentication.
// See: https://github.com/gofiber/jwt
func Protected() fiber.Handler {
	// Create config for JWT authentication middleware.
	return jwtware.New(jwtware.Config{
		SigningKey:   []byte(config.Env.JwtSecretKey),
		ContextKey:   "jwt", // used in private routes
		ErrorHandler: jwtError,
	})
}

func jwtError(c *fiber.Ctx, err error) error {
	if err.Error() == "Missing or malformed JWT" {
		return c.Status(fiber.StatusForbidden).JSON(presenter.BadResponse("Authorization header missing"))
	}

	return c.Status(fiber.StatusUnauthorized).JSON(presenter.ErrorResponse(err))
}
