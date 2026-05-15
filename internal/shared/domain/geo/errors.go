package geo

import "errors"

// ErrInvalidLongitude is returned when a longitude value is outside [-180, 180].
var ErrInvalidLongitude = errors.New("geo: longitude must be in the range [-180, 180]")

// ErrInvalidLatitude is returned when a latitude value is outside [-90, 90].
var ErrInvalidLatitude = errors.New("geo: latitude must be in the range [-90, 90]")

// ErrBoundingBoxOrder is returned when a BoundingBox min coordinate exceeds its max
// coordinate on the same axis.
var ErrBoundingBoxOrder = errors.New("geo: bounding box min must be <= max on both axes")

// ErrAntimeridian is returned when a BoundingBox crosses the antimeridian
// (min.Lon > max.Lon). Antimeridian-crossing bounding boxes are not supported.
var ErrAntimeridian = errors.New("geo: antimeridian-crossing bounding boxes are not supported (min.Lon > max.Lon)")

// ErrPolygonNotClosed is returned when a polygon ring's first point does not
// equal its last point.
var ErrPolygonNotClosed = errors.New("geo: polygon ring is not closed (first point must equal last point)")

// ErrPolygonTooFewPoints is returned when a polygon ring has fewer than 4 points
// (the minimum for a closed, non-degenerate ring).
var ErrPolygonTooFewPoints = errors.New("geo: polygon ring must have at least 4 points")

// ErrNegativeDistance is returned when a Distance is constructed with a negative
// value.
var ErrNegativeDistance = errors.New("geo: distance must be non-negative")

// ErrNaNOrInf is returned when a Point's Lon or Lat is NaN or ±Inf, which
// cannot be encoded in WKB.
var ErrNaNOrInf = errors.New("geo: point coordinates must be finite (NaN/Inf not allowed)")

// ErrInvalidWKB is returned when a byte slice cannot be decoded as a valid
// EWKB-encoded Point geometry.
var ErrInvalidWKB = errors.New("geo: invalid EWKB bytes for Point")
