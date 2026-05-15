package geo

// BoundingBox represents an axis-aligned geographic bounding box.
//
// Min is the south-west corner (minimum longitude, minimum latitude).
// Max is the north-east corner (maximum longitude, maximum latitude).
//
// Antimeridian-crossing bounding boxes (where Min.Lon > Max.Lon) are not
// supported. NewBoundingBox returns ErrAntimeridian in that case.
type BoundingBox struct {
	Min Point
	Max Point
}

// NewBoundingBox constructs a BoundingBox and validates that sw ≤ ne on
// both axes. Returns ErrAntimeridian if sw.Lon > ne.Lon, or
// ErrBoundingBoxOrder if sw.Lat > ne.Lat.
func NewBoundingBox(sw, ne Point) (BoundingBox, error) {
	if sw.Lon > ne.Lon {
		return BoundingBox{}, ErrAntimeridian
	}
	if sw.Lat > ne.Lat {
		return BoundingBox{}, ErrBoundingBoxOrder
	}
	return BoundingBox{Min: sw, Max: ne}, nil
}

// Contains reports whether p lies within the bounding box.
// The check is inclusive on both Min and Max (closed interval on all four sides),
// following the GeoJSON RFC 7946 §5 convention.
func (b BoundingBox) Contains(p Point) bool {
	return p.Lon >= b.Min.Lon && p.Lon <= b.Max.Lon &&
		p.Lat >= b.Min.Lat && p.Lat <= b.Max.Lat
}
