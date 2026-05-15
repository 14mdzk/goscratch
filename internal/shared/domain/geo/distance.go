package geo

// Distance represents a non-negative geographic distance in meters.
// Meters is the single canonical storage unit; conversion helpers are
// provided for common display units.
type Distance struct {
	Meters float64
}

// Unit constants for common distances.
var (
	Meter        = Distance{Meters: 1}
	Kilometer    = Distance{Meters: 1000}
	Mile         = Distance{Meters: 1609.344}
	NauticalMile = Distance{Meters: 1852}
)

// NewDistance constructs a Distance. Returns ErrNegativeDistance if meters
// is negative.
func NewDistance(meters float64) (Distance, error) {
	if meters < 0 {
		return Distance{}, ErrNegativeDistance
	}
	return Distance{Meters: meters}, nil
}

// Kilometers returns the distance expressed in kilometers.
func (d Distance) Kilometers() float64 {
	return d.Meters / 1000
}

// Miles returns the distance expressed in international miles.
func (d Distance) Miles() float64 {
	return d.Meters / 1609.344
}

// NauticalMiles returns the distance expressed in nautical miles.
func (d Distance) NauticalMiles() float64 {
	return d.Meters / 1852
}

// Add returns the sum of d and other.
func (d Distance) Add(other Distance) Distance {
	return Distance{Meters: d.Meters + other.Meters}
}

// Sub returns d minus other, clamped to zero on underflow.
// No error is returned; callers that need to detect underflow should compare
// d.Meters and other.Meters before calling Sub.
func (d Distance) Sub(other Distance) Distance {
	result := d.Meters - other.Meters
	if result < 0 {
		result = 0
	}
	return Distance{Meters: result}
}
