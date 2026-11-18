// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/xo/okage/kurbo"
	"github.com/xo/okage/roxmltree"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests the instances of variable fonts of interjector_skrifa.go
// and cff2.go. TestFontToolsVariations ports the tests of the variable fonts
// of subsetter tests/src/font_tools.rs. Upstream dumps the instance with
// fonttools ttx, and compares the text with the files of tests/ttx, which
// testdata holds. The port reads the outlines and the metrics of each glyph
// from these files, and compares them with the instance.
//
// typesetting computes the deltas of the variations in float32, where skrifa
// computes them in fixed point, so a coordinate of an instance can differ by
// 1 unit, and an on-curve point at the middle of two off-curve points can be
// implied in one instance and not in the other. The test of each glyph
// allows that.

// fontToolsVariations lists the lines of tests/data/fonttools.tests of
// subsetter with variations: the font, the glyphs, the variations and the
// file of tests/ttx. skip gives the reason to skip a line.
var fontToolsVariations = []struct {
	font, gids string
	vars       []Variation
	ttx        string
	skip       string
}{
	{"NotoSans-Regular_var.ttf", "10,40,58,201-205", []Variation{{Tag{'w', 'g', 'h', 't'}, 400}}, "NotoSans-Regular_var_1.ttx", ""},
	{"NotoSans-Regular_var.ttf", "10,40,58,201-205", []Variation{{Tag{'w', 'g', 'h', 't'}, 900}}, "NotoSans-Regular_var_2.ttx", ""},
	{"NotoSans-Regular_var.ttf", "10,40,58,201-205", []Variation{{Tag{'w', 'g', 'h', 't'}, 800}, {Tag{'w', 'd', 't', 'h'}, 70}}, "NotoSans-Regular_var_3.ttx", ""},
	{"Cantarell-VF.otf", "1,15,30-35,40,103-105", []Variation{{Tag{'w', 'g', 'h', 't'}, 400}}, "Cantarell-VF_1.ttx", ""},
	{"Cantarell-VF.otf", "1,15,30-35,40,103-105", []Variation{{Tag{'w', 'g', 'h', 't'}, 800}}, "Cantarell-VF_2.ttx", ""},
}

// TestFontToolsVariations is test_font_tools in subsetter
// tests/src/main.rs for the lines of tests/data/fonttools.tests with
// variations.
func TestFontToolsVariations(t *testing.T) {
	for _, tc := range fontToolsVariations {
		t.Run(tc.ttx, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			data := readTestFont(t, tc.font)
			f, err := parse(data, 0)
			if err != nil {
				t.Fatal(err)
			}
			maxp, ok := f.table(tagMAXP)
			if !ok || len(maxp) < 6 {
				t.Fatal("the font has no maxp table")
			}
			numGlyphs := binary.BigEndian.Uint16(maxp[4:])
			subset, err := SubsetWithVariations(data, 0, tc.vars, NewGlyphRemapperFromGlyphs(parseGids(tc.gids, numGlyphs)))
			if err != nil {
				t.Fatal(err)
			}
			want := readTTX(t, filepath.Join("testdata", tc.ttx))
			got := readGlyphTables(t, subset)
			if int(got.numGlyphs) != len(want) {
				t.Fatalf("the instance has %d glyphs, and the ttx file has %d", got.numGlyphs, len(want))
			}
			for gid, w := range want {
				g := tables.GlyphID(gid)
				if !within1(int(got.hmtx.Advance(g)), w.advance) || !within1(int(got.hmtx.SideBearing(g)), w.lsb) {
					t.Errorf("the glyph %d has the metrics (%d, %d), and the ttx file has (%d, %d)",
						gid, got.hmtx.Advance(g), got.hmtx.SideBearing(g), w.advance, w.lsb)
				}
				var contours [][]curvePoint
				if sg, ok := got.glyf[gid].Data.(tables.SimpleGlyph); ok {
					start := 0
					for _, end := range sg.EndPtsOfContours {
						var c []curvePoint
						for _, p := range sg.Points[start : end+1] {
							c = append(c, curvePoint{p.X, p.Y, p.Flag&flagOnCurvePoint != 0})
						}
						contours = append(contours, c)
						start = int(end) + 1
					}
				}
				if !sameContours(contours, w.contours) {
					t.Errorf("the glyph %d has the contours %v, and the ttx file has %v", gid, contours, w.contours)
				}
			}
		})
	}
}

// ttxGlyph is a glyph of a ttx file: its metrics and its contours.
type ttxGlyph struct {
	advance, lsb int
	contours     [][]curvePoint
}

// readTTX reads the glyph order, the hmtx table and the glyf table of a
// file of fonttools ttx.
func readTTX(t *testing.T, name string) []ttxGlyph {
	t.Helper()
	text, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := roxmltree.Parse(string(text))
	if err != nil {
		t.Fatal(err)
	}
	attr := func(n roxmltree.Node, name string) string {
		v, _ := n.Attribute(roxmltree.ExpandedNameFromStr(name))
		return v
	}
	num := func(n roxmltree.Node, name string) int {
		v, err := strconv.Atoi(attr(n, name))
		if err != nil {
			t.Fatalf("%s: the attribute %s: %v", name, name, err)
		}
		return v
	}
	ids := map[string]int{}
	var glyphs []ttxGlyph
	for n := range doc.Root().Descendants() {
		if !n.IsElement() {
			continue
		}
		switch n.TagName().Name() {
		case "GlyphID":
			ids[attr(n, "name")] = num(n, "id")
			glyphs = append(glyphs, ttxGlyph{})
		case "mtx":
			g := &glyphs[ids[attr(n, "name")]]
			g.advance, g.lsb = num(n, "width"), num(n, "lsb")
		case "TTGlyph":
			g := &glyphs[ids[attr(n, "name")]]
			for c := range n.Children() {
				if !c.IsElement() || c.TagName().Name() != "contour" {
					continue
				}
				var contour []curvePoint
				for p := range c.Children() {
					if p.IsElement() && p.TagName().Name() == "pt" {
						contour = append(contour, curvePoint{int16(num(p, "x")), int16(num(p, "y")), attr(p, "on") == "1"})
					}
				}
				g.contours = append(g.contours, contour)
			}
		}
	}
	return glyphs
}

// within1 tells whether two values differ by at most 1.
func within1(a, b int) bool {
	return a-b >= -1 && a-b <= 1
}

// sameContours tells whether two lists of contours have the same points
// within 1 unit, with each implied on-curve point made explicit. A contour
// can start at another point, when the first point of one contour is
// implied.
func sameContours(a, b [][]curvePoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		pa, pb := explicitPoints(a[i]), explicitPoints(b[i])
		if len(pa) != len(pb) {
			return false
		}
		found := false
		for r := range pb {
			ok := true
			for j := range pa {
				p, q := pa[j], pb[(j+r)%len(pb)]
				// The coordinates are in half units.
				if p[2] != q[2] || p[0]-q[0] < -2 || p[0]-q[0] > 2 || p[1]-q[1] < -2 || p[1]-q[1] > 2 {
					ok = false
					break
				}
			}
			if ok {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// explicitPoints returns the points of a contour with each implied on-curve
// point made explicit, in half units, so that a midpoint is an integer.
// The third value of a point is 1 for an on-curve point.
func explicitPoints(c []curvePoint) [][3]int {
	var out [][3]int
	for i, p := range c {
		on := 0
		if p.onCurve {
			on = 1
		}
		out = append(out, [3]int{2 * int(p.x), 2 * int(p.y), on})
		q := c[(i+1)%len(c)]
		if !p.onCurve && !q.onCurve {
			out = append(out, [3]int{int(p.x) + int(q.x), int(p.y) + int(q.y), 1})
		}
	}
	return out
}

// TestNopFilterSink is nop_filter_sink of read-fonts src/ps/cs.rs. The
// filter drops a move without a line, a line of length 0, and a line back to
// the start before a close.
func TestNopFilterSink(t *testing.T) {
	var pen outlinePath
	nopFilter := nopFilterSink{inner: &pen}
	sx, sy := fixedFromF32(10.2), fixedFromF32(20.4)
	// filtered
	nopFilter.moveTo(fixedFromF32(0.0), fixedFromF32(0.0))
	nopFilter.moveTo(sx, sy)
	// filtered
	nopFilter.lineTo(sx, sy)
	nopFilter.curveTo(fixedFromF32(5.0), fixedFromF32(-5.0), fixedFromF32(1.5), fixedFromF32(2.0), fixedFromF32(4.5), fixedFromF32(-10.0))
	// filtered
	nopFilter.lineTo(fixedFromF32(4.5), fixedFromF32(-10.0))
	// filtered due to next close
	nopFilter.lineTo(sx, sy)
	nopFilter.close()

	pt := func(x, y fixed) kurbo.Point { return pointF32(x.toF32(), y.toF32()) }
	var want bezPath
	want.moveTo(pt(sx, sy))
	// The pen converts the cubic curve to quadratic curves.
	quads := kurbo.NewCubicBez(pt(sx, sy), kurbo.NewPoint(5, -5), kurbo.NewPoint(1.5, 2), kurbo.NewPoint(4.5, -10)).ToQuads(0.025)
	for {
		_, _, q, ok := quads.Next()
		if !ok {
			break
		}
		want.quadTo(pointF32(float32(q.P1.X), float32(q.P1.Y)), pointF32(float32(q.P2.X), float32(q.P2.Y)))
	}
	want.closePath()
	if len(pen.path) != len(want) {
		t.Fatalf("the path has %d elements, want %d: %v", len(pen.path), len(want), pen.path)
	}
	for i := range want {
		if pen.path[i] != want[i] {
			t.Errorf("the element %d is %v, want %v", i, pen.path[i], want[i])
		}
	}
}

// TestUnscaledTransformSinkProducesIntegers is
// unscaled_transform_sink_produces_integers of read-fonts src/ps/cs.rs.
func TestUnscaledTransformSinkProducesIntegers(t *testing.T) {
	var sink transformSink
	for _, coord := range []float32{50.0, 50.1, 50.125, 50.5, 50.9} {
		if x, _ := sink.transform(fixedFromF32(coord), 0); x.toF32() != 50.0 {
			t.Errorf("%v gives %v, want 50", coord, x.toF32())
		}
	}
}

// TestContourToPathFreeType checks the start of a contour in the style of
// FreeType, as the tests of skrifa src/outline/path.rs do: a contour that
// starts off the curve starts at its last point when it is on the curve, and
// at the midpoint of its last and its first point when it is not.
func TestContourToPathFreeType(t *testing.T) {
	pt := func(x, y int32, on bool) flatPoint { return flatPoint{onCurve: on, x: x << 6, y: y << 6} }
	for _, tc := range []struct {
		points []flatPoint
		want   bezPath
	}{
		{
			[]flatPoint{pt(0, 0, false), pt(10, 0, true), pt(10, 10, false), pt(0, 10, true)},
			bezPath{
				kurbo.MoveTo(kurbo.NewPoint(0, 10)),
				kurbo.QuadTo(kurbo.NewPoint(0, 0), kurbo.NewPoint(10, 0)),
				kurbo.QuadTo(kurbo.NewPoint(10, 10), kurbo.NewPoint(0, 10)),
				kurbo.ClosePath(),
			},
		},
		{
			[]flatPoint{pt(0, 0, false), pt(10, 0, false), pt(10, 10, false), pt(1, 10, false)},
			bezPath{
				kurbo.MoveTo(kurbo.NewPoint(0.5, 5)),
				kurbo.QuadTo(kurbo.NewPoint(0, 0), kurbo.NewPoint(5, 0)),
				kurbo.QuadTo(kurbo.NewPoint(10, 0), kurbo.NewPoint(10, 5)),
				kurbo.QuadTo(kurbo.NewPoint(10, 10), kurbo.NewPoint(5.5, 10)),
				kurbo.QuadTo(kurbo.NewPoint(1, 10), kurbo.NewPoint(0.5, 5)),
				kurbo.ClosePath(),
			},
		},
	} {
		var pen outlinePath
		contourToPathFreeType(tc.points, &pen)
		if len(pen.path) != len(tc.want) {
			t.Errorf("the path is %v, want %v", pen.path, tc.want)
			continue
		}
		for i := range tc.want {
			if pen.path[i] != tc.want[i] {
				t.Errorf("the path is %v, want %v", pen.path, tc.want)
				break
			}
		}
	}
}

// TestInterjectCFF2 subsets the CFF2 font of buildCFF2 at the default
// location and at the ends of its axis. The CFF2 table becomes the tables
// glyf and loca, and the blend moves the second point of the glyph 1.
func TestInterjectCFF2(t *testing.T) {
	data := buildCFF2()
	m := NewGlyphRemapperFromGlyphs([]uint16{1})
	for _, tc := range []struct {
		wght float32
		x    int16
	}{{400, 100}, {900, 150}, {100, 50}} {
		out, err := SubsetWithVariations(data, 0, []Variation{{Tag{'w', 'g', 'h', 't'}, tc.wght}}, m)
		if err != nil {
			t.Fatalf("wght %v: %v", tc.wght, err)
		}
		g := readGlyphTables(t, out)
		sg, ok := g.glyf[1].Data.(tables.SimpleGlyph)
		if !ok {
			t.Fatalf("wght %v: the glyph 1 is %T", tc.wght, g.glyf[1].Data)
		}
		var xs []int16
		for _, p := range sg.Points {
			xs = append(xs, p.X)
		}
		if len(xs) < 3 || xs[1] != tc.x {
			t.Errorf("wght %v: the x coordinates are %v, want %d for the second point", tc.wght, xs, tc.x)
		}
		if adv := g.hmtx.Advance(1); adv != 600 {
			t.Errorf("wght %v: the advance is %d, want 600", tc.wght, adv)
		}
	}
}

// buildCFF2 returns a variable font with a CFF2 table, and the tables fvar,
// head, hhea, hmtx and maxp. It has one axis, wght, from 100 to 900, with
// 400 as the default, and two glyphs. The glyph 1 is a triangle with a
// curve, and the x coordinate of its second point is 100 at the default and
// moves by 50 at each end of the axis.
func buildCFF2() []byte {
	// number encodes an integer operand of a char string.
	number := func(v int) []byte {
		if v >= -107 && v <= 107 {
			return []byte{byte(v + 139)}
		}
		return []byte{28, byte(v >> 8), byte(v)}
	}
	var cs []byte
	for _, part := range [][]byte{
		number(0), number(0), {21}, // rmoveto
		// rlineto, with a dx of 100 that a blend moves by 50.
		number(100), number(50), number(-50), number(1), {16}, number(0), {5},
		// rrcurveto
		number(-20), number(50), number(-40), number(50), number(-40), number(0), {8},
	} {
		cs = append(cs, part...)
	}
	// index2 returns a CFF2 INDEX of the items, with offsets of one byte.
	index2 := func(items ...[]byte) []byte {
		out := binary.BigEndian.AppendUint32(nil, uint32(len(items)))
		if len(items) == 0 {
			return out
		}
		out = append(out, 1)
		off := 1
		out = append(out, byte(off))
		for _, it := range items {
			off += len(it)
			out = append(out, byte(off))
		}
		for _, it := range items {
			out = append(out, it...)
		}
		return out
	}
	charStrings := index2([]byte{}, cs)
	// The item variation store, with one region: the whole positive side
	// and then the whole negative side of the axis.
	var vstore []byte
	vstore = binary.BigEndian.AppendUint16(vstore, 1)  // format
	vstore = binary.BigEndian.AppendUint32(vstore, 12) // variationRegionListOffset
	vstore = binary.BigEndian.AppendUint16(vstore, 1)  // itemVariationDataCount
	vstore = binary.BigEndian.AppendUint32(vstore, 28) // itemVariationDataOffsets
	vstore = binary.BigEndian.AppendUint16(vstore, 1)  // axisCount
	vstore = binary.BigEndian.AppendUint16(vstore, 2)  // regionCount
	for _, v := range []uint16{0, 0x4000, 0x4000, 0xC000, 0xC000, 0} {
		vstore = binary.BigEndian.AppendUint16(vstore, v)
	}
	vstore = binary.BigEndian.AppendUint16(vstore, 0) // itemCount
	vstore = binary.BigEndian.AppendUint16(vstore, 0) // wordDeltaCount
	vstore = binary.BigEndian.AppendUint16(vstore, 2) // regionIndexCount
	vstore = binary.BigEndian.AppendUint16(vstore, 0)
	vstore = binary.BigEndian.AppendUint16(vstore, 1)
	vstore = append(binary.BigEndian.AppendUint16(nil, uint16(len(vstore))), vstore...)

	// The layout: header, Top DICT, Global Subr INDEX, CharStrings, Font
	// DICT INDEX, the empty Private DICT, and the variation store. Each
	// offset of the Top DICT is a number of 5 bytes.
	offset := func(v int) []byte { return binary.BigEndian.AppendUint32([]byte{29}, uint32(v)) }
	const topLen = 3*5 + 1 + 2 + 1
	start := 5 + topLen
	gsubrs := index2()
	charStringsAt := start + len(gsubrs)
	fontDict := append(append(number(0), offset(0)...), 18) // Private: size 0, offset 0
	fdArray := index2(fontDict)
	fdArrayAt := charStringsAt + len(charStrings)
	vstoreAt := fdArrayAt + len(fdArray)
	var top []byte
	top = append(append(top, offset(charStringsAt)...), 17)
	top = append(append(top, offset(fdArrayAt)...), 12, 36)
	top = append(append(top, offset(vstoreAt)...), 24)
	if len(top) != topLen {
		panic("the Top DICT does not have the expected length")
	}
	cff2 := []byte{2, 0, 5, 0, byte(len(top))}
	cff2 = append(cff2, top...)
	cff2 = append(cff2, gsubrs...)
	cff2 = append(cff2, charStrings...)
	cff2 = append(cff2, fdArray...)
	cff2 = append(cff2, vstore...)

	head := make([]byte, 54)
	binary.BigEndian.PutUint32(head, 0x00010000)
	binary.BigEndian.PutUint16(head[18:], 1000)
	hhea := make([]byte, 36)
	binary.BigEndian.PutUint32(hhea, 0x00010000)
	binary.BigEndian.PutUint16(hhea[34:], 2)
	hmtx := []byte{0x02, 0x58, 0, 0, 0x02, 0x58, 0, 0}
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint32(maxp, 0x00005000)
	binary.BigEndian.PutUint16(maxp[4:], 2)
	var fvar []byte
	for _, v := range []uint16{1, 0, 16, 2, 1, 20, 0, 0} {
		fvar = binary.BigEndian.AppendUint16(fvar, v)
	}
	fvar = append(fvar, 'w', 'g', 'h', 't')
	for _, v := range []uint32{100 << 16, 400 << 16, 900 << 16} {
		fvar = binary.BigEndian.AppendUint32(fvar, v)
	}
	fvar = binary.BigEndian.AppendUint32(fvar, 0) // flags, axisNameID
	return buildSfnt([]table{
		{tagCFF2, cff2},
		{Tag{'f', 'v', 'a', 'r'}, fvar},
		{tagHEAD, head},
		{tagHHEA, hhea},
		{tagHMTX, hmtx},
		{tagMAXP, maxp},
	})
}

// variableFvar returns an fvar table with one axis, wght, from 100 to 900,
// with 400 as the default.
func variableFvar() []byte {
	var fvar []byte
	for _, v := range []uint16{1, 0, 16, 2, 1, 20, 0, 0} {
		fvar = binary.BigEndian.AppendUint16(fvar, v)
	}
	fvar = append(fvar, 'w', 'g', 'h', 't')
	for _, v := range []uint32{100 << 16, 400 << 16, 900 << 16} {
		fvar = binary.BigEndian.AppendUint32(fvar, v)
	}
	// flags, axisNameID
	return binary.BigEndian.AppendUint32(fvar, 0)
}

// buildVariableTrueType returns a variable TrueType font with the tables
// fvar and gvar, and the tables of buildTrueType. It has one axis, wght,
// from 100 to 900, with 400 as the default, and three glyphs. The glyph 1 is
// a square of 100 units with a quadratic curve, and its points move by 25,
// 50, 75 and 25 units to the right at the end of the axis. The glyph 2 is
// the glyph 1 at the offset (10, 0), which moves by 25 units up.
func buildVariableTrueType() []byte {
	var square []byte
	for _, v := range []uint16{1, 0, 0, 100, 100, 3, 0} {
		square = binary.BigEndian.AppendUint16(square, v)
	}
	// Four points with coordinates of two bytes: (0, 0), (100, 0) off the
	// curve, (100, 100) and (0, 100).
	square = append(square, 0x01, 0x00, 0x01, 0x01)
	for _, v := range []int16{0, 100, 0, -100, 0, 0, 100, 0} {
		square = binary.BigEndian.AppendUint16(square, uint16(v))
	}
	var composite []byte
	for _, v := range []uint16{0xFFFF, 0, 0, 110, 100, 0x0002 | 0x0001, 1} {
		composite = binary.BigEndian.AppendUint16(composite, v)
	}
	composite = binary.BigEndian.AppendUint16(composite, 10)
	composite = binary.BigEndian.AppendUint16(composite, 0)
	font := buildTrueType([][]byte{nil, square, composite})

	// tupleData returns the data of the variations of a glyph: one tuple at
	// the peak 1 of the axis, with the deltas of all the points.
	tupleData := func(dx, dy []int8) []byte {
		var deltas []byte
		deltas = append(deltas, 0) // a point count of 0: all the points
		for _, d := range [][]int8{dx, dy} {
			deltas = append(deltas, byte(len(d)-1))
			for _, v := range d {
				deltas = append(deltas, byte(v))
			}
		}
		var out []byte
		out = binary.BigEndian.AppendUint16(out, 1)   // tupleVariationCount
		out = binary.BigEndian.AppendUint16(out, 4+6) // dataOffset
		out = binary.BigEndian.AppendUint16(out, uint16(len(deltas)))
		out = binary.BigEndian.AppendUint16(out, 0x8000|0x2000) // embedded peak, private points
		out = binary.BigEndian.AppendUint16(out, 0x4000)
		out = append(out, deltas...)
		if len(out)%2 != 0 {
			out = append(out, 0)
		}
		return out
	}
	glyphData := [][]byte{
		nil,
		tupleData([]int8{25, 50, 75, 25, 0, 0, 0, 0}, []int8{0, 0, 0, 0, 0, 0, 0, 0}),
		tupleData([]int8{0, 0, 0, 0, 0}, []int8{25, 0, 0, 0, 0}),
	}
	var gvar []byte
	for _, v := range []uint16{1, 0, 1, 0} {
		gvar = binary.BigEndian.AppendUint16(gvar, v)
	}
	headerLen := 20 + 4*(len(glyphData)+1)
	gvar = binary.BigEndian.AppendUint32(gvar, uint32(headerLen)) // sharedTuplesOffset
	gvar = binary.BigEndian.AppendUint16(gvar, uint16(len(glyphData)))
	gvar = binary.BigEndian.AppendUint16(gvar, 1) // long offsets
	gvar = binary.BigEndian.AppendUint32(gvar, uint32(headerLen))
	at := 0
	for _, d := range glyphData {
		gvar = binary.BigEndian.AppendUint32(gvar, uint32(at))
		at += len(d)
	}
	gvar = binary.BigEndian.AppendUint32(gvar, uint32(at))
	for _, d := range glyphData {
		gvar = append(gvar, d...)
	}

	f, err := parse(font, 0)
	if err != nil {
		panic(err)
	}
	tbls := []table{{Tag{'f', 'v', 'a', 'r'}, variableFvar()}, {Tag{'g', 'v', 'a', 'r'}, gvar}}
	for _, r := range f.records {
		data, _ := f.table(r.tag)
		tbls = append(tbls, table{r.tag, data})
	}
	slices.SortFunc(tbls, func(a, b table) int { return a.tag.compare(b.tag) })
	return buildSfnt(tbls)
}

// TestInterjectTrueType subsets the font of buildVariableTrueType at the
// default location and at the end of its axis. The composite glyph becomes
// a simple glyph, and the deltas move its points.
func TestInterjectTrueType(t *testing.T) {
	data := buildVariableTrueType()
	m := NewGlyphRemapperFromGlyphs([]uint16{1, 2})
	for _, tc := range []struct {
		wght   float32
		square []curvePoint
		moved  []curvePoint
	}{
		{400, nil, nil},
		// The left side bearing of the glyph i of buildTrueType is i, and the
		// xMin of each glyph is 0, so the outline moves i units to the right,
		// as the phantom points of TrueType say.
		{900, []curvePoint{{26, 0, true}, {151, 0, false}, {176, 100, true}, {26, 100, true}},
			[]curvePoint{{37, 25, true}, {162, 25, false}, {187, 125, true}, {37, 125, true}}},
	} {
		out, err := SubsetWithVariations(data, 0, []Variation{{Tag{'w', 'g', 'h', 't'}, tc.wght}}, m)
		if err != nil {
			t.Fatalf("wght %v: %v", tc.wght, err)
		}
		if tc.square == nil {
			want, err := Subset(data, 0, m)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(out, want) {
				t.Errorf("wght %v: the subset is not the subset of Subset", tc.wght)
			}
			continue
		}
		g := readGlyphTables(t, out)
		for gid, want := range [][]curvePoint{nil, tc.square, tc.moved} {
			if want == nil {
				continue
			}
			sg, ok := g.glyf[gid].Data.(tables.SimpleGlyph)
			if !ok {
				t.Fatalf("wght %v: the glyph %d is %T", tc.wght, gid, g.glyf[gid].Data)
			}
			var got []curvePoint
			for _, p := range sg.Points {
				got = append(got, curvePoint{p.X, p.Y, p.Flag&flagOnCurvePoint != 0})
			}
			if !slices.Equal(got, want) {
				t.Errorf("wght %v: the glyph %d has the points %v, want %v", tc.wght, gid, got, want)
			}
		}
	}
}
