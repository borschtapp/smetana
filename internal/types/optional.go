package types

import (
	"bytes"
	"encoding/json"
)

// Optional tells an absent JSON key (Set false) from an explicit null (Set true, Value nil).
type Optional[T any] struct {
	Set   bool
	Value *T
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		o.Value = nil
		return nil
	}

	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// ValidatorValue implements validator.Valuer, so validate tags apply to the inner value.
// It returns a typed pointer because `omitnil` only skips a typed nil.
func (o Optional[T]) ValidatorValue() any {
	return o.Value
}
