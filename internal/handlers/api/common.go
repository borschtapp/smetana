package api

import (
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"borscht.app/smetana/internal/sentinels"
	"borscht.app/smetana/internal/types"
)

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Validate Optional fields by their inner value.
	v.RegisterCustomTypeFunc(func(field reflect.Value) any {
		return field.Interface().(interface{ ValidationValue() any }).ValidationValue()
	}, types.Optional[float64]{}, types.Optional[string]{}, types.Optional[uuid.UUID]{})
	return v
}

type mergeRequest struct {
	MergeInto uuid.UUID `json:"merge_into" validate:"required"`
}

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
