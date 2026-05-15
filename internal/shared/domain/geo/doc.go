// Package geo provides spatial primitive value types for use across modules.
//
// # Coordinate convention: Lon, Lat (X, Y)
//
// All types in this package follow the PostGIS / GeoJSON / WKT convention where
// longitude (X) comes before latitude (Y). This matches the order used by
// RFC 7946 (GeoJSON), ISO 19111, and PostGIS geography functions. It is the
// single most common footgun in geospatial code — human intuition says
// "latitude, longitude" (e.g. Google Maps) but every standard wire format
// says "longitude, latitude". All constructors and struct literals in this
// package use Lon first, Lat second. Comments on the Point type repeat this
// warning at the call site.
//
// # What is NOT in this package
//
// The following are intentionally deferred to later waves and separate packages:
//
//   - pgx Scan / Value round-trip (WKB encode/decode) — wave 2, depends on the
//     PostGIS testcontainer image bump (F2).
//   - GeoJSON MarshalJSON / UnmarshalJSON — wave 2 (C3).
//   - Haversine, ST_DWithin, point-in-polygon, and other spatial computations —
//     wave 2 (C4, pkg/geoutil).
//   - Any concrete domain module (location, points_of_interest, etc.) — the first
//     consumer defines its own table. This package ships types only.
//
// # Intended usage
//
// Any future module that needs to work with geographic coordinates should consume
// the types defined here rather than redefining them. This package has no
// external dependencies beyond the Go standard library.
package geo
