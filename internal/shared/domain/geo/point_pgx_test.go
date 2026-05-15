package geo_test

import (
	"errors"
	"math"
	"testing"

	"github.com/14mdzk/goscratch/internal/shared/domain/geo"
)

// TestEncodeEWKB_NominalCases checks that well-known coordinates encode to
// the expected EWKB bytes and can be round-tripped through DecodeEWKB.
func TestEncodeEWKB_NominalCases(t *testing.T) {
	cases := []struct {
		name string
		lon  float64
		lat  float64
	}{
		{"origin", 0, 0},
		{"positive", 120.5, 35.7},
		{"negative", -74.006, -33.865},
		{"max", 180, 90},
		{"min", -180, -90},
		{"antipode", -179.9999, -89.9999},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := geo.MustPoint(tc.lon, tc.lat)

			b, err := p.EncodeEWKB()
			if err != nil {
				t.Fatalf("EncodeEWKB(%v,%v) unexpected error: %v", tc.lon, tc.lat, err)
			}

			// Byte order marker must be little-endian.
			if b[0] != 0x01 {
				t.Errorf("byte-order marker = 0x%02x, want 0x01", b[0])
			}

			// Length must be exactly 25 bytes.
			if len(b) != 25 {
				t.Errorf("EWKB length = %d, want 25", len(b))
			}

			// Round-trip decode.
			var got geo.Point
			if err := got.DecodeEWKB(b); err != nil {
				t.Fatalf("DecodeEWKB round-trip error: %v", err)
			}

			if got.Lon != tc.lon || got.Lat != tc.lat {
				t.Errorf("round-trip: got {%v, %v}, want {%v, %v}",
					got.Lon, got.Lat, tc.lon, tc.lat)
			}
		})
	}
}

// TestEncodeEWKB_NaNInf ensures NaN and ±Inf values are rejected.
func TestEncodeEWKB_NaNInf(t *testing.T) {
	cases := []struct {
		name string
		p    geo.Point
	}{
		{"NaN lon", geo.Point{Lon: math.NaN(), Lat: 0}},
		{"NaN lat", geo.Point{Lon: 0, Lat: math.NaN()}},
		{"+Inf lon", geo.Point{Lon: math.Inf(1), Lat: 0}},
		{"-Inf lon", geo.Point{Lon: math.Inf(-1), Lat: 0}},
		{"+Inf lat", geo.Point{Lon: 0, Lat: math.Inf(1)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.p.EncodeEWKB()
			if !errors.Is(err, geo.ErrNaNOrInf) {
				t.Errorf("EncodeEWKB(%v) = %v, want ErrNaNOrInf", tc.p, err)
			}
		})
	}
}

// TestDecodeEWKB_Malformed checks that malformed byte slices return ErrInvalidWKB.
func TestDecodeEWKB_Malformed(t *testing.T) {
	cases := []struct {
		name string
		b    []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"too short (5 bytes)", []byte{0x01, 0x01, 0x00, 0x00, 0x20}},
		{"wrong geometry type (linestring)", func() []byte {
			// Construct a valid-looking EWKB but with type=LineString (0x02)
			b := make([]byte, 25)
			b[0] = 0x01
			// type = LineString | SRID flag
			b[1], b[2], b[3], b[4] = 0x02, 0x00, 0x00, 0x20
			return b
		}()},
		{"bad byte order marker", []byte{0x02, 0x01, 0x00, 0x00, 0x20, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"unexpected SRID 3857", func() []byte {
			b := make([]byte, 25)
			b[0] = 0x01
			// type = Point | SRID flag (little-endian)
			b[1] = 0x01
			b[2] = 0x00
			b[3] = 0x00
			b[4] = 0x20
			// SRID = 3857 (little-endian)
			b[5] = 0x11
			b[6] = 0x0F
			b[7] = 0x00
			b[8] = 0x00
			return b
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p geo.Point
			err := p.DecodeEWKB(tc.b)
			if !errors.Is(err, geo.ErrInvalidWKB) {
				t.Errorf("DecodeEWKB(%q) = %v, want ErrInvalidWKB", tc.b, err)
			}
		})
	}
}

// TestScan_NilReturnsError verifies that scanning SQL NULL returns an error
// (Point is a value type, not a pointer — callers wanting nullable geography
// must use *Point or a wrapper).
func TestScan_NilReturnsError(t *testing.T) {
	var p geo.Point
	err := p.Scan(nil)
	if !errors.Is(err, geo.ErrInvalidWKB) {
		t.Errorf("Scan(nil) = %v, want ErrInvalidWKB", err)
	}
}

// TestScan_UnsupportedType verifies that scanning an unexpected type returns ErrInvalidWKB.
func TestScan_UnsupportedType(t *testing.T) {
	var p geo.Point
	err := p.Scan(42)
	if !errors.Is(err, geo.ErrInvalidWKB) {
		t.Errorf("Scan(int) = %v, want ErrInvalidWKB", err)
	}
}

// TestScan_HexString verifies that scanning a hex-encoded EWKB string works.
func TestScan_HexString(t *testing.T) {
	original := geo.MustPoint(139.6917, 35.6895) // Tokyo

	b, err := original.EncodeEWKB()
	if err != nil {
		t.Fatalf("EncodeEWKB: %v", err)
	}

	// Encode to hex string (text protocol).
	hexStr := make([]byte, len(b)*2)
	for i, by := range b {
		hexStr[i*2] = "0123456789abcdef"[by>>4]
		hexStr[i*2+1] = "0123456789abcdef"[by&0xf]
	}

	var got geo.Point
	if err := got.Scan(string(hexStr)); err != nil {
		t.Fatalf("Scan(hex string): %v", err)
	}

	if got.Lon != original.Lon || got.Lat != original.Lat {
		t.Errorf("Scan hex round-trip: got {%v, %v}, want {%v, %v}",
			got.Lon, got.Lat, original.Lon, original.Lat)
	}
}

// TestScan_BadHex verifies that malformed hex strings return ErrInvalidWKB.
func TestScan_BadHex(t *testing.T) {
	var p geo.Point
	err := p.Scan("not-valid-hex!!")
	if !errors.Is(err, geo.ErrInvalidWKB) {
		t.Errorf("Scan(bad hex) = %v, want ErrInvalidWKB", err)
	}
}

// TestValue_NominalAndNaNInf verifies driver.Valuer behavior.
// Value() returns a lowercase hex-encoded EWKB string so that pgx v5 binds
// it with OID 25 (text), which PostGIS accepts for geography columns. Returning
// []byte would cause pgx to use OID 17 (bytea) and PostGIS would reject it
// with "parse error - invalid geometry".
func TestValue_NominalAndNaNInf(t *testing.T) {
	t.Run("valid point returns hex string", func(t *testing.T) {
		p := geo.MustPoint(-43.1729, -22.9068) // Rio de Janeiro
		v, err := p.Value()
		if err != nil {
			t.Fatalf("Value() error: %v", err)
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("Value() type = %T, want string", v)
		}
		// EWKB 25 bytes → 50 hex chars.
		if len(s) != 50 {
			t.Errorf("Value() hex len = %d, want 50", len(s))
		}
		// Must be valid lower hex (no 0x prefix).
		for i, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("Value() hex char %q at position %d is not lowercase hex", c, i)
				break
			}
		}
		// Round-trip: hex string must decode back to original coordinates.
		var got geo.Point
		if err := got.Scan(s); err != nil {
			t.Fatalf("Scan(Value()) round-trip error: %v", err)
		}
		if got.Lon != p.Lon || got.Lat != p.Lat {
			t.Errorf("round-trip: got {%v, %v}, want {%v, %v}", got.Lon, got.Lat, p.Lon, p.Lat)
		}
	})

	t.Run("NaN returns error", func(t *testing.T) {
		p := geo.Point{Lon: math.NaN(), Lat: 0}
		_, err := p.Value()
		if !errors.Is(err, geo.ErrNaNOrInf) {
			t.Errorf("Value() with NaN = %v, want ErrNaNOrInf", err)
		}
	})
}

// TestDecodeEWKB_SRIDZeroWithFlag verifies that EWKB with the SRID flag set
// but SRID=0 is rejected. SRID=0 is the "unknown" SRID and is ambiguous for
// a geography(Point,4326) column; it must not be silently accepted as 4326.
func TestDecodeEWKB_SRIDZeroWithFlag(t *testing.T) {
	// Construct little-endian EWKB with SRID flag set and SRID=0.
	b := make([]byte, 25)
	b[0] = 0x01 // little-endian
	// geomType = wkbPoint | wkbSRIDFlag = 0x20000001 (little-endian)
	b[1] = 0x01
	b[2] = 0x00
	b[3] = 0x00
	b[4] = 0x20
	// SRID = 0 (all zero bytes)
	b[5], b[6], b[7], b[8] = 0x00, 0x00, 0x00, 0x00

	var p geo.Point
	err := p.DecodeEWKB(b)
	if !errors.Is(err, geo.ErrInvalidWKB) {
		t.Errorf("DecodeEWKB(SRID-flag+SRID=0) = %v, want ErrInvalidWKB", err)
	}
}

// TestEWKB_BigEndianDecode verifies that big-endian EWKB can also be decoded.
func TestEWKB_BigEndianDecode(t *testing.T) {
	// Build a big-endian EWKB for (1.0, 2.0) with SRID 4326.
	b := make([]byte, 25)
	b[0] = 0x00 // big-endian

	// geomType = wkbPoint | wkbSRIDFlag = 0x20000001
	b[1] = 0x20
	b[2] = 0x00
	b[3] = 0x00
	b[4] = 0x01

	// SRID = 4326 = 0x000010E6
	b[5] = 0x00
	b[6] = 0x00
	b[7] = 0x10
	b[8] = 0xE6

	// Lon = 1.0
	lonBits := math.Float64bits(1.0)
	b[9] = byte(lonBits >> 56)
	b[10] = byte(lonBits >> 48)
	b[11] = byte(lonBits >> 40)
	b[12] = byte(lonBits >> 32)
	b[13] = byte(lonBits >> 24)
	b[14] = byte(lonBits >> 16)
	b[15] = byte(lonBits >> 8)
	b[16] = byte(lonBits)

	// Lat = 2.0
	latBits := math.Float64bits(2.0)
	b[17] = byte(latBits >> 56)
	b[18] = byte(latBits >> 48)
	b[19] = byte(latBits >> 40)
	b[20] = byte(latBits >> 32)
	b[21] = byte(latBits >> 24)
	b[22] = byte(latBits >> 16)
	b[23] = byte(latBits >> 8)
	b[24] = byte(latBits)

	var p geo.Point
	if err := p.DecodeEWKB(b); err != nil {
		t.Fatalf("DecodeEWKB big-endian: %v", err)
	}
	if p.Lon != 1.0 || p.Lat != 2.0 {
		t.Errorf("got {%v, %v}, want {1.0, 2.0}", p.Lon, p.Lat)
	}
}

// TestEWKB_NoSRIDDecode verifies that plain WKB (no SRID) can be decoded.
func TestEWKB_NoSRIDDecode(t *testing.T) {
	// Build a little-endian WKB for (3.0, 4.0) without SRID flag.
	b := make([]byte, 21)
	b[0] = 0x01 // little-endian

	// geomType = wkbPoint = 0x00000001 (no SRID flag)
	b[1] = 0x01
	b[2] = 0x00
	b[3] = 0x00
	b[4] = 0x00

	lonBits := math.Float64bits(3.0)
	for i := 0; i < 8; i++ {
		b[5+i] = byte(lonBits >> (uint(i) * 8))
	}
	latBits := math.Float64bits(4.0)
	for i := 0; i < 8; i++ {
		b[13+i] = byte(latBits >> (uint(i) * 8))
	}

	var p geo.Point
	if err := p.DecodeEWKB(b); err != nil {
		t.Fatalf("DecodeEWKB no-SRID: %v", err)
	}
	if p.Lon != 3.0 || p.Lat != 4.0 {
		t.Errorf("got {%v, %v}, want {3.0, 4.0}", p.Lon, p.Lat)
	}
}
