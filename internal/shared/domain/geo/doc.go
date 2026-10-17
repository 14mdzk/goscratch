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
//   - pgx Scan / Value round-trip (WKB encode/decode) — shipped in wave 2
//     (point_pgx.go, Tier C2).
//   - GeoJSON MarshalJSON / UnmarshalJSON — shipped in wave 2
//     (point_geojson.go, polygon_geojson.go, Tier C3).
//   - Haversine — shipped in wave 2 (pkg/geoutil, Tier C4).
//     ST_DWithin wrappers and point-in-polygon stay deferred: no consumer yet.
//   - Any concrete domain module (location, points_of_interest, etc.) — the first
//     consumer defines its own table. This package ships types only.
//
// # Intended usage
//
// Any future module that needs to work with geographic coordinates should consume
// the types defined here rather than redefining them. This package has no
// external dependencies beyond the Go standard library.
package geo
