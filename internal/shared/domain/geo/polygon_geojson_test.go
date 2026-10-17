package geo_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

// square returns a closed unit-ish square ring (Lon-first).
func square() []geo.Point {
	return []geo.Point{
		{Lon: 0, Lat: 0},
		{Lon: 1, Lat: 0},
		{Lon: 1, Lat: 1},
		{Lon: 0, Lat: 0},
	}
}

func TestPolygon_MarshalJSON_Golden(t *testing.T) {
	p, err := geo.NewPolygon(square())
	if err != nil {
		t.Fatalf("NewPolygon unexpected error: %v", err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal unexpected error: %v", err)
	}
	want := `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]]]}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}
}

func TestPolygon_MarshalJSON_WithHole(t *testing.T) {
	outer := []geo.Point{{Lon: 0, Lat: 0}, {Lon: 4, Lat: 0}, {Lon: 4, Lat: 4}, {Lon: 0, Lat: 0}}
	hole := []geo.Point{{Lon: 1, Lat: 1}, {Lon: 2, Lat: 1}, {Lon: 2, Lat: 2}, {Lon: 1, Lat: 1}}
	p, err := geo.NewPolygon(outer, hole)
	if err != nil {
		t.Fatalf("NewPolygon unexpected error: %v", err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal unexpected error: %v", err)
	}
	want := `{"type":"Polygon","coordinates":[[[0,0],[4,0],[4,4],[0,0]],[[1,1],[2,1],[2,2],[1,1]]]}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}
}

func TestPolygon_UnmarshalJSON_Valid(t *testing.T) {
	raw := `{"type":"Polygon","coordinates":[[[0,0],[4,0],[4,4],[0,0]],[[1,1],[2,1],[2,2],[1,1]]]}`
	var p geo.Polygon
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("Unmarshal unexpected error: %v", err)
	}
	if len(p.Exterior) != 4 {
		t.Fatalf("Exterior has %d points, want 4", len(p.Exterior))
	}
	if len(p.Holes) != 1 || len(p.Holes[0]) != 4 {
		t.Fatalf("Holes = %v, want 1 hole of 4 points", p.Holes)
	}
	if p.Exterior[1] != (geo.Point{Lon: 4, Lat: 0}) {
		t.Fatalf("Exterior[1] = %v, want {4 0}", p.Exterior[1])
	}
}

func TestPolygon_UnmarshalJSON_Invalid(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"wrong type", `{"type":"Point","coordinates":[0,0]}`, geo.ErrInvalidGeoJSON},
		{"no rings", `{"type":"Polygon","coordinates":[]}`, geo.ErrInvalidGeoJSON},
		{"unclosed ring", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[2,2]]]}`, geo.ErrPolygonNotClosed},
		{"too few points", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`, geo.ErrPolygonTooFewPoints},
		{"lon out of range", `{"type":"Polygon","coordinates":[[[0,0],[500,0],[1,1],[0,0]]]}`, geo.ErrInvalidLongitude},
		{"lat out of range", `{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,95],[0,0]]]}`, geo.ErrInvalidLatitude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p geo.Polygon
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

func TestPolygon_UnmarshalJSON_SyntaxError(t *testing.T) {
	// As with Point: syntactically-invalid JSON fails the encoding/json scan
	// before UnmarshalJSON runs, so the error is *json.SyntaxError, not
	// ErrInvalidGeoJSON.
	var p geo.Polygon
	err := json.Unmarshal([]byte(`not json`), &p)
	if err == nil {
		t.Fatalf("Unmarshal expected error, got nil")
	}
	var synErr *json.SyntaxError
	if !errors.As(err, &synErr) {
		t.Fatalf("Unmarshal error = %T (%v), want *json.SyntaxError", err, err)
	}
}

func TestPolygon_GeoJSON_RoundTrip(t *testing.T) {
	outer := []geo.Point{{Lon: -73.9, Lat: 40.7}, {Lon: -73.8, Lat: 40.7}, {Lon: -73.8, Lat: 40.8}, {Lon: -73.9, Lat: 40.7}}
	hole := []geo.Point{{Lon: -73.88, Lat: 40.72}, {Lon: -73.86, Lat: 40.72}, {Lon: -73.86, Lat: 40.74}, {Lon: -73.88, Lat: 40.72}}
	p, err := geo.NewPolygon(outer, hole)
	if err != nil {
		t.Fatalf("NewPolygon unexpected error: %v", err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal unexpected error: %v", err)
	}
	var back geo.Polygon
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("Unmarshal unexpected error: %v", err)
	}
	b2, err := json.Marshal(back)
	if err != nil {
		t.Fatalf("re-Marshal unexpected error: %v", err)
	}
	if string(b2) != string(b) {
		t.Fatalf("re-Marshal = %s, want %s", b2, b)
	}
	if len(back.Exterior) != len(p.Exterior) || len(back.Holes) != len(p.Holes) {
		t.Fatalf("round trip changed ring structure: %+v vs %+v", back, p)
	}
}
