package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapKeys(t *testing.T) {
	set := map[string]bool{"a": true, "b": true}
	keys := MapKeys(set)
	assert.ElementsMatch(t, []string{"a", "b"}, keys)
}
