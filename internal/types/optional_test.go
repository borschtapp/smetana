package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"borscht.app/smetana/internal/types"
)

func TestOptional_UnmarshalJSON(t *testing.T) {
	type form struct {
		Amount types.Optional[float64] `json:"amount"`
	}

	tests := []struct {
		name      string
		body      string
		wantSet   bool
		wantValue *float64
	}{
		{name: "absent", body: `{}`, wantSet: false},
		{name: "null", body: `{"amount": null}`, wantSet: true},
		{name: "value", body: `{"amount": 2.5}`, wantSet: true, wantValue: new(2.5)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f form
			require.NoError(t, json.Unmarshal([]byte(tt.body), &f))
			assert.Equal(t, tt.wantSet, f.Amount.Set)
			assert.Equal(t, tt.wantValue, f.Amount.Value)
		})
	}
}

func TestOptional_UnmarshalJSON_WrongType(t *testing.T) {
	var f struct {
		Amount types.Optional[float64] `json:"amount"`
	}
	assert.Error(t, json.Unmarshal([]byte(`{"amount": "two"}`), &f))
}
