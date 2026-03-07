package sentinels

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
)

func BadRequest(m string) *Error {
	return &Error{Status: fiber.StatusBadRequest, Message: m}
}

func BadRequestVal(err error) *Error {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		return &Error{Status: fiber.StatusBadRequest, Message: err.Error()}
	}

	fields := map[string]string{}
	for _, e := range validationErrors {
		fields[e.Field()] = fmt.Sprintf("Field '%s' validation failed: %v", e.Tag(), e.Error())
	}

	return &Error{Status: fiber.StatusBadRequest, Message: "Request validation failed", Fields: &fields}
}

func BadRequestField(field string, reason string) *Error {
	// Define fields map.
	fields := map[string]string{}

	// Add field and reason to fail.
	fields[field] = fmt.Sprintf("Field '%s' validation failed: %v", field, reason)

	return &Error{Status: fiber.StatusBadRequest, Message: "Failed to validate request body", Fields: &fields}
}

func Unauthorized(m string) *Error {
	return &Error{Status: fiber.StatusUnauthorized, Message: m}
}

func NotImplemented(m string) *Error {
	return &Error{Status: fiber.StatusNotImplemented, Message: m}
}

func InternalServerError(m string) *Error {
	return &Error{Status: fiber.StatusInternalServerError, Message: m}
}
