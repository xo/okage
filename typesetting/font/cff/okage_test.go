// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package cff

// okage: this file is not in go-text. It tests the additions of D59 with
// small CFF fonts that it builds.

import (
	"encoding/binary"
	"math"
	"strconv"
	"testing"

	ps "github.com/xo/okage/typesetting/font/cff/interpreter"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// The operators of the DICTs and the charstrings that the tests write. An
// escaped operator is 1200 plus its second byte.
const (
	opCharset     = 15
	opCharStrings = 17
	opPrivate     = 18
	opFontMatrix  = 1207
	opROS         = 1230
	opFDArray     = 1236
	opFDSelect    = 1237

	opRMoveTo = 21
	opRLineTo = 5
	opEndChar = 14
)

// testOps encodes operands and operators. An int is an operator, an int32
// is an integer operand of five bytes, and a float64 is a real operand.
func testOps(items ...any) []byte {
	var b []byte
	for _, it := range items {
		switch v := it.(type) {
		case int:
			if v >= 1200 {
				b = append(b, 12, byte(v-1200))
			} else {
				b = append(b, byte(v))
			}
		case int32:
			b = append(b, 29)
			b = binary.BigEndian.AppendUint32(b, uint32(v))
		case float64:
			b = append(b, testReal(v)...)
		}
	}
	return b
}

// testReal encodes a real operand of a DICT.
func testReal(v float64) []byte {
	var nibbles []byte
	for _, c := range strconv.FormatFloat(v, 'f', -1, 64) {
		switch {
		case c >= '0' && c <= '9':
			nibbles = append(nibbles, byte(c-'0'))
		case c == '.':
			nibbles = append(nibbles, 0xa)
		case c == '-':
			nibbles = append(nibbles, 0xe)
		}
	}
	nibbles = append(nibbles, 0xf)
	if len(nibbles)%2 != 0 {
		nibbles = append(nibbles, 0xf)
	}
	b := []byte{30}
	for i := 0; i < len(nibbles); i += 2 {
		b = append(b, nibbles[i]<<4|nibbles[i+1])
	}
	return b
}

// testCharstring encodes a Type 2 charstring. An int is an operator, and
// a float64 is an operand of three bytes.
func testCharstring(items ...any) []byte {
	var b []byte
	for _, it := range items {
		switch v := it.(type) {
		case int:
			b = append(b, byte(v))
		case float64:
			b = append(b, 28)
			b = binary.BigEndian.AppendUint16(b, uint16(int16(v)))
		}
	}
	return b
}

// testIndex encodes an INDEX with offsets of four bytes.
func testIndex(items ...[]byte) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(items)))
	if len(items) == 0 {
		return b
	}
	b = append(b, 4)
	off := uint32(1)
	b = binary.BigEndian.AppendUint32(b, off)
	for _, it := range items {
		off += uint32(len(it))
		b = binary.BigEndian.AppendUint32(b, off)
	}
	for _, it := range items {
		b = append(b, it...)
	}
	return b
}

// testFont describes a CFF font for buildTestFont.
type testFont struct {
	glyphs [][]byte // the charstrings
	// charset holds the SIDs of a charset of format 0, without the glyph
	// 0. If it is nil, the font has the predefined charset ISOAdobe.
	charset []uint16
	// top holds more operands and operators of the Top DICT.
	top []byte
	// fds holds the font dictionaries of a CIDFont, without their
	// Private, and fdSelect the font dictionary of each glyph.
	fds      [][]byte
	fdSelect []byte
}

// buildTestFont writes a CFF font. The offsets of the Top DICT have five
// bytes, so that the size of the Top DICT does not depend on them.
func buildTestFont(f testFont) []byte {
	header := []byte{1, 0, 4, 4}
	names := testIndex([]byte("Test"))
	topDict := func(charset, chars, fdArray, fdSelect int32) []byte {
		d := append([]byte(nil), f.top...)
		if f.charset != nil {
			d = append(d, testOps(charset, opCharset)...)
		}
		d = append(d, testOps(chars, opCharStrings)...)
		if f.fds != nil {
			d = append(d, testOps(int32(0), int32(0), int32(0), opROS, fdArray, opFDArray, fdSelect, opFDSelect)...)
		} else {
			d = append(d, testOps(int32(0), int32(0), opPrivate)...)
		}
		return d
	}
	start := len(header) + len(names) + len(testIndex(topDict(0, 0, 0, 0))) + 2*len(testIndex())
	var tail []byte
	charset := int32(start + len(tail))
	if f.charset != nil {
		tail = append(tail, 0)
		for _, sid := range f.charset {
			tail = binary.BigEndian.AppendUint16(tail, sid)
		}
	}
	chars := int32(start + len(tail))
	tail = append(tail, testIndex(f.glyphs...)...)
	fdArray := int32(start + len(tail))
	if f.fds != nil {
		var dicts [][]byte
		for _, fd := range f.fds {
			dicts = append(dicts, append(append([]byte(nil), fd...), testOps(int32(0), int32(0), opPrivate)...))
		}
		tail = append(tail, testIndex(dicts...)...)
	}
	fdSelect := int32(start + len(tail))
	if f.fds != nil {
		tail = append(tail, 0)
		tail = append(tail, f.fdSelect...)
	}
	var b []byte
	b = append(b, header...)
	b = append(b, names...)
	b = append(b, testIndex(topDict(charset, chars, fdArray, fdSelect))...)
	b = append(b, testIndex()...) // String INDEX
	b = append(b, testIndex()...) // Global Subr INDEX
	return append(b, tail...)
}

// testSquare returns a charstring that draws the rectangle with the corner
// x, y and the size w, h, and ends with the operands of end.
func testSquare(x, y, w, h float64, end ...any) []byte {
	items := []any{x, y, opRMoveTo, w, 0., opRLineTo, 0., h, opRLineTo, -w, 0., opRLineTo}
	items = append(items, end...)
	return testCharstring(append(items, opEndChar)...)
}

// testPoints returns the points of the segments, in order.
func testPoints(segs []ot.Segment) [][2]float32 {
	var out [][2]float32
	for _, s := range segs {
		for _, p := range s.ArgsSlice() {
			out = append(out, [2]float32{p.X, p.Y})
		}
	}
	return out
}

// sameNumber tells whether a and b are equal within a small part of
// their size.
func sameNumber(a, b float64) bool {
	return math.Abs(a-b) <= 1e-6*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}

// checkGlyph loads a glyph and compares its points and bounds.
func checkGlyph(t *testing.T, f *CFF, glyph tables.GlyphID, points [][2]float32, bounds ps.PathBounds) {
	t.Helper()
	segs, got, err := f.LoadGlyph(glyph)
	if err != nil {
		t.Fatalf("glyph %d: %v", glyph, err)
	}
	pts := testPoints(segs)
	if len(pts) != len(points) {
		t.Fatalf("glyph %d: expected the points %v, got %v", glyph, points, pts)
	}
	for i := range pts {
		if !sameNumber(float64(pts[i][0]), float64(points[i][0])) || !sameNumber(float64(pts[i][1]), float64(points[i][1])) {
			t.Fatalf("glyph %d: expected the points %v, got %v", glyph, points, pts)
		}
	}
	if !sameNumber(got.Min.X, bounds.Min.X) || !sameNumber(got.Min.Y, bounds.Min.Y) ||
		!sameNumber(got.Max.X, bounds.Max.X) || !sameNumber(got.Max.Y, bounds.Max.Y) {
		t.Fatalf("glyph %d: expected the bounds %v, got %v", glyph, bounds, got)
	}
}

// squarePoints returns the points of testSquare, moved by the matrix.
func squarePoints(x, y, w, h float64, m fontMatrix) [][2]float32 {
	var out [][2]float32
	for _, p := range [][2]float64{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}, {x, y}} {
		q := m.apply(ot.SegmentPoint{X: float32(p[0]), Y: float32(p[1])})
		out = append(out, [2]float32{q.X, q.Y})
	}
	return out
}

func TestPredefinedCharsets(t *testing.T) {
	if len(CharsetISOAdobe) != 229 || len(CharsetExpert) != 166 || len(CharsetExpertSubset) != 87 {
		t.Fatal("unexpected lengths of the predefined charsets")
	}
	for i, sid := range CharsetISOAdobe {
		if int(sid) != i {
			t.Fatalf("ISOAdobe: expected the SID %d for the glyph %d, got %d", i, i, sid)
		}
	}
	if CharsetExpert[2] != 229 || CharsetExpert[165] != 378 || CharsetExpertSubset[86] != 346 {
		t.Fatal("unexpected SIDs in the predefined charsets")
	}
	// A change to an exported charset does not change the parser.
	CharsetISOAdobe[34] = 0
	defer func() { CharsetISOAdobe[34] = 34 }()
	if charsetISOAdobe[34] != 34 {
		t.Fatal("the exported charset shares its array with the parser")
	}
}

func TestStandardEncoding(t *testing.T) {
	for code, name := range map[int]string{
		'A': "A", 'z': "z", ' ': "space", 0xa1: "exclamdown", 0xc2: "acute",
		0xc8: "dieresis", 0xe1: "AE", 0xfb: "germandbls",
	} {
		if got := stdStrings[standardEncoding[code]]; got != name {
			t.Errorf("code %#x: expected %q, got %q", code, name, got)
		}
	}
}

// seacGlyphs returns the charstrings of a font that has the base glyph A
// at baseGID, the accent acute at accentGID, and two glyphs of seac after
// them: one without a width and one with a width.
func seacGlyphs(n, baseGID, accentGID int) [][]byte {
	glyphs := make([][]byte, n)
	for i := range glyphs {
		glyphs[i] = testCharstring(opEndChar)
	}
	glyphs[baseGID] = testSquare(10, 0, 400, 600)
	glyphs[accentGID] = testSquare(0, 700, 100, 50)
	// adx ady bchar achar endchar: A and acute of StandardEncoding.
	glyphs[n-2] = testCharstring(100., 200., 65., 194., opEndChar)
	glyphs[n-1] = testCharstring(500., 100., 200., 65., 194., opEndChar)
	return glyphs
}

func TestLoadGlyphSeac(t *testing.T) {
	identity := fontMatrix{1, 0, 0, 1, 0, 0}
	want := append(squarePoints(10, 0, 400, 600, identity), squarePoints(100, 900, 100, 50, identity)...)
	bounds := ps.PathBounds{Min: ps.Point{X: 10, Y: 0}, Max: ps.Point{X: 410, Y: 950}}

	// The charset ISOAdobe: the SID of A is 34 and the SID of acute is 125.
	font, err := Parse(buildTestFont(testFont{glyphs: seacGlyphs(130, 34, 125)}))
	if err != nil {
		t.Fatal(err)
	}
	if font.GlyphName(34) != "A" || font.GlyphName(125) != "acute" {
		t.Fatalf("unexpected glyph names %q and %q", font.GlyphName(34), font.GlyphName(125))
	}
	checkGlyph(t, font, 128, want, bounds)
	checkGlyph(t, font, 129, want, bounds)

	// A charset of format 0.
	font, err = Parse(buildTestFont(testFont{glyphs: seacGlyphs(5, 1, 2), charset: []uint16{34, 125, 200, 201}}))
	if err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, font, 3, want, bounds)
	checkGlyph(t, font, 4, want, bounds)

	// Without the accent, the seac draws the base glyph.
	font, err = Parse(buildTestFont(testFont{glyphs: seacGlyphs(5, 1, 2), charset: []uint16{34, 126, 200, 201}}))
	if err != nil {
		t.Fatal(err)
	}
	checkGlyph(t, font, 3, squarePoints(10, 0, 400, 600, identity),
		ps.PathBounds{Min: ps.Point{X: 10, Y: 0}, Max: ps.Point{X: 410, Y: 600}})
}

func TestLoadGlyphFontMatrix(t *testing.T) {
	glyphs := [][]byte{testCharstring(opEndChar), testSquare(10, 0, 400, 600)}
	for _, test := range []struct {
		name       string
		top        []byte
		unitsPerEm float64
		m          fontMatrix
	}{
		{"none", nil, 1000, fontMatrix{1, 0, 0, 1, 0, 0}},
		{"1/2000", testOps(0.0005, 0., 0., 0.0005, 0., 0., opFontMatrix), 2000, fontMatrix{1, 0, 0, 1, 0, 0}},
		{"skew", testOps(0.001, 0., 0.0002, 0.001, 0., 0., opFontMatrix), 1000, fontMatrix{1, 0, 0.2, 1, 0, 0}},
		{"move", testOps(0.001, 0., 0., 0.001, 0.05, 0., opFontMatrix), 1000, fontMatrix{1, 0, 0, 1, 50, 0}},
		{"degenerate", testOps(0., 0., 0., 0., 0., 0., opFontMatrix), 1000, fontMatrix{1, 0, 0, 1, 0, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			top := append([]byte(nil), test.top...)
			font, err := Parse(buildTestFont(testFont{glyphs: glyphs, charset: []uint16{34}, top: top}))
			if err != nil {
				t.Fatal(err)
			}
			if got := font.UnitsPerEm(); !sameNumber(got, test.unitsPerEm) {
				t.Fatalf("expected %v units per em, got %v", test.unitsPerEm, got)
			}
			pts := squarePoints(10, 0, 400, 600, test.m)
			var bounds ps.PathBounds
			for i, p := range pts {
				pt := ps.Point{X: float64(p[0]), Y: float64(p[1])}
				if i == 0 {
					bounds.Min, bounds.Max = pt, pt
				}
				bounds.Enlarge(pt)
			}
			checkGlyph(t, font, 1, pts, bounds)
		})
	}
}

func TestLoadGlyphCIDFontMatrix(t *testing.T) {
	glyphs := [][]byte{
		testCharstring(opEndChar),
		testSquare(10, 0, 400, 600),
		testSquare(10, 0, 400, 600),
		// A seac does not apply in a CIDFont.
		testCharstring(100., 200., 1., 2., opEndChar),
	}
	fds := [][]byte{
		testOps(0.001, 0., 0., 0.001, 0., 0., opFontMatrix),
		testOps(0.002, 0., 0., 0.002, 0.01, 0., opFontMatrix),
	}
	for _, test := range []struct {
		name       string
		top        []byte
		unitsPerEm float64
		m1, m2     fontMatrix
	}{
		// The Top DICT has the identity: a unit is an em.
		{
			"identity", testOps(1., 0., 0., 1., 0., 0., opFontMatrix), 1,
			fontMatrix{0.001, 0, 0, 0.001, 0, 0}, fontMatrix{0.002, 0, 0, 0.002, 0.01, 0},
		},
		// The translation of the Top DICT does not apply.
		{
			"identity moved", testOps(1., 0., 0., 1., 5., 5., opFontMatrix), 1,
			fontMatrix{0.001, 0, 0, 0.001, 0, 0}, fontMatrix{0.002, 0, 0, 0.002, 0.01, 0},
		},
		// Without a FontMatrix in the Top DICT, the font dictionary applies alone.
		{
			"none", nil, 1000,
			fontMatrix{1, 0, 0, 1, 0, 0}, fontMatrix{2, 0, 0, 2, 10, 0},
		},
		{
			"half", testOps(0.5, 0., 0., 0.5, 0., 0., opFontMatrix), 2,
			fontMatrix{0.001, 0, 0, 0.001, 0, 0}, fontMatrix{0.002, 0, 0, 0.002, 0.01, 0},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			top := append([]byte(nil), test.top...)
			font, err := Parse(buildTestFont(testFont{glyphs: glyphs, top: top, fds: fds, fdSelect: []byte{0, 0, 1, 0}}))
			if err != nil {
				t.Fatal(err)
			}
			if got := font.UnitsPerEm(); !sameNumber(got, test.unitsPerEm) {
				t.Fatalf("expected %v units per em, got %v", test.unitsPerEm, got)
			}
			for glyph, m := range map[tables.GlyphID]fontMatrix{1: test.m1, 2: test.m2} {
				pts := squarePoints(10, 0, 400, 600, m)
				bounds := ps.PathBounds{
					Min: ps.Point{X: float64(pts[0][0]), Y: float64(pts[0][1])},
					Max: ps.Point{X: float64(pts[2][0]), Y: float64(pts[2][1])},
				}
				checkGlyph(t, font, glyph, pts, bounds)
			}
			segs, _, err := font.LoadGlyph(3)
			if err != nil || len(segs) != 0 {
				t.Fatalf("expected no outline for a seac in a CIDFont, got %v, %v", segs, err)
			}
		})
	}
}
