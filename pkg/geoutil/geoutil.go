// Package geoutil provides pure-Go spatial computations over the
// internal/shared/domain/geo value types.
//
// Everything here runs without a database round-trip: use these helpers for
// cheap pre-filters, with Postgres remaining the source of truth. No external
// dependencies beyond the standard library.
package geoutil

import (
	"math"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

// earthRadiusMeters is the IUGG mean earth radius. Haversine distances inherit
// its ~0.1% spherical-earth approximation error; callers needing ellipsoid
// precision must use PostGIS geography functions instead.
const earthRadiusMeters = 6371000.0

// Haversine returns the great-circle distance between a and b on a spherical
// earth. The atan2 formulation stays numerically stable for near-identical
// and near-antipodal points, where the asin formulation loses precision.
// Identical points return exactly Distance{0}.
func Haversine(a, b geo.Point) geo.Distance {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLon := (b.Lon - a.Lon) * math.Pi / 180

	sinHalfLat := math.Sin(dLat / 2)
	sinHalfLon := math.Sin(dLon / 2)
	h := sinHalfLat*sinHalfLat + math.Cos(lat1)*math.Cos(lat2)*sinHalfLon*sinHalfLon

	meters := 2 * earthRadiusMeters * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	// Haversine output is non-negative by construction; NewDistance cannot
	// fail here. A defensive zero-clamp absorbs float dust (e.g. -1e-12).
	if meters < 0 {
		meters = 0
	}
	d, err := geo.NewDistance(meters)
	if err != nil {
		return geo.Distance{}
	}
	return d
}
