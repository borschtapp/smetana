package presenter

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type Response struct {
	Status   bool              `json:"status"`
	Data     interface{}       `json:"data,omitempty"`
	Error    string            `json:"error,omitempty"`
	Messages map[string]string `json:"messages,omitempty"`
}

func OkResponse(data interface{}) Response {
	return Response{
		Status: true,
		Data:   data,
	}
}

func BadResponse(err string) Response {
	return Response{
		Status: false,
		Error:  err,
	}
}

func ErrorResponse(err error) Response {
	return Response{
		Status: false,
		Error:  err.Error(),
	}
}

func ValidatorResponse(err error) Response {
	// Define fields map.
	fields := map[string]string{}

	// Make error message for each invalid field.
	for _, err := range err.(validator.ValidationErrors) {
		fields[err.Field()] = fmt.Sprintf("Field validation failed on the '%s' tag", err.Tag())
	}

	return Response{
		Status:   false,
		Messages: fields,
	}
}
