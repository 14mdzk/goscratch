package geo

import (
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
)

// hexEncodeBytes encodes b as a lowercase hex string without any prefix.
// PostGIS geography columns accept this form as their text-protocol input.
func hexEncodeBytes(b []byte) string {
	return hex.EncodeToString(b)
}

// EWKB constants for a 2-D Point with SRID.
const (
	wkbLittleEndian byte   = 0x01
	wkbPoint        uint32 = 0x00000001
	wkbSRIDFlag     uint32 = 0x20000000
	wkbSRID4326     uint32 = 4326

	// ewkbPointLen is the byte length of an EWKB 2-D Point with SRID:
	//   1  byte  byte order
	//   4  bytes geometry type (wkbPoint | wkbSRIDFlag)
	//   4  bytes SRID
	//   8  bytes X (longitude)
	//   8  bytes Y (latitude)
	ewkbPointLen = 1 + 4 + 4 + 8 + 8
)

// EncodeEWKB encodes p as an Extended Well-Known Binary (EWKB) byte slice
// using little-endian byte order and SRID 4326.
//
// Returns ErrNaNOrInf if either coordinate is NaN or ±Inf.
func (p Point) EncodeEWKB() ([]byte, error) {
	if math.IsNaN(p.Lon) || math.IsInf(p.Lon, 0) ||
		math.IsNaN(p.Lat) || math.IsInf(p.Lat, 0) {
		return nil, ErrNaNOrInf
	}

	buf := make([]byte, ewkbPointLen)
	buf[0] = wkbLittleEndian

	binary.LittleEndian.PutUint32(buf[1:5], wkbPoint|wkbSRIDFlag)
	binary.LittleEndian.PutUint32(buf[5:9], wkbSRID4326)
	binary.LittleEndian.PutUint64(buf[9:17], math.Float64bits(p.Lon))
	binary.LittleEndian.PutUint64(buf[17:25], math.Float64bits(p.Lat))

	return buf, nil
}

// DecodeEWKB decodes an EWKB byte slice (little-endian or big-endian,
// with or without SRID flag) into p.
//
// Returns ErrInvalidWKB if b is nil, too short, has an unexpected geometry
// type, or carries an SRID flag with a value other than 4326. WKB without an
// SRID flag (plain WKB, 21 bytes) is accepted without restriction.
func (p *Point) DecodeEWKB(b []byte) error {
	if len(b) < 21 { // minimum: 1+4+8+8 = 21 (no SRID)
		return fmt.Errorf("%w: need at least 21 bytes, got %d", ErrInvalidWKB, len(b))
	}

	byteOrder := b[0]
	var bo binary.ByteOrder
	switch byteOrder {
	case 0x01:
		bo = binary.LittleEndian
	case 0x00:
		bo = binary.BigEndian
	default:
		return fmt.Errorf("%w: unknown byte order marker 0x%02x", ErrInvalidWKB, byteOrder)
	}

	geomType := bo.Uint32(b[1:5])
	hasSRID := geomType&wkbSRIDFlag != 0
	baseType := geomType &^ wkbSRIDFlag

	if baseType != wkbPoint {
		return fmt.Errorf("%w: expected Point type 0x%08x, got 0x%08x", ErrInvalidWKB, wkbPoint, baseType)
	}

	offset := 5
	if hasSRID {
		if len(b) < ewkbPointLen {
			return fmt.Errorf("%w: SRID-tagged EWKB too short: %d bytes", ErrInvalidWKB, len(b))
		}
		srid := bo.Uint32(b[offset : offset+4])
		if srid != wkbSRID4326 {
			// SRID flag is set: SRID=0 means "unknown" and is ambiguous for a
			// geography(Point,4326) column. Reject anything that is not exactly 4326.
			return fmt.Errorf("%w: unexpected SRID %d (want 4326)", ErrInvalidWKB, srid)
		}
		offset += 4
	}

	lonBits := bo.Uint64(b[offset : offset+8])
	latBits := bo.Uint64(b[offset+8 : offset+16])

	p.Lon = math.Float64frombits(lonBits)
	p.Lat = math.Float64frombits(latBits)

	return nil
}

// Scan implements sql.Scanner so that *Point can be used as a scan destination
// with pgx v5 (and database/sql).
//
// PostGIS sends geography columns as EWKB bytes in binary protocol, or as a
// hex-encoded EWKB string in text protocol. Both forms are handled.
func (p *Point) Scan(src any) error {
	if src == nil {
		return fmt.Errorf("%w: NULL geography cannot be scanned into a non-pointer Point", ErrInvalidWKB)
	}

	var raw []byte
	switch v := src.(type) {
	case []byte:
		raw = v
	case string:
		// Text protocol: hex-encoded EWKB
		b, err := hex.DecodeString(v)
		if err != nil {
			return fmt.Errorf("%w: hex-decode failed: %v", ErrInvalidWKB, err)
		}
		raw = b
	default:
		return fmt.Errorf("%w: unsupported source type %T", ErrInvalidWKB, src)
	}

	return p.DecodeEWKB(raw)
}

// Value implements driver.Valuer so that Point can be passed as a query
// argument to pgx v5 (and database/sql) for a geography(Point,4326) column.
//
// The value is returned as a lowercase hex-encoded EWKB string (e.g.
// "0120000020e6100000..."). pgx v5 transmits string values with OID 25 (text);
// PostGIS interprets hex-encoded EWKB text as its geography text-input form,
// which is the correct production query shape:
//
//	INSERT INTO t (loc) VALUES ($1)   -- $1 = geo.Point
//
// Returning raw []byte would cause pgx to bind the parameter as bytea (OID 17),
// which PostGIS rejects with "parse error - invalid geometry" for geography columns.
func (p Point) Value() (driver.Value, error) {
	b, err := p.EncodeEWKB()
	if err != nil {
		return nil, err
	}
	return hexEncodeBytes(b), nil
}
