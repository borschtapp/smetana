package api

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/internal/sentinels"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// bindBody binds the request body to dst and validates it.
func bindBody[T any](c fiber.Ctx, dst *T) error {
	if err := c.Bind().Body(dst); err != nil {
		return sentinels.BadRequest(err.Error())
	}
	if err := validate.Struct(dst); err != nil {
		return sentinels.BadRequestVal(err)
	}
	return nil
}
