package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJSON_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
		wantErr  bool
	}{
		{
			name:     "[]byte value",
			input:    []byte(`{"a":1}`),
			expected: `{"a":1}`,
		},
		{
			name:     "string value",
			input:    `{"a":1}`,
			expected: `{"a":1}`,
		},
		{
			name:  "nil value",
			input: nil,
		},
		{
			name:    "invalid type",
			input:   42,
			wantErr: true,
		},
		{
			name:    "invalid json",
			input:   []byte(`not json`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var j JSON
			err := j.Scan(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, string(j))
		})
	}
}
