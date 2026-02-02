package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsPublicURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"https://google.com", true},
		{"http://127.0.0.1", false},
		{"http://localhost", false},
		{"http://10.0.0.1", false},
		{"http://192.168.1.1", false},
		{"http://172.16.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsPublicURL(tt.url))
		})
	}
}
