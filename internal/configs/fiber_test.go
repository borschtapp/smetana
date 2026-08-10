package configs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/internal/sentinels"
)

func TestFiberConfig_ErrorHandler_UnwrapsSentinelErrors(t *testing.T) {
	tests := []struct {
		name     string
		sentinel *sentinels.Error
	}{
		{name: "not found", sentinel: sentinels.ErrNotFound},
		{name: "conflict", sentinel: sentinels.ErrAlreadyExists},
		{name: "forbidden", sentinel: sentinels.ErrForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New(FiberConfig())
			app.Get("/", func(fiber.Ctx) error {
				return fmt.Errorf("repository operation failed: %w", tt.sentinel)
			})

			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, tt.sentinel.Status, resp.StatusCode)

			var body sentinels.Error
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
			assert.Equal(t, *tt.sentinel, body)
		})
	}
}

func TestFiberConfig_ErrorHandler_UnexpectedErrorIsGeneric500(t *testing.T) {
	app := fiber.New(FiberConfig())
	app.Get("/", func(fiber.Ctx) error { return fmt.Errorf("database connection lost") })

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var body sentinels.Error
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, fiber.StatusInternalServerError, body.Status)
	assert.Equal(t, "An internal error occurred", body.Message)
}
