package geo_test

import (
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

func TestNewPoint_ValidCoordinates(t *testing.T) {
	cases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{"zero", 0, 0},
		{"positive", 120.5, 35.7},
		{"negative", -74.006, -33.865},
		{"max lon", 180, 0},
		{"min lon", -180, 0},
		{"max lat", 0, 90},
		{"min lat", 0, -90},
		{"both max", 180, 90},
		{"both min", -180, -90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := geo.NewPoint(tc.lon, tc.lat)
			if err != nil {
				t.Fatalf("NewPoint(%v, %v) unexpected error: %v", tc.lon, tc.lat, err)
			}
			if p.Lon != tc.lon || p.Lat != tc.lat {
				t.Fatalf("NewPoint(%v, %v) = {%v, %v}, want {%v, %v}",
					tc.lon, tc.lat, p.Lon, p.Lat, tc.lon, tc.lat)
			}
		})
	}
}

func TestNewPoint_InvalidLongitude(t *testing.T) {
	cases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{"lon too high", 180.0000001, 0},
		{"lon too low", -180.0000001, 0},
		{"lon way off", 270, 45},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := geo.NewPoint(tc.lon, tc.lat)
			if !errors.Is(err, geo.ErrInvalidLongitude) {
				t.Fatalf("NewPoint(%v, %v) error = %v, want ErrInvalidLongitude", tc.lon, tc.lat, err)
			}
		})
	}
}

func TestNewPoint_InvalidLatitude(t *testing.T) {
	cases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{"lat too high", 0, 90.0000001},
		{"lat too low", 0, -90.0000001},
		{"lat way off", 0, 180},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := geo.NewPoint(tc.lon, tc.lat)
			if !errors.Is(err, geo.ErrInvalidLatitude) {
				t.Fatalf("NewPoint(%v, %v) error = %v, want ErrInvalidLatitude", tc.lon, tc.lat, err)
			}
		})
	}
}

func TestPoint_RoundsToZero(t *testing.T) {
	p, err := geo.NewPoint(0, 0)
	if err != nil {
		t.Fatalf("NewPoint(0, 0) unexpected error: %v", err)
	}
	if p.Lon != 0 || p.Lat != 0 {
		t.Fatalf("NewPoint(0, 0) = {%v, %v}, want {0, 0}", p.Lon, p.Lat)
	}
}

func TestPoint_Boundary(t *testing.T) {
	validCases := []struct {
		lon float64
		lat float64
	}{
		{180, 90},
		{-180, -90},
	}
	for _, tc := range validCases {
		_, err := geo.NewPoint(tc.lon, tc.lat)
		if err != nil {
			t.Errorf("NewPoint(%v, %v) unexpected error: %v", tc.lon, tc.lat, err)
		}
	}

	invalidCases := []struct {
		lon     float64
		lat     float64
		wantErr error
	}{
		{180.0000001, 0, geo.ErrInvalidLongitude},
		{0, 90.0000001, geo.ErrInvalidLatitude},
	}
	for _, tc := range invalidCases {
		_, err := geo.NewPoint(tc.lon, tc.lat)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("NewPoint(%v, %v) error = %v, want %v", tc.lon, tc.lat, err, tc.wantErr)
		}
	}
}

func TestMustPoint_Valid(t *testing.T) {
	p := geo.MustPoint(10, 20)
	if p.Lon != 10 || p.Lat != 20 {
		t.Fatalf("MustPoint(10, 20) = {%v, %v}, want {10, 20}", p.Lon, p.Lat)
	}
}

func TestMustPoint_PanicsOnInvalidLon(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustPoint with invalid lon should panic")
		}
	}()
	geo.MustPoint(200, 0)
}

func TestMustPoint_PanicsOnInvalidLat(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustPoint with invalid lat should panic")
		}
	}()
	geo.MustPoint(0, 100)
}

func TestPoint_Validate(t *testing.T) {
	cases := []struct {
		name    string
		p       geo.Point
		wantErr error
	}{
		{"valid", geo.Point{Lon: 10, Lat: 20}, nil},
		{"bad lon", geo.Point{Lon: 200, Lat: 0}, geo.ErrInvalidLongitude},
		{"bad lat", geo.Point{Lon: 0, Lat: 100}, geo.ErrInvalidLatitude},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.p.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Point{%v, %v}.Validate() = %v, want %v", tc.p.Lon, tc.p.Lat, err, tc.wantErr)
			}
		})
	}
}
