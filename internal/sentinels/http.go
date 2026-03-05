package sentinels

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"

	"borscht.app/smetana/domain"
)

func BadRequest(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusBadRequest, Message: m}
}

func BadRequestVal(err error) *domain.Error {
	// Define fields map.
	fields := map[string]string{}

	// Make error message for each invalid field.
	for _, err := range err.(validator.ValidationErrors) {
		fields[err.Field()] = fmt.Sprintf("Field '%s' validation failed: %v", err.Tag(), err.Error())
	}

	return &domain.Error{Status: fiber.StatusBadRequest, Message: err.Error(), Fields: &fields}
}

func BadRequestField(field string, reason string) *domain.Error {
	// Define fields map.
	fields := map[string]string{}

	// Add field and reason to fail.
	fields[field] = fmt.Sprintf("Field '%s' validation failed: %v", field, reason)

	return &domain.Error{Status: fiber.StatusBadRequest, Message: "Failed to validate request body", Fields: &fields}
}

func Unauthorized(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusUnauthorized, Message: m}
}

func Forbidden(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusForbidden, Message: m}
}

func NotFound(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusNotFound, Message: m}
}

func NotImplemented(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusNotImplemented, Message: m}
}

func InternalServerError(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusInternalServerError, Message: m}
}
