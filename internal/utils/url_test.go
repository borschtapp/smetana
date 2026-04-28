package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Scheme and Host
		{"HTTP://WWW.Example.COM:80/feed", "http://example.com/feed"},
		{"https://example.com:443/feed", "https://example.com/feed"},
		{"https://example.com:8443/feed", "https://example.com:8443/feed"},
		{"https://www.test.com/", "https://test.com"},

		// Path normalization
		{"https://example.com/a/b/../c", "https://example.com/a/c"},
		{"https://example.com/a/./b", "https://example.com/a/b"},
		{"https://example.com//path", "https://example.com/path"},
		{"https://example.com/path/%2f%20", "https://example.com/path/%20"},

		// Trailing slash
		{"https://example.com/", "https://example.com"},
		{"https://example.com/path/", "https://example.com/path"},

		// Unreserved Character Decoding
		{"https://example.com/%7Euser", "https://example.com/~user"},
		{"https://example.com/path/%2f%20%7e", "https://example.com/path/%20~"},

		// Fragment
		{"https://example.com/#section", "https://example.com"},
		{"https://example.com/path?a=1#frag", "https://example.com/path?a=1"},

		// Complex Example
		{"HttpS://WWW.Example.COM:443/foo/../bar//baz/%7euser/index.html?utm_source=twitter&b=2&a=1#section", "https://example.com/bar/baz/~user/index.html?a=1&b=2&utm_source=twitter"},

		// Invalid URL
		{"", ""},
		{"invalid-url", "invalid-url"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeURL(tt.input))
		})
	}
}
