package geo_test

import (
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

func TestNewBoundingBox_Valid(t *testing.T) {
	min := geo.MustPoint(-10, -10)
	max := geo.MustPoint(10, 10)
	bb, err := geo.NewBoundingBox(min, max)
	if err != nil {
		t.Fatalf("NewBoundingBox unexpected error: %v", err)
	}
	if bb.Min != min || bb.Max != max {
		t.Fatalf("NewBoundingBox stored incorrect points")
	}
}

func TestNewBoundingBox_EqualMinMax(t *testing.T) {
	p := geo.MustPoint(5, 5)
	_, err := geo.NewBoundingBox(p, p)
	if err != nil {
		t.Fatalf("NewBoundingBox with equal min/max unexpected error: %v", err)
	}
}

func TestNewBoundingBox_Antimeridian(t *testing.T) {
	min := geo.MustPoint(10, -10)
	max := geo.MustPoint(-10, 10)
	_, err := geo.NewBoundingBox(min, max)
	if !errors.Is(err, geo.ErrAntimeridian) {
		t.Fatalf("NewBoundingBox(min.Lon > max.Lon) error = %v, want ErrAntimeridian", err)
	}
}

func TestNewBoundingBox_LatOrder(t *testing.T) {
	min := geo.MustPoint(-10, 10)
	max := geo.MustPoint(10, -10)
	_, err := geo.NewBoundingBox(min, max)
	if !errors.Is(err, geo.ErrBoundingBoxOrder) {
		t.Fatalf("NewBoundingBox(min.Lat > max.Lat) error = %v, want ErrBoundingBoxOrder", err)
	}
}

func TestBoundingBox_Contains(t *testing.T) {
	bb, _ := geo.NewBoundingBox(geo.MustPoint(-10, -10), geo.MustPoint(10, 10))

	cases := []struct {
		name string
		p    geo.Point
		want bool
	}{
		{"center", geo.MustPoint(0, 0), true},
		{"min corner", geo.MustPoint(-10, -10), true},
		{"max corner", geo.MustPoint(10, 10), true},
		{"min lon max lat", geo.MustPoint(-10, 10), true},
		{"max lon min lat", geo.MustPoint(10, -10), true},
		{"just inside", geo.MustPoint(9.9999, 9.9999), true},
		{"lon too low", geo.MustPoint(-10.0001, 0), false},
		{"lon too high", geo.MustPoint(10.0001, 0), false},
		{"lat too low", geo.MustPoint(0, -10.0001), false},
		{"lat too high", geo.MustPoint(0, 10.0001), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bb.Contains(tc.p)
			if got != tc.want {
				t.Fatalf("BoundingBox.Contains(%v) = %v, want %v", tc.p, got, tc.want)
			}
		})
	}
}
