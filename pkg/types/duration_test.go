package types

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		expected string
	}{
		{
			name:     "20 minutes",
			duration: Duration(20 * time.Minute),
			expected: "1200",
		},
		{
			name:     "15 minutes",
			duration: Duration(15 * time.Minute),
			expected: "900",
		},
		{
			name:     "35 minutes",
			duration: Duration(35 * time.Minute),
			expected: "2100",
		},
		{
			name:     "1 hour 30 minutes",
			duration: Duration(90 * time.Minute),
			expected: "5400",
		},
		{
			name:     "zero duration",
			duration: Duration(0),
			expected: "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := json.Marshal(tt.duration)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(result))
			}
		})
	}
}

func TestDuration_UnmarshalJSON_Integer(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Duration
	}{
		{
			name:     "20 minutes in seconds",
			input:    "1200",
			expected: Duration(20 * time.Minute),
		},
		{
			name:     "15 minutes in seconds",
			input:    "900",
			expected: Duration(15 * time.Minute),
		},
		{
			name:     "1 hour 30 minutes in seconds",
			input:    "5400",
			expected: Duration(90 * time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Duration
			err := json.Unmarshal([]byte(tt.input), &result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDuration_UnmarshalJSON_ISO8601(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Duration
	}{
		{
			name:     "PT20M (20 minutes)",
			input:    `"PT20M"`,
			expected: Duration(20 * time.Minute),
		},
		{
			name:     "PT15M (15 minutes)",
			input:    `"PT15M"`,
			expected: Duration(15 * time.Minute),
		},
		{
			name:     "PT1H30M (1 hour 30 minutes)",
			input:    `"PT1H30M"`,
			expected: Duration(90 * time.Minute),
		},
		{
			name:     "PT35M (35 minutes)",
			input:    `"PT35M"`,
			expected: Duration(35 * time.Minute),
		},
		{
			name:     "PT2H (2 hours)",
			input:    `"PT2H"`,
			expected: Duration(2 * time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Duration
			err := json.Unmarshal([]byte(tt.input), &result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v (%d seconds), got %v (%d seconds)",
					tt.expected, int64(tt.expected.Seconds()),
					result, int64(result.Seconds()))
			}
		})
	}
}

func TestDuration_UnmarshalJSON_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid ISO 8601",
			input: `"INVALID"`,
		},
		{
			name:  "empty string",
			input: `""`,
		},
		{
			name:  "boolean",
			input: `true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Duration
			err := json.Unmarshal([]byte(tt.input), &result)
			if err == nil {
				t.Errorf("expected error for input %s, got nil", tt.input)
			}
		})
	}
}

func TestFromISO8601(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Duration
		wantErr  bool
	}{
		{
			name:     "PT20M",
			input:    "PT20M",
			expected: Duration(20 * time.Minute),
			wantErr:  false,
		},
		{
			name:     "PT1H30M",
			input:    "PT1H30M",
			expected: Duration(90 * time.Minute),
			wantErr:  false,
		},
		{
			name:    "invalid",
			input:   "INVALID",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DurationFromISO8601(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromISO8601() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDuration_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "integer seconds",
			input:    "1200",
			expected: "1200",
		},
		{
			name:     "ISO 8601 PT20M",
			input:    `"PT20M"`,
			expected: "1200", // Always marshals to seconds
		},
		{
			name:     "ISO 8601 PT1H30M",
			input:    `"PT1H30M"`,
			expected: "5400", // Always marshals to seconds
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Unmarshal
			var ds Duration
			err := json.Unmarshal([]byte(tt.input), &ds)
			if err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			// Marshal
			result, err := json.Marshal(ds)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(result))
			}
		})
	}
}
