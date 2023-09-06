package errors

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

type Error struct {
	Status  int                `json:"status"`
	Code    string             `json:"code"`
	Message string             `json:"message"`
	Fields  *map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

func BadRequest(m string) *Error {
	return &Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: m}
}

func BadRequestVal(err error) *Error {
	// Define fields map.
	fields := map[string]string{}

	// Make error message for each invalid field.
	for _, err := range err.(validator.ValidationErrors) {
		fields[err.Field()] = fmt.Sprintf("Field '%s' validation failed: %v", err.Tag(), err.Error())
	}

	return &Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: err.Error(), Fields: &fields}
}

func BadRequestField(field string, reason string) *Error {
	// Define fields map.
	fields := map[string]string{}

	// Add field and reason to fail.
	fields[field] = fmt.Sprintf("Field '%s' validation failed: %v", field, reason)

	return &Error{Status: fiber.StatusBadRequest, Code: "bad-request", Message: "Failed to validate request body", Fields: &fields}
}

func Unauthorized(m string) *Error {
	return &Error{Status: fiber.StatusUnauthorized, Code: "unauthorized", Message: m}
}

func Forbidden(m string) *Error {
	return &Error{Status: fiber.StatusForbidden, Code: "forbidden", Message: m}
}

func NotFound(m string) *Error {
	return &Error{Status: fiber.StatusNotFound, Code: "not-found", Message: m}
}
