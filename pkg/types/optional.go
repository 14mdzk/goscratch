package types

import "encoding/json"

// Opt represents an optional value that tracks whether it was explicitly set.
// Useful for distinguishing between "not provided" and "provided with value" in DTOs.
type Opt[T any] struct {
	Set bool
	Val T
}

// UnmarshalJSON implements json.Unmarshaler
func (o *Opt[T]) UnmarshalJSON(b []byte) error {
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	o.Set = true
	o.Val = v
	return nil
}

// MarshalJSON implements json.Marshaler
func (o Opt[T]) MarshalJSON() ([]byte, error) {
	if !o.Set {
		return []byte("null"), nil
	}
	return json.Marshal(o.Val)
}

// Some creates an Opt with a value set
func Some[T any](val T) Opt[T] {
	return Opt[T]{Set: true, Val: val}
}

// None creates an unset Opt
func None[T any]() Opt[T] {
	return Opt[T]{}
}

// Get returns the value and whether it was set
func (o Opt[T]) Get() (T, bool) {
	return o.Val, o.Set
}

// GetOr returns the value if set, otherwise returns the default
func (o Opt[T]) GetOr(def T) T {
	if o.Set {
		return o.Val
	}
	return def
}

// NOpt represents a nullable optional value that tracks:
// - Present: whether the field was included in the JSON
// - Valid: whether the value is non-null
// - Val: the actual value
// This is useful for PATCH operations where you need to distinguish between:
// - Field not sent (don't update)
// - Field sent as null (set to NULL)
// - Field sent with value (update to value)
type NOpt[T any] struct {
	Present bool
	Valid   bool
	Val     T
}

// UnmarshalJSON implements json.Unmarshaler
func (o *NOpt[T]) UnmarshalJSON(b []byte) error {
	o.Present = true
	if string(b) == "null" {
		o.Valid = false
		var zero T
		o.Val = zero
		return nil
	}

	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	o.Valid = true
	o.Val = v
	return nil
}

// MarshalJSON implements json.Marshaler
func (o NOpt[T]) MarshalJSON() ([]byte, error) {
	if !o.Present || !o.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(o.Val)
}

// Null creates a NOpt that represents an explicit null
func Null[T any]() NOpt[T] {
	return NOpt[T]{Present: true, Valid: false}
}

// NSome creates a NOpt with a valid value
func NSome[T any](v T) NOpt[T] {
	return NOpt[T]{Present: true, Valid: true, Val: v}
}

// NNone creates an unset NOpt (field not present)
func NNone[T any]() NOpt[T] {
	return NOpt[T]{}
}

// Get returns the value and whether it's valid
func (o NOpt[T]) Get() (T, bool) {
	return o.Val, o.Present && o.Valid
}

// GetOr returns the value if present and valid, otherwise returns the default
func (o NOpt[T]) GetOr(def T) T {
	if o.Present && o.Valid {
		return o.Val
	}
	return def
}
