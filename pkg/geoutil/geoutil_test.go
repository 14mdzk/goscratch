package geoutil_test

import (
	"math"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
	"github.com/14mdzk/goscratch/pkg/geoutil"
)

// withinRel fails the test when got differs from want by more than relTol
// relative (plus an absolute floor for near-zero expectations).
func withinRel(t *testing.T, got, want, relTol float64) {
	t.Helper()
	diff := math.Abs(got - want)
	allow := math.Abs(want)*relTol + 1e-9
	if diff > allow {
		t.Fatalf("got %v, want %v (rel tol %v)", got, want, relTol)
	}
}

func TestHaversine(t *testing.T) {
	nyc := geo.MustPoint(-74.006, 40.7128)
	london := geo.MustPoint(-0.1278, 51.5074)

	cases := []struct {
		name string
		a, b geo.Point
		want float64 // meters
		tol  float64 // relative tolerance
	}{
		{"identical points are exactly zero", nyc, nyc, 0, 0},
		// One degree of equator: 2πR/360 with R = 6,371,000 m.
		{"one degree on equator", geo.MustPoint(0, 0), geo.MustPoint(1, 0), 111194.93, 1e-6},
		// Published NYC–London great-circle distance ≈ 5,570 km.
		{"new york to london", nyc, london, 5570000, 0.005},
		// Antipodes: half the circumference πR.
		{"antipodes", geo.MustPoint(0, 0), geo.MustPoint(180, 0), math.Pi * 6371000, 1e-9},
		// Pole to pole: also πR via a different path.
		{"north to south pole", geo.MustPoint(0, 90), geo.MustPoint(0, -90), math.Pi * 6371000, 1e-9},
		// Near-identical points: atan2 formulation must not collapse.
		{"one meter apart", geo.MustPoint(0, 0), geo.MustPoint(0.000009, 0), 1.0, 0.01},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := geoutil.Haversine(tc.a, tc.b).Meters
			withinRel(t, got, tc.want, tc.tol)
			// Symmetry: H(a,b) == H(b,a).
			back := geoutil.Haversine(tc.b, tc.a).Meters
			withinRel(t, back, got, 1e-12)
		})
	}
}

func TestHaversine_NonNegative(t *testing.T) {
	// Fuzz-ish sweep: no pair may ever yield a negative distance.
	pts := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(180, 0),
		geo.MustPoint(-180, 0),
		geo.MustPoint(0, 90),
		geo.MustPoint(0, -90),
		geo.MustPoint(106.8456, -6.2088),
		geo.MustPoint(-0.1278, 51.5074),
	}
	for _, a := range pts {
		for _, b := range pts {
			if d := geoutil.Haversine(a, b); d.Meters < 0 {
				t.Fatalf("Haversine(%v, %v) = %v, want >= 0", a, b, d.Meters)
			}
		}
	}
}
