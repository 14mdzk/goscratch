package geo_test

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

func TestPoint_MarshalJSON_Golden(t *testing.T) {
	p := geo.MustPoint(100.5, -0.25)
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal unexpected error: %v", err)
	}
	want := `{"type":"Point","coordinates":[100.5,-0.25]}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}
}

func TestPoint_UnmarshalJSON_Valid(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		lon  float64
		lat  float64
	}{
		{"basic", `{"type":"Point","coordinates":[100.5,-0.25]}`, 100.5, -0.25},
		{"zero", `{"type":"Point","coordinates":[0,0]}`, 0, 0},
		{"integer coords", `{"type":"Point","coordinates":[-74,40]}`, -74, 40},
		{"bounds", `{"type":"Point","coordinates":[180,90]}`, 180, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p geo.Point
			if err := json.Unmarshal([]byte(tc.raw), &p); err != nil {
				t.Fatalf("Unmarshal unexpected error: %v", err)
			}
			if p.Lon != tc.lon || p.Lat != tc.lat {
				t.Fatalf("Unmarshal = {%v, %v}, want {%v, %v}", p.Lon, p.Lat, tc.lon, tc.lat)
			}
		})
	}
}

func TestPoint_UnmarshalJSON_Invalid(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"wrong type", `{"type":"LineString","coordinates":[[0,0],[1,1]]}`, geo.ErrInvalidGeoJSON},
		{"missing type", `{"coordinates":[0,0]}`, geo.ErrInvalidGeoJSON},
		{"one coordinate", `{"type":"Point","coordinates":[1]}`, geo.ErrInvalidGeoJSON},
		{"three coordinates", `{"type":"Point","coordinates":[1,2,3]}`, geo.ErrInvalidGeoJSON},
		{"lon out of range", `{"type":"Point","coordinates":[200,0]}`, geo.ErrInvalidLongitude},
		{"lat out of range", `{"type":"Point","coordinates":[0,95]}`, geo.ErrInvalidLatitude},
		{"null", `null`, geo.ErrInvalidGeoJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p geo.Point
			err := json.Unmarshal([]byte(tc.raw), &p)
			if err == nil {
				t.Fatalf("Unmarshal(%s) expected error, got nil", tc.raw)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Unmarshal(%s) error = %v, want errors.Is %v", tc.raw, err, tc.wantErr)
			}
		})
	}
}

func TestPoint_UnmarshalJSON_SyntaxError(t *testing.T) {
	// Syntactically-invalid JSON never reaches UnmarshalJSON: encoding/json
	// fails the scan first and returns *json.SyntaxError unwrapped. The
	// ErrInvalidGeoJSON contract covers semantically-wrong GeoJSON only.
	var p geo.Point
	err := json.Unmarshal([]byte(`not json`), &p)
	if err == nil {
		t.Fatalf("Unmarshal expected error, got nil")
	}
	var synErr *json.SyntaxError
	if !errors.As(err, &synErr) {
		t.Fatalf("Unmarshal error = %T (%v), want *json.SyntaxError", err, err)
	}
	if errors.Is(err, geo.ErrInvalidGeoJSON) {
		t.Fatalf("Unmarshal error unexpectedly wraps ErrInvalidGeoJSON: %v", err)
	}
}

func TestPoint_GeoJSON_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{"origin", 0, 0},
		{"jakarta", 106.8456, -6.2088},
		{"antimeridian edge", -179.999, 89.999},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := geo.MustPoint(tc.lon, tc.lat)
			b, err := json.Marshal(p)
			if err != nil {
				t.Fatalf("Marshal unexpected error: %v", err)
			}
			var back geo.Point
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatalf("Unmarshal unexpected error: %v", err)
			}
			if back != p {
				t.Fatalf("round trip = {%v, %v}, want {%v, %v}", back.Lon, back.Lat, p.Lon, p.Lat)
			}
			// Marshal stability: re-marshal must be byte-identical.
			b2, err := json.Marshal(back)
			if err != nil {
				t.Fatalf("re-Marshal unexpected error: %v", err)
			}
			if string(b2) != string(b) {
				t.Fatalf("re-Marshal = %s, want %s", b2, b)
			}
		})
	}
}

func TestPoint_MarshalJSON_NonFinite(t *testing.T) {
	// encoding/json cannot represent NaN/Inf: Marshal must fail closed
	// rather than emit invalid JSON. Literals bypass NewPoint validation.
	cases := []struct {
		name string
		p    geo.Point
	}{
		{"NaN lon", geo.Point{Lon: math.NaN(), Lat: 0}},
		{"+Inf lat", geo.Point{Lon: 0, Lat: math.Inf(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := json.Marshal(tc.p); err == nil {
				t.Fatalf("Marshal(%v) expected error for non-finite coordinate, got nil", tc.p)
			}
		})
	}
}
