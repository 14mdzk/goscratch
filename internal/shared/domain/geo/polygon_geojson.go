package geo

import (
	"encoding/json"
	"fmt"
)

// polygonGeoJSON is the RFC 7946 §3.1.6 wire shape for a Polygon:
// exterior ring first, then holes.
type polygonGeoJSON struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// MarshalJSON encodes the Polygon as an RFC 7946 Polygon geometry with
// Lon-first positions.
func (p Polygon) MarshalJSON() ([]byte, error) {
	rings := make([][][2]float64, 0, len(p.Holes)+1)
	rings = append(rings, pointsToPositions(p.Exterior))
	for _, hole := range p.Holes {
		rings = append(rings, pointsToPositions(hole))
	}
	return json.Marshal(polygonGeoJSON{
		Type:        geoJSONTypePolygon,
		Coordinates: rings,
	})
}

// UnmarshalJSON decodes an RFC 7946 Polygon geometry. The type member must be
// "Polygon" with at least one ring; ring validation (closure, minimum count)
// runs through NewPolygon so the rules stay in one place. Each position is
// range-validated through NewPoint. As with Point, syntactically-invalid JSON
// yields *json.SyntaxError from encoding/json, not ErrInvalidGeoJSON.
func (p *Polygon) UnmarshalJSON(data []byte) error {
	var g polygonGeoJSON
	if err := json.Unmarshal(data, &g); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidGeoJSON, err)
	}
	if g.Type != geoJSONTypePolygon {
		return fmt.Errorf("%w: expected type %q, got %q", ErrInvalidGeoJSON, geoJSONTypePolygon, g.Type)
	}
	if len(g.Coordinates) == 0 {
		return fmt.Errorf("%w: Polygon must have at least one ring", ErrInvalidGeoJSON)
	}
	rings := make([][]Point, 0, len(g.Coordinates))
	for _, ring := range g.Coordinates {
		pts := make([]Point, 0, len(ring))
		for _, pos := range ring {
			pt, err := NewPoint(pos[0], pos[1])
			if err != nil {
				return err
			}
			pts = append(pts, pt)
		}
		rings = append(rings, pts)
	}
	poly, err := NewPolygon(rings[0], rings[1:]...)
	if err != nil {
		return err
	}
	*p = poly
	return nil
}

// pointsToPositions converts a ring to Lon-first GeoJSON positions.
func pointsToPositions(ring []Point) [][2]float64 {
	pos := make([][2]float64, 0, len(ring))
	for _, pt := range ring {
		pos = append(pos, [2]float64{pt.Lon, pt.Lat})
	}
	return pos
}
