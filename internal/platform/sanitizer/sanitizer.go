package sanitizer

import (
	"reflect"

	"github.com/microcosm-cc/bluemonday"
)

// policy is a shared strict sanitization policy that strips all HTML
var policy = bluemonday.StrictPolicy()

// Sanitize strips all HTML tags from the input string
func Sanitize(input string) string {
	return policy.Sanitize(input)
}

// SanitizeStruct recursively sanitizes all string fields in a struct.
// The argument must be a pointer to a struct.
func SanitizeStruct(v interface{}) {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	sanitizeValue(val)
}

// sanitizeValue recursively walks a reflect.Value and sanitizes string fields
func sanitizeValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.String:
		if v.CanSet() {
			v.SetString(Sanitize(v.String()))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanSet() {
				sanitizeValue(field)
			}
		}
	case reflect.Ptr:
		if !v.IsNil() {
			sanitizeValue(v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			sanitizeValue(v.Index(i))
		}
	}
}
