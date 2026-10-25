package types_test

import (
	"encoding/json"
	"testing"

	"github.com/go-playground/validator/v10"
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

func TestOptional_Validate_UsesInnerValue(t *testing.T) {
	validate := validator.New(validator.WithRequiredStructEnabled())
	type form struct {
		Amount types.Optional[float64] `validate:"omitnil,gt=0"`
		Note   types.Optional[string]  `validate:"omitnil,max=3"`
	}

	tests := []struct {
		name    string
		form    form
		wantErr bool
	}{
		{name: "absent", form: form{}},
		{name: "null", form: form{Amount: types.Optional[float64]{Set: true}}},
		{name: "valid", form: form{Amount: types.Optional[float64]{Set: true, Value: new(2.0)}, Note: types.Optional[string]{Set: true, Value: new("ok")}}},
		{name: "invalid number", form: form{Amount: types.Optional[float64]{Set: true, Value: new(-1.0)}}, wantErr: true},
		{name: "invalid string", form: form{Note: types.Optional[string]{Set: true, Value: new("too long")}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.form)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
