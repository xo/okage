package colr

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests the walk of model.go on the test font of COLRv1 of
// fontations, whose item variation store has deltas of 32 bits (D101).

// loadFontations loads testdata/test_glyphs-glyf_colr_1_variable.ttf, the
// font of font-test-data of fontations at the tag skrifa-v0.49.0. It has 44
// axes. The item variation data 2 of its table COLR has the flag
// LONG_WORDS, and the deltas 46875, 48241 and 49151 for the axis ROTA, the
// axis 21. go-text could not parse it, and the font had no table COLR.
func loadFontations(t *testing.T) *font.Font {
	t.Helper()
	data, err := os.ReadFile("testdata/test_glyphs-glyf_colr_1_variable.ttf")
	if err != nil {
		t.Fatal(err)
	}
	ld, err := ot.NewLoader(ot.NewSlicedResource(data))
	if err != nil {
		t.Fatal(err)
	}
	ft, err := font.NewFont(ld)
	if err != nil {
		t.Fatal(err)
	}
	if ft.COLR == nil {
		t.Fatal("the font has no table COLR")
	}
	return ft
}

// paintAll paints each color glyph of ft at coords, and returns the lines
// of the calls, in the format of the Rust program of D100 and D101: a line
// "glyph <id> <ok>" for each color glyph, and the calls of the glyph when
// the paint succeeds.
func paintAll(ft *font.Font, coords []tables.Coord) string {
	var b strings.Builder
	// The font has fewer than 1000 glyphs, and the Rust program also tried
	// the glyphs 0 to 999.
	for g := range 1000 {
		if _, ok := ft.COLR.Search(tables.GlyphID(g)); !ok {
			continue
		}
		p := &recordPainter{}
		ok := PaintModel(ft, coords, GlyphID(g), p)
		fmt.Fprintf(&b, "glyph %d %v\n", g, ok)
		if ok {
			for _, l := range p.lines {
				b.WriteString(l)
				b.WriteByte('\n')
			}
		}
	}
	return b.String()
}

// fontationsCoords returns 44 normalized coordinates: each axis at all, and
// the axis ROTA at rota.
func fontationsCoords(all, rota tables.Coord) []tables.Coord {
	coords := make([]tables.Coord, 44)
	for i := range coords {
		coords[i] = all
	}
	coords[21] = rota
	return coords
}

// TestPaintModelFontations checks the calls of PaintModel for each of the
// 201 color glyphs of the font at five locations. A Rust program read the
// font with read-fonts at the tag skrifa-v0.49.0, called ColorGlyph::paint
// on each color glyph, and printed the calls in the format of paintAll.
// The test holds the SHA-256 of that output. The glyphs 178 and 179 are
// cycles, and both walks fail on them.
func TestPaintModelFontations(t *testing.T) {
	ft := loadFontations(t)
	for _, c := range []struct {
		name   string
		coords []tables.Coord
		sum    string
	}{
		{"the default location", nil, "16b3b27394ebe37374115707794f83693fefb623fea4719f2309bfe01047c40b"},
		{"each axis at 0.5", fontationsCoords(8192, 8192), "0702add4405a6ea08ec54ea7c6f14448e220c43ca11d287e965265aeaae981db"},
		{"each axis at -0.5", fontationsCoords(-8192, -8192), "bdfe449285980bf686d0cf034dbc1cb65e4f30cb1d069fb5910c4f7a15b25880"},
		{"ROTA at 1", fontationsCoords(0, 16384), "158740d68bd9f60a9d817953b2f759ab3279107d5855adb024172e95f5653e26"},
		{"ROTA at 1/3", fontationsCoords(0, 5461), "bad8f1b11be2345aafde6da47f6e54e1e9b81874bebc5b2effd4470aaad5d122"},
	} {
		sum := sha256.Sum256([]byte(paintAll(ft, c.coords)))
		if got := hex.EncodeToString(sum[:]); got != c.sum {
			t.Errorf("%s: the SHA-256 of the calls is %s, want %s", c.name, got, c.sum)
		}
	}
}

// TestPaintModelLongWords checks the calls of the glyphs 99 to 102, which
// rotate by an angle with a delta of 32 bits, with the axis ROTA at 1. The
// lines are the ones that read-fonts printed.
func TestPaintModelLongWords(t *testing.T) {
	ft := loadFontations(t)
	coords := fontationsCoords(0, 16384)
	for _, c := range []struct {
		glyph     GlyphID
		transform string
	}{
		{99, "pt bf800000 3948f999 b948f999 bf800000 00000000 00000000"},
		{100, "pt bf7c1aa8 3e31f777 be31f777 bf7c1aa8 4506e93d 44e25f8f"},
		{101, "pt bf800000 3948f999 b948f999 bf800000 447a0622 4479f9de"},
		{102, "pt bf7744bd 3e849429 be849429 bf7744bd 448b0d5e 44555e64"},
	} {
		want := []string{
			"layer 3",
			"fillg 3 - solid 4 3f000000",
			"layer 4",
			c.transform,
			"fillg 3 - solid 1 3f333400",
			"popt",
			"popl",
			"popl",
		}
		p := &recordPainter{}
		ok := PaintModel(ft, coords, c.glyph, p)
		if !ok || strings.Join(p.lines, "\n") != strings.Join(want, "\n") {
			t.Errorf("glyph %d: PaintModel is %v with\n%s\nwant true with\n%s", c.glyph, ok, strings.Join(p.lines, "\n"), strings.Join(want, "\n"))
		}
	}
}
