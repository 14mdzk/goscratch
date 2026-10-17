package geo

import (
	"encoding/json"
	"fmt"
)

// GeoJSON geometry type members (RFC 7946 §3).
const (
	geoJSONTypePoint   = "Point"
	geoJSONTypePolygon = "Polygon"
)

// pointGeoJSON is the RFC 7946 §3.1.2 wire shape for a Point.
type pointGeoJSON struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// MarshalJSON encodes the Point as an RFC 7946 Point geometry with
// Lon-first coordinates and no altitude.
func (p Point) MarshalJSON() ([]byte, error) {
	return json.Marshal(pointGeoJSON{
		Type:        geoJSONTypePoint,
		Coordinates: []float64{p.Lon, p.Lat},
	})
}

// UnmarshalJSON decodes an RFC 7946 Point geometry. The type member must be
// "Point" with exactly two numeric coordinates; range validation runs through
// NewPoint so out-of-range input returns ErrInvalidLongitude /
// ErrInvalidLatitude rather than a silently-wrapped literal.
// Syntactically-invalid JSON never reaches this method (encoding/json fails
// the scan first with *json.SyntaxError); ErrInvalidGeoJSON covers
// semantically-wrong GeoJSON only.
func (p *Point) UnmarshalJSON(data []byte) error {
	var g pointGeoJSON
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidGeoJSON, err)
	}
	if g.Type != geoJSONTypePoint {
		return fmt.Errorf("%w: expected type %q, got %q", ErrInvalidGeoJSON, geoJSONTypePoint, g.Type)
	}
	if len(g.Coordinates) != 2 {
		return fmt.Errorf("%w: Point coordinates must have exactly 2 elements, got %d", ErrInvalidGeoJSON, len(g.Coordinates))
	}
	pt, err := NewPoint(g.Coordinates[0], g.Coordinates[1])
	if err != nil {
		return err
	}
	*p = pt
	return nil
}
