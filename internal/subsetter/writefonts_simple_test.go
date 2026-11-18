// Copyright 2019 Colin Rothfels
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"slices"
	"testing"

	"github.com/xo/okage/kurbo"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// The tests of write-fonts src/tables/glyf/simple.rs for one path. The tests
// of interpolatable glyphs of many paths, and the tests that read the test
// fonts of write-fonts, are not ported, because the port takes one path.
// Upstream reads the glyph back with read-fonts, and the port reads it with
// typesetting.

// pt is a point of kurbo.
func pt(x, y float64) kurbo.Point {
	return kurbo.NewPoint(x, y)
}

// dumpAndRead writes the glyph of a path, and reads it back.
func dumpAndRead(t *testing.T, path bezPath) tables.SimpleGlyph {
	t.Helper()
	glyph, malformed := simpleGlyphFromBezPath(path)
	if malformed != malformedPathNone {
		t.Fatalf("the path is malformed: %d", malformed)
	}
	data, ok := glyph.dump()
	if !ok {
		t.Fatal("the glyph cannot be written")
	}
	g, _, err := tables.ParseGlyph(data)
	if err != nil {
		t.Fatal(err)
	}
	sg, ok := g.Data.(tables.SimpleGlyph)
	if !ok {
		t.Fatalf("the glyph is %T", g.Data)
	}
	return sg
}

// checkPoints compares the points of a glyph with x, y and the flag
// on-curve of each point.
func checkPoints(t *testing.T, sg tables.SimpleGlyph, ends []uint16, want []curvePoint) {
	t.Helper()
	if !slices.Equal(sg.EndPtsOfContours, ends) {
		t.Errorf("the end points are %v, want %v", sg.EndPtsOfContours, ends)
	}
	var got []curvePoint
	for _, p := range sg.Points {
		got = append(got, curvePoint{p.X, p.Y, p.Flag&flagOnCurvePoint != 0})
	}
	if !slices.Equal(got, want) {
		t.Errorf("the points are %v, want %v", got, want)
	}
}

// TestBadPathInput is bad_path_input.
func TestBadPathInput(t *testing.T) {
	var path bezPath
	path.moveTo(pt(0., 0.))
	path = append(path, kurbo.CurveTo(pt(10., 10.), pt(20., 20.), pt(30., 30.)))
	path.lineTo(pt(50., 50.))
	path.lineTo(pt(10., 10.))
	if _, malformed := simpleGlyphFromBezPath(path); malformed != malformedPathHasCubic {
		t.Errorf("the path gives %d, want malformedPathHasCubic", malformed)
	}
	if _, malformed := simpleGlyphFromBezPath(bezPath{kurbo.LineTo(pt(1, 1))}); malformed != malformedPathMissingMove {
		t.Errorf("a path without a move gives %d, want malformedPathMissingMove", malformed)
	}
}

// TestSimpleGlyphOpenPath is simple_glyph_open_path.
func TestSimpleGlyphOpenPath(t *testing.T) {
	var path bezPath
	path.moveTo(pt(20., -100.))
	path.quadTo(pt(1337., 1338.), pt(-50., -69.0))
	path.quadTo(pt(13., 255.), pt(-255., 256.))
	// even if the last point is on top of the first, the path was not
	// deliberately closed hence there is going to be an extra point (6, not
	// 5 in total)
	path.lineTo(pt(20., -100.))

	checkPoints(t, dumpAndRead(t, path), []uint16{5}, []curvePoint{
		{20, -100, true}, {1337, 1338, false}, {-50, -69, true},
		{13, 255, false}, {-255, 256, true}, {20, -100, true},
	})
}

// TestSimpleGlyphClosedPathImplicitVsExplicitClosingLine is
// simple_glyph_closed_path_implicit_vs_explicit_closing_line.
func TestSimpleGlyphClosedPathImplicitVsExplicitClosingLine(t *testing.T) {
	var path1 bezPath
	path1.moveTo(pt(20., -100.))
	path1.quadTo(pt(1337., 1338.), pt(-50., -69.0))
	path1.quadTo(pt(13., 255.), pt(-255., 256.))
	path1.closePath()

	var path2 bezPath
	path2.moveTo(pt(20., -100.))
	path2.quadTo(pt(1337., 1338.), pt(-50., -69.0))
	path2.quadTo(pt(13., 255.), pt(-255., 256.))
	// this line_to (absent from path1) makes no difference since in both
	// cases the path is closed with a close_path (5 points in total, not 6)
	path2.lineTo(pt(20., -100.))
	path2.closePath()

	for _, path := range []bezPath{path1, path2} {
		checkPoints(t, dumpAndRead(t, path), []uint16{4}, []curvePoint{
			{20, -100, true}, {1337, 1338, false}, {-50, -69, true},
			{13, 255, false}, {-255, 256, true},
		})
	}
}

// TestKeepSinglePointContours is keep_single_point_contours.
func TestKeepSinglePointContours(t *testing.T) {
	// single points may be meaningless, but are also harmless
	var path bezPath
	path.moveTo(pt(0.0, 0.0))
	// path.close_path();  // doesn't really matter if this is closed
	path.moveTo(pt(1.0, 2.0))
	path.closePath()

	checkPoints(t, dumpAndRead(t, path), []uint16{0, 1}, []curvePoint{{0, 0, true}, {1, 2, true}})
}

// TestCompileRepeatableFlags is compile_repeatable_flags.
func TestCompileRepeatableFlags(t *testing.T) {
	var path bezPath
	path.moveTo(pt(20., -100.))
	path.lineTo(pt(25., -90.))
	path.lineTo(pt(50., -69.))
	path.lineTo(pt(80., -20.))

	glyph, _ := simpleGlyphFromBezPath(path)
	var flags []uint8
	for _, d := range glyph.computePointDeltas() {
		flags = append(flags, d.flag)
	}
	if r := repeatableFlagsFromFlags(flags); len(r) != 2 {
		t.Errorf("the flags are %v, want 2 flags", r)
	}
	checkPoints(t, dumpAndRead(t, path), []uint16{3}, []curvePoint{
		{20, -100, true}, {25, -90, true}, {50, -69, true}, {80, -20, true},
	})
}

// TestSimpleGlyphsFromKurboKeepOnCurveUnlessImpliable is the part for one
// path of simple_glyphs_from_kurbo_keep_on_curve_unless_impliable_for_all.
func TestSimpleGlyphsFromKurboKeepOnCurveUnlessImpliable(t *testing.T) {
	var path bezPath
	path.moveTo(pt(0.0, 0.0))
	path.quadTo(pt(0.0, 1.0), pt(1.0, 1.0)) // on-curve equidistant from prev/next off-curves
	path.quadTo(pt(2.0, 1.0), pt(2.0, 0.0))
	path.lineTo(pt(0.0, 0.0))
	path.closePath()

	// when making a SimpleGlyph from this path alone, the on-curve point at
	// (1, 1) can be implied/dropped.
	glyph, _ := simpleGlyphFromBezPath(path)
	want := [][]curvePoint{{{0, 0, true}, {0, 1, false}, {2, 1, false}, {2, 0, true}}}
	if !slices.EqualFunc(glyph.contours, want, slices.Equal) {
		t.Errorf("the contours are %v, want %v", glyph.contours, want)
	}
}

// TestSimpleGlyphFromKurboEquidistantButNotCollinearPoints is
// simple_glyph_from_kurbo_equidistant_but_not_collinear_points.
func TestSimpleGlyphFromKurboEquidistantButNotCollinearPoints(t *testing.T) {
	var path bezPath
	path.moveTo(pt(0.0, 0.0))
	path.quadTo(pt(2.0, 2.0), pt(4.0, 3.0))
	path.quadTo(pt(6.0, 2.0), pt(8.0, 0.0))
	path.closePath()

	glyph, _ := simpleGlyphFromBezPath(path)
	want := [][]curvePoint{{
		{0, 0, true},
		{2, 2, false},
		// the following on-curve point is equidistant from the
		// previous/next off-curve points but it is not on the same line
		// hence it must NOT be dropped
		{4, 3, true},
		{6, 2, false},
		{8, 0, true},
	}}
	if !slices.EqualFunc(glyph.contours, want, slices.Equal) {
		t.Errorf("the contours are %v, want %v", glyph.contours, want)
	}
}

// TestRepeatableFlagsBasic is repeatable_flags_basic.
func TestRepeatableFlagsBasic(t *testing.T) {
	flags := []uint8{flagOnCurvePoint, flagXShortVector, flagXShortVector}
	repeatable := repeatableFlagsFromFlags(flags)
	// even though we have a repeating flag at the end, we should still
	// produce three flags, since we don't bother with repeat counts < 2.
	want := []repeatableFlag{{flagOnCurvePoint, 0}, {flagXShortVector, 0}, {flagXShortVector, 0}}
	if !slices.Equal(repeatable, want) {
		t.Errorf("the flags are %v, want %v", repeatable, want)
	}
}

// TestRepeatableFlagsRepeats is repeatable_flags_repeats.
func TestRepeatableFlagsRepeats(t *testing.T) {
	var flags []uint8
	for range 4 {
		flags = append(flags, flagOnCurvePoint)
	}
	for range 257 {
		flags = append(flags, flagYShortVector)
	}
	repeatable := repeatableFlagsFromFlags(flags)
	want := []repeatableFlag{
		{flagOnCurvePoint | flagRepeatFlag, 3},
		{flagYShortVector | flagRepeatFlag, 255},
		{flagYShortVector, 0},
	}
	if !slices.Equal(repeatable, want) {
		t.Errorf("the flags are %v, want %v", repeatable, want)
	}
}

// TestMidPoints is mid_points.
func TestMidPoints(t *testing.T) {
	for _, tc := range []struct {
		p0, p1, p2 kurbo.Point
		want       bool
	}{
		// exactly in the middle
		{pt(0.0, 0.0), pt(1.0, 1.0), pt(2.0, 2.0), true},
		// in the middle but rounding would make it not; take it anyway
		{pt(0.5, 0.5), pt(3.0, 3.0), pt(5.5, 5.5), true},
		// very close to the middle
		{pt(0.0, 0.0), pt(1.00001, 0.99999), pt(2.0, 2.0), true},
		// not quite in the middle but rounding would make it so; why throw
		// it away?
		{pt(0.0, 0.0), pt(-1.499999, 0.500001), pt(-2.0, 2.0), true},
		// not in the middle, neither before nor after rounding
		{pt(0.0, 0.0), pt(1.0, 1.5), pt(2.0, 2.0), false},
	} {
		if got := isMidPoint(tc.p0, tc.p1, tc.p2); got != tc.want {
			t.Errorf("isMidPoint(%v, %v, %v) = %v, want %v", tc.p0, tc.p1, tc.p2, got, tc.want)
		}
	}
}
