package colr

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests the walk of read-fonts 0.46 of model.go (D100).

// recordPainter is a Painter that writes each call as one line. A float32
// is written as the hex of its bits, so that a line gives the exact value.
// The Rust program of D100 writes the calls of read-fonts in the same way.
type recordPainter struct {
	lines []string
}

func hexF32(v float32) string { return fmt.Sprintf("%08x", math.Float32bits(v)) }

func transformString(t Transform) string {
	return strings.Join([]string{hexF32(t.XX), hexF32(t.YX), hexF32(t.XY), hexF32(t.YY), hexF32(t.DX), hexF32(t.DY)}, " ")
}

func stopsString(stops []ColorStop) string {
	out := make([]string, len(stops))
	for i, s := range stops {
		out[i] = fmt.Sprintf("%s:%d:%s", hexF32(s.Offset), s.PaletteIndex, hexF32(s.Alpha))
	}
	return strings.Join(out, ",")
}

func brushString(b Brush) string {
	switch b.Kind {
	case BrushSolid:
		return fmt.Sprintf("solid %d %s", b.PaletteIndex, hexF32(b.Alpha))
	case BrushLinearGradient:
		return fmt.Sprintf("lin %s %s %s %s %d %s", hexF32(b.P0.X), hexF32(b.P0.Y), hexF32(b.P1.X), hexF32(b.P1.Y), b.Extend, stopsString(b.ColorStops))
	case BrushRadialGradient:
		return fmt.Sprintf("rad %s %s %s %s %s %s %d %s", hexF32(b.C0.X), hexF32(b.C0.Y), hexF32(b.R0), hexF32(b.C1.X), hexF32(b.C1.Y), hexF32(b.R1), b.Extend, stopsString(b.ColorStops))
	}
	return fmt.Sprintf("sweep %s %s %s %s %d %s", hexF32(b.C0.X), hexF32(b.C0.Y), hexF32(b.StartAngle), hexF32(b.EndAngle), b.Extend, stopsString(b.ColorStops))
}

func (p *recordPainter) addf(format string, args ...any) {
	p.lines = append(p.lines, fmt.Sprintf(format, args...))
}

func (p *recordPainter) PushTransform(t Transform) { p.addf("pt %s", transformString(t)) }
func (p *recordPainter) PopTransform()             { p.addf("popt") }
func (p *recordPainter) FillGlyph(g GlyphID, t *Transform, b Brush) {
	ts := "-"
	if t != nil {
		ts = transformString(*t)
	}
	p.addf("fillg %d %s %s", g, ts, brushString(b))
}
func (p *recordPainter) PushClipGlyph(g GlyphID) { p.addf("clipg %d", g) }
func (p *recordPainter) PushClipBox(b BoundingBox) {
	p.addf("clipb %s %s %s %s", hexF32(b.XMin), hexF32(b.YMin), hexF32(b.XMax), hexF32(b.YMax))
}
func (p *recordPainter) PopClip()                         { p.addf("popc") }
func (p *recordPainter) Fill(b Brush)                     { p.addf("fill %s", brushString(b)) }
func (p *recordPainter) PushLayer(m tables.CompositeMode) { p.addf("layer %d", m) }
func (p *recordPainter) PopLayer()                        { p.addf("popl") }

// walkCOLR is the table COLR that the script of D100 builds with fontTools
// for SourceSans-VF.ttf, whose one axis is wght:
//
//   - The glyph 1 is a PaintGlyph of the glyph 3, of a PaintTranslate by 10
//     and 20, of a PaintColrLayers of two PaintSolid.
//   - The glyph 2 is a PaintColrGlyph of itself, a cycle.
//   - The glyph 3 is a PaintGlyph of the glyph 1, of a PaintVarSolid with
//     the alpha 0.5 and the VarIndexBase 0x10000. The table has no
//     DeltaSetIndexMap, so the index names the delta set 0 of the item
//     variation data 1, the delta -3001.
const walkCOLR = "0001000000000000000000000000000000220000005e0000000000000000000000" +
	"740000000300010000001600020000002a00030000002d0a00000600030e000008" +
	"000a00140102000000000b00020a0000060001030001200000010000000000020000" +
	"000c000000110200034000020004400000010000001000020000001a000000240001" +
	"0001000040004000000100010001000003e80002000100010000f4470007"

// TestPaintModelReadFonts checks the calls that PaintModel makes on
// walkCOLR at the coordinate 8193 of wght. The lines are the ones that the
// Rust program of D100 printed for ColorGlyph::paint of read-fonts 0.46.
// The walk of skrifa 0.44 of colr.go, which TestPaintSkrifa checks, gives
// other calls for the glyphs 1 and 3.
func TestPaintModelReadFonts(t *testing.T) {
	data, err := hex.DecodeString(walkCOLR)
	if err != nil {
		t.Fatal(err)
	}
	colr, err := tables.ParseCOLR(data)
	if err != nil {
		t.Fatal(err)
	}
	ft := &font.Font{COLR: &colr}
	coords := []tables.Coord{8193}
	for _, c := range []struct {
		glyph GlyphID
		ok    bool
		want  []string
	}{
		// The chain of transforms does not end in a brush, so the glyph is a
		// clip and the transform is pushed, where skrifa 0.44 fills each
		// layer with FillGlyph.
		{1, true, []string{
			"clipg 3",
			"pt 3f800000 00000000 00000000 3f800000 41200000 41a00000",
			"fill solid 3 3f800000",
			"fill solid 4 3f800000",
			"popt",
			"popc",
		}},
		// The cycle is found on the first PaintColrGlyph.
		{2, false, nil},
		// The delta of the outer index 1 gives the alpha 0.40840578. skrifa
		// 0.44 takes the outer index 0, whose delta is 1000.
		{3, true, []string{"fillg 1 - solid 1 3ed11a89"}},
	} {
		p := &recordPainter{}
		ok := PaintModel(ft, coords, c.glyph, p)
		if ok != c.ok || strings.Join(p.lines, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("glyph %d: PaintModel is %v with\n%s\nwant %v with\n%s", c.glyph, ok, strings.Join(p.lines, "\n"), c.ok, strings.Join(c.want, "\n"))
		}
	}
}

// TestModelDecycler checks that the decycler of read-fonts compares a node
// with the node at half the depth only, so a cycle is found once it
// repeats, and a node that comes back at another depth is not a cycle.
func TestModelDecycler(t *testing.T) {
	var d modelDecycler
	for i, id := range []uint64{1, 2, 1, 3} {
		if !d.enter(id) {
			t.Fatalf("enter %d at the depth %d is false", id, i)
		}
	}
	// The node at half the depth 4 is the node 1, at the depth 2.
	if d.enter(1) {
		t.Error("enter 1 at the depth 4 is true, want a cycle")
	}
	for range 4 {
		d.leave()
	}
	for i := range maxTraversalDepth {
		if !d.enter(uint64(i)) {
			t.Fatalf("enter at the depth %d is false", i)
		}
	}
	if d.enter(1000) {
		t.Error("enter past the largest depth is true")
	}
}

// TestPaintSkrifa checks that Paint, which pdf calls, keeps the walk of
// skrifa 0.44 on walkCOLR, where it differs from read-fonts 0.46.
func TestPaintSkrifa(t *testing.T) {
	data, err := hex.DecodeString(walkCOLR)
	if err != nil {
		t.Fatal(err)
	}
	colr, err := tables.ParseCOLR(data)
	if err != nil {
		t.Fatal(err)
	}
	ft := &font.Font{COLR: &colr}
	coords := []tables.Coord{8193}
	for _, c := range []struct {
		glyph GlyphID
		ok    bool
		want  []string
	}{
		{1, true, []string{
			"fillg 3 3f800000 00000000 00000000 3f800000 41200000 41a00000 solid 3 3f800000",
			"fillg 3 3f800000 00000000 00000000 3f800000 41200000 41a00000 solid 4 3f800000",
		}},
		{2, false, nil},
		{3, true, []string{"fillg 1 - solid 1 3f07d03e"}},
	} {
		p := &recordPainter{}
		ok := Paint(ft, coords, c.glyph, p)
		if ok != c.ok || strings.Join(p.lines, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("glyph %d: Paint is %v with\n%s\nwant %v with\n%s", c.glyph, ok, strings.Join(p.lines, "\n"), c.ok, strings.Join(c.want, "\n"))
		}
	}
}
