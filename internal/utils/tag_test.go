package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTag(t *testing.T) {
	tests := []struct {
		args string
		want string
	}{
		{args: "Hello world.", want: "hello world"},
		{args: "Привіт Світ!!", want: "привіт світ"},
	}
	for _, tt := range tests {
		t.Run(tt.args, func(t *testing.T) {
			got := CreateTag(tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}
