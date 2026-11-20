// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"os"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// okage: the tests of D73.

// shapedGlyph is a glyph of the output of HarfBuzz 15.1.0, at the scale of
// the units of the font.
type shapedGlyph struct {
	glyph            GID
	cluster          int
	advance          Position
	offsetX, offsetY Position
}

// shapedText is a text and the glyphs that HarfBuzz 15.1.0 gives for it.
type shapedText struct {
	text string
	want []shapedGlyph
}

// testShapedGlyphs shapes each text in the font of the file, and compares
// the glyphs with the glyphs of HarfBuzz.
func testShapedGlyphs(t *testing.T, path string, texts []shapedText) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	face, err := font.ParseTTF(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range texts {
		buf := NewBuffer()
		buf.AddRunes([]rune(test.text), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(NewFont(face), nil)
		if len(buf.Info) != len(test.want) {
			t.Errorf("%q: got %d glyphs, want %d", test.text, len(buf.Info), len(test.want))
			continue
		}
		for i, want := range test.want {
			info, pos := buf.Info[i], buf.Pos[i]
			got := shapedGlyph{info.Glyph, info.Cluster, pos.XAdvance, pos.XOffset, pos.YOffset}
			if got != want {
				t.Errorf("%q, glyph %d: got %+v, want %+v", test.text, i, got, want)
			}
		}
	}
}

// TestMarkBaseOfLookup shapes two short texts in Amiri. In each text, GSUB
// adds a tatweel after the lam. One lookup of GPOS attaches the mark to
// the tatweel, and a later lookup attaches it to the lam. go-text kept the
// base of the first lookup for the second, which then did not attach the
// mark.
func TestMarkBaseOfLookup(t *testing.T) {
	testShapedGlyphs(t, "../font/testdata/Amiri-Regular.ttf", []shapedText{
		{"لًكنننننن", []shapedGlyph{
			{3982, 8, 542, 0, 0}, {3964, 7, 218, 0, 0}, {2140, 6, 244, 0, 0}, {2140, 5, 244, 0, 0},
			{4511, 4, 244, 0, 0}, {2140, 3, 244, 0, 0}, {2307, 2, 588, 0, 0}, {427, 0, 0, -34, 0},
			{5992, 0, 185, 0, 0}, {2329, 0, 175, 0, 0},
		}},
		{"هلَكو", []shapedGlyph{
			{2448, 4, 345, 0, 0}, {2307, 3, 588, 0, 0}, {430, 1, 0, -23, 0}, {5992, 1, 185, 0, 0},
			{2335, 1, 197, 0, 0}, {2263, 0, 452, 0, 0},
		}},
	})
}

// TestMarkWithoutRecord shapes a text in NotoSans-Regular_var.ttf, the
// same file as NotoSans_variable.ttf of krilla. Its lookup 35, a
// MarkLigPos, covers 287 marks with 281 mark records, and six records have
// a null anchor offset. U+0334 has such a record, and U+1ACB has no record.
// go-text dropped the whole table GPOS, so the text had no kerning.
func TestMarkWithoutRecord(t *testing.T) {
	testShapedGlyphs(t, "../../internal/subsetter/testdata/NotoSans-Regular_var.ttf", []shapedText{
		{"fi\u0334 fl\u1acb AV", []shapedGlyph{
			{1654, 0, 602, 0, 0}, {2712, 0, 0, -129, 0}, {3, 3, 260, 0, 0}, {1655, 4, 602, 0, 0},
			{3767, 4, 0, -130, 765}, {3, 7, 260, 0, 0}, {36, 8, 599, 0, 0}, {57, 9, 600, 0, 0},
		}},
	})
}
