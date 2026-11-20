// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It tests the deltas of gvar in 16.16
// of D84.

import (
	"testing"

	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
	td "github.com/xo/okage/typesetting/internal/testfonts/opentype"
)

// TestInferDeltaFixed is the test interpolate of skrifa
// src/outline/glyf/deltas.rs, which takes the example of inferred deltas of
// the gvar chapter of the OpenType specification. The point (260, 700) lies
// between the points (245, 630) and (305, 680), whose deltas are (28, -62)
// and (-42, -57).
func TestInferDeltaFixed(t *testing.T) {
	fx := func(i int32) tables.Fixed { return tables.FixedFromInt(i) }
	// 10.4999237060547 in the 16.16 format, as skrifa gives it.
	if got := inferDelta(260, 245, 305, fx(28), fx(-42)); got != 688123 {
		t.Errorf("the delta x is %d, want 688123", got)
	}
	if got := inferDelta(700, 630, 680, fx(-62), fx(-57)); got != fx(-57) {
		t.Errorf("the delta y is %d, want %d", got, fx(-57))
	}
	// Two reference points at the same coordinate with other deltas give
	// no delta.
	if got := inferDelta(10, 20, 20, fx(1), fx(2)); got != 0 {
		t.Errorf("the delta is %d, want 0", got)
	}
}

// TestGlyphVariationDeltas checks that the deltas of GlyphVariationDeltas
// are the deltas that move the points of the outline. It skips when
// typesetting/testdata is not there (D76).
func TestGlyphVariationDeltas(t *testing.T) {
	data, err := td.Files.ReadFile("common/Selawik-VF.ttf")
	if err != nil {
		t.Skip(err)
	}
	ld, err := ot.NewLoader(ot.NewSlicedResource(data))
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewFont(ld)
	if err != nil {
		t.Fatal(err)
	}
	face := NewFace(f)
	for gid := range GID(len(f.glyf)) {
		if face.GlyphVariationDeltas(gid) != nil {
			t.Fatalf("the glyph %d has deltas at the default location", gid)
		}
	}
	face.SetVariations([]Variation{{Tag: ot.MustNewTag("wght"), Value: 650}})
	n := 0
	for gid := range GID(len(f.glyf)) {
		sg, ok := f.glyf[gid].Data.(tables.SimpleGlyph)
		if !ok {
			continue
		}
		deltas := face.GlyphVariationDeltas(gid)
		if len(deltas) != len(sg.Points)+phantomCount {
			t.Fatalf("the glyph %d has %d deltas, want %d", gid, len(deltas), len(sg.Points)+phantomCount)
		}
		points := face.getPointsForGlyph(tables.GlyphID(gid))
		left := deltas[len(sg.Points)+phantomLeft]
		for i, p := range sg.Points {
			// getPointsForGlyph shifts the points by the first phantom
			// point.
			d := deltas[i]
			x := float64(p.X) + float64(d[0]-left[0])/(1<<16) - float64(f.glyf[gid].XMin-f.hmtx.SideBearing(tables.GlyphID(gid)))
			y := float64(p.Y) + float64(d[1])/(1<<16)
			if dx, dy := float64(points[i].X)-x, float64(points[i].Y)-y; dx > 1e-3 || dx < -1e-3 || dy > 1e-3 || dy < -1e-3 {
				t.Fatalf("the point %d of the glyph %d is (%g, %g), and the deltas give (%g, %g)", i, gid, points[i].X, points[i].Y, x, y)
			}
		}
		for _, d := range deltas {
			if d != [2]tables.Fixed{} {
				n++
			}
		}
	}
	if n == 0 {
		t.Fatal("no point has a delta")
	}
}
