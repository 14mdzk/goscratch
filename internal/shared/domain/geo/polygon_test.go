package geo_test

import (
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

func closedSquare(lonMin, latMin, lonMax, latMax float64) []geo.Point {
	return []geo.Point{
		geo.MustPoint(lonMin, latMin),
		geo.MustPoint(lonMax, latMin),
		geo.MustPoint(lonMax, latMax),
		geo.MustPoint(lonMin, latMax),
		geo.MustPoint(lonMin, latMin),
	}
}

func TestNewPolygon_Valid(t *testing.T) {
	ext := closedSquare(-10, -10, 10, 10)
	p, err := geo.NewPolygon(ext)
	if err != nil {
		t.Fatalf("NewPolygon unexpected error: %v", err)
	}
	if len(p.Exterior) != len(ext) {
		t.Fatalf("Exterior length = %d, want %d", len(p.Exterior), len(ext))
	}
	if len(p.Holes) != 0 {
		t.Fatalf("expected no holes, got %d", len(p.Holes))
	}
}

func TestNewPolygon_WithHole(t *testing.T) {
	ext := closedSquare(-10, -10, 10, 10)
	hole := closedSquare(-5, -5, 5, 5)
	p, err := geo.NewPolygon(ext, hole)
	if err != nil {
		t.Fatalf("NewPolygon with hole unexpected error: %v", err)
	}
	if len(p.Holes) != 1 {
		t.Fatalf("expected 1 hole, got %d", len(p.Holes))
	}
}

func TestNewPolygon_ExteriorTooFewPoints(t *testing.T) {
	ext := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(1, 0),
		geo.MustPoint(0, 0),
	}
	_, err := geo.NewPolygon(ext)
	if !errors.Is(err, geo.ErrPolygonTooFewPoints) {
		t.Fatalf("error = %v, want ErrPolygonTooFewPoints", err)
	}
}

func TestNewPolygon_ExteriorNotClosed(t *testing.T) {
	ext := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(1, 0),
		geo.MustPoint(1, 1),
		geo.MustPoint(0, 1),
	}
	_, err := geo.NewPolygon(ext)
	if !errors.Is(err, geo.ErrPolygonNotClosed) {
		t.Fatalf("error = %v, want ErrPolygonNotClosed", err)
	}
}

func TestNewPolygon_HoleTooFewPoints(t *testing.T) {
	ext := closedSquare(-10, -10, 10, 10)
	hole := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(1, 0),
		geo.MustPoint(0, 0),
	}
	_, err := geo.NewPolygon(ext, hole)
	if !errors.Is(err, geo.ErrPolygonTooFewPoints) {
		t.Fatalf("error = %v, want ErrPolygonTooFewPoints", err)
	}
}

func TestNewPolygon_HoleNotClosed(t *testing.T) {
	ext := closedSquare(-10, -10, 10, 10)
	hole := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(1, 0),
		geo.MustPoint(1, 1),
		geo.MustPoint(0, 1),
	}
	_, err := geo.NewPolygon(ext, hole)
	if !errors.Is(err, geo.ErrPolygonNotClosed) {
		t.Fatalf("error = %v, want ErrPolygonNotClosed", err)
	}
}

func TestNewPolygon_ExactlyFourPoints(t *testing.T) {
	ext := []geo.Point{
		geo.MustPoint(0, 0),
		geo.MustPoint(1, 0),
		geo.MustPoint(1, 1),
		geo.MustPoint(0, 0),
	}
	_, err := geo.NewPolygon(ext)
	if err != nil {
		t.Fatalf("NewPolygon with 4 points unexpected error: %v", err)
	}
}
