package geo_test

import (
	"errors"
	"math"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

func TestNewDistance_Valid(t *testing.T) {
	cases := []struct {
		name   string
		meters float64
	}{
		{"zero", 0},
		{"positive", 1000},
		{"fractional", 1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := geo.NewDistance(tc.meters)
			if err != nil {
				t.Fatalf("NewDistance(%v) unexpected error: %v", tc.meters, err)
			}
			if d.Meters != tc.meters {
				t.Fatalf("NewDistance(%v).Meters = %v", tc.meters, d.Meters)
			}
		})
	}
}

func TestNewDistance_Negative(t *testing.T) {
	_, err := geo.NewDistance(-1)
	if !errors.Is(err, geo.ErrNegativeDistance) {
		t.Fatalf("NewDistance(-1) error = %v, want ErrNegativeDistance", err)
	}
}

func TestDistance_Constants(t *testing.T) {
	if geo.Meter.Meters != 1 {
		t.Errorf("Meter = %v, want 1", geo.Meter.Meters)
	}
	if geo.Kilometer.Meters != 1000 {
		t.Errorf("Kilometer = %v, want 1000", geo.Kilometer.Meters)
	}
	if geo.Mile.Meters != 1609.344 {
		t.Errorf("Mile = %v, want 1609.344", geo.Mile.Meters)
	}
	if geo.NauticalMile.Meters != 1852 {
		t.Errorf("NauticalMile = %v, want 1852", geo.NauticalMile.Meters)
	}
}

func TestDistance_Conversions(t *testing.T) {
	const epsilon = 1e-9
	d := geo.Distance{Meters: 5000}

	if math.Abs(d.Kilometers()-5) > epsilon {
		t.Errorf("Kilometers() = %v, want 5", d.Kilometers())
	}
	wantMiles := 5000 / 1609.344
	if math.Abs(d.Miles()-wantMiles) > epsilon {
		t.Errorf("Miles() = %v, want %v", d.Miles(), wantMiles)
	}
	wantNM := 5000.0 / 1852
	if math.Abs(d.NauticalMiles()-wantNM) > epsilon {
		t.Errorf("NauticalMiles() = %v, want %v", d.NauticalMiles(), wantNM)
	}
}

func TestDistance_Add(t *testing.T) {
	a := geo.Distance{Meters: 1000}
	b := geo.Distance{Meters: 500}
	got := a.Add(b)
	if got.Meters != 1500 {
		t.Fatalf("Add() = %v, want 1500", got.Meters)
	}
}

func TestDistance_Sub(t *testing.T) {
	cases := []struct {
		name string
		a    float64
		b    float64
		want float64
	}{
		{"normal", 1000, 300, 700},
		{"exact zero", 500, 500, 0},
		{"underflow clamped", 100, 500, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := geo.Distance{Meters: tc.a}
			b := geo.Distance{Meters: tc.b}
			got := a.Sub(b)
			if got.Meters != tc.want {
				t.Fatalf("Sub() = %v, want %v", got.Meters, tc.want)
			}
		})
	}
}
