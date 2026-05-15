package geo

// Point is a geographic coordinate value type.
//
// IMPORTANT: The field order is Lon (longitude / X) then Lat (latitude / Y).
// This matches the PostGIS / GeoJSON / WKT convention. Do NOT swap the
// arguments — longitude comes first, latitude second.
type Point struct {
	Lon float64
	Lat float64
}

// NewPoint constructs a Point and validates the coordinate range.
// lon must be in [-180, 180]; lat must be in [-90, 90].
// Returns ErrInvalidLongitude or ErrInvalidLatitude on out-of-range values.
func NewPoint(lon, lat float64) (Point, error) {
	if lon < -180 || lon > 180 {
		return Point{}, ErrInvalidLongitude
	}
	if lat < -90 || lat > 90 {
		return Point{}, ErrInvalidLatitude
	}
	return Point{Lon: lon, Lat: lat}, nil
}

// MustPoint constructs a Point and panics if the coordinates are invalid.
// Intended for use in tests and compile-time literals where the values are
// known-good constants.
func MustPoint(lon, lat float64) Point {
	p, err := NewPoint(lon, lat)
	if err != nil {
		panic("geo.MustPoint: " + err.Error())
	}
	return p
}

// Validate re-validates the point's coordinate range.
// Useful when a Point has been constructed via a struct literal.
func (p Point) Validate() error {
	if p.Lon < -180 || p.Lon > 180 {
		return ErrInvalidLongitude
	}
	if p.Lat < -90 || p.Lat > 90 {
		return ErrInvalidLatitude
	}
	return nil
}
