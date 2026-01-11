package types

import (
	"reflect"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// RegisterFiberDecoders registers custom decoders for Opt types with Fiber's parser.
// Call this in your application's init or main function before creating Fiber app.
func RegisterFiberDecoders() {
	fiber.SetParserDecoder(fiber.ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType: []fiber.ParserType{
			{
				Customtype: Opt[string]{},
				Converter:  optStringConverter,
			},
			{
				Customtype: Opt[int]{},
				Converter:  optIntConverter,
			},
			{
				Customtype: Opt[bool]{},
				Converter:  optBoolConverter,
			},
			{
				Customtype: NOpt[string]{},
				Converter:  noptStringConverter,
			},
			{
				Customtype: NOpt[int]{},
				Converter:  noptIntConverter,
			},
			{
				Customtype: NOpt[bool]{},
				Converter:  noptBoolConverter,
			},
		},
	})
}

// Opt[string] converter
func optStringConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(None[string]())
	}
	return reflect.ValueOf(Some(value))
}

// Opt[int] converter
func optIntConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(None[int]())
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return reflect.ValueOf(None[int]())
	}
	return reflect.ValueOf(Some(i))
}

// Opt[bool] converter - handles "true", "false", "1", "0"
func optBoolConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(None[bool]())
	}
	switch value {
	case "true", "1", "yes", "on":
		return reflect.ValueOf(Some(true))
	case "false", "0", "no", "off":
		return reflect.ValueOf(Some(false))
	default:
		return reflect.ValueOf(None[bool]())
	}
}

// NOpt[string] converter
func noptStringConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(NNone[string]())
	}
	if value == "null" {
		return reflect.ValueOf(Null[string]())
	}
	return reflect.ValueOf(NSome(value))
}

// NOpt[int] converter
func noptIntConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(NNone[int]())
	}
	if value == "null" {
		return reflect.ValueOf(Null[int]())
	}
	i, err := strconv.Atoi(value)
	if err != nil {
		return reflect.ValueOf(NNone[int]())
	}
	return reflect.ValueOf(NSome(i))
}

// NOpt[bool] converter
func noptBoolConverter(value string) reflect.Value {
	if value == "" {
		return reflect.ValueOf(NNone[bool]())
	}
	if value == "null" {
		return reflect.ValueOf(Null[bool]())
	}
	switch value {
	case "true", "1", "yes", "on":
		return reflect.ValueOf(NSome(true))
	case "false", "0", "no", "off":
		return reflect.ValueOf(NSome(false))
	default:
		return reflect.ValueOf(NNone[bool]())
	}
}
