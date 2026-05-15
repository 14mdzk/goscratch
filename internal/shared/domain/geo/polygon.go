package geo

// Polygon represents a geographic polygon with a single exterior ring and
// zero or more interior rings (holes), matching the GeoJSON geometry type.
//
// Each ring must be closed (first point equals last point) and must contain
// at least 4 points (3 unique vertices + 1 closing point). No
// point-in-polygon computation is provided here; that belongs in pkg/geoutil
// (wave 2).
type Polygon struct {
	Exterior []Point
	Holes    [][]Point
}

// NewPolygon constructs a Polygon and validates ring closure and minimum point
// count for the exterior ring and every hole.
//
// Validation rules:
//   - exterior must have ≥ 4 points.
//   - exterior must be closed: exterior[0] == exterior[len-1].
//   - each hole must have ≥ 4 points.
//   - each hole must be closed: hole[0] == hole[len-1].
//
// Returns ErrPolygonTooFewPoints or ErrPolygonNotClosed on violation.
func NewPolygon(exterior []Point, holes ...[]Point) (Polygon, error) {
	if err := validateRing(exterior); err != nil {
		return Polygon{}, err
	}
	for _, hole := range holes {
		if err := validateRing(hole); err != nil {
			return Polygon{}, err
		}
	}
	return Polygon{Exterior: exterior, Holes: holes}, nil
}

// validateRing checks that a ring has at least 4 points and is closed.
func validateRing(ring []Point) error {
	if len(ring) < 4 {
		return ErrPolygonTooFewPoints
	}
	first := ring[0]
	last := ring[len(ring)-1]
	if first.Lon != last.Lon || first.Lat != last.Lat {
		return ErrPolygonNotClosed
	}
	return nil
}
