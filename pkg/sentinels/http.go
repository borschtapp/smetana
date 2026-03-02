package sentinels

import (
	"errors"
	"fmt"

	"borscht.app/smetana/domain"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

func BadRequest(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: m}
}

func BadRequestVal(err error) *domain.Error {
	// Define fields map.
	fields := map[string]string{}

	// Make error message for each invalid field.
	for _, err := range err.(validator.ValidationErrors) {
		fields[err.Field()] = fmt.Sprintf("Field '%s' validation failed: %v", err.Tag(), err.Error())
	}

	return &domain.Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: err.Error(), Fields: &fields}
}

func BadRequestField(field string, reason string) *domain.Error {
	// Define fields map.
	fields := map[string]string{}

	// Add field and reason to fail.
	fields[field] = fmt.Sprintf("Field '%s' validation failed: %v", field, reason)

	return &domain.Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: "Failed to validate request body", Fields: &fields}
}

func Unauthorized(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusUnauthorized, Code: "unauthorized", Message: m}
}

func Forbidden(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusForbidden, Code: "forbidden", Message: m}
}

func NotFound(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusNotFound, Code: "not-found", Message: m}
}

func IsNotFound(err error) bool {
	var e *domain.Error
	if errors.As(err, &e) {
		return e.Status == fiber.StatusNotFound
	}
	return false
}

func NotImplemented(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusNotImplemented, Code: "not-implemented", Message: m}
}

func InternalServerError(m string) *domain.Error {
	return &domain.Error{Status: fiber.StatusInternalServerError, Code: "internal-server-error", Message: m}
}
