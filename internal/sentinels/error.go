package sentinels

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

type Error struct {
	Status  int                `json:"status"`
	Message string             `json:"message"`
	Fields  *map[string]string `json:"fields,omitempty"`
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Status == t.Status
}

var ErrUnauthorized = &Error{Status: fiber.StatusUnauthorized, Message: "Invalid credentials"}
var ErrForbidden = &Error{Status: fiber.StatusForbidden, Message: "Access denied"}
var ErrNotFound = &Error{Status: fiber.StatusNotFound, Message: "The requested entity does not exist"}
var ErrAlreadyExists = &Error{Status: fiber.StatusConflict, Message: "The entity already exists"}
