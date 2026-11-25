// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package harfbuzz

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// okage: the tests of D93. test/harfbuzz compares the flag
// unsafe-to-concat with HarfBuzz on more texts.

// concatFlags shapes the text in the face with the buffer flag
// ProduceUnsafeToConcat, and gives the flags unsafe-to-break (1) and
// unsafe-to-concat (2) of each glyph.
func concatFlags(t *testing.T, face *font.Face, text string) []GlyphMask {
	t.Helper()
	buf := NewBuffer()
	buf.AddRunes([]rune(text), 0, -1)
	buf.GuessSegmentProperties()
	buf.Flags |= ProduceUnsafeToConcat
	buf.Shape(NewFont(face), nil)
	out := make([]GlyphMask, len(buf.Info))
	for i, info := range buf.Info {
		out[i] = info.Mask & (GlyphUnsafeToBreak | GlyphUnsafeToConcat)
	}
	return out
}

// TestUnsafeToConcatFlags tests the flags of texts where go-text marked
// other glyphs unsafe to concat than HarfBuzz 15.1.0. The flags are the
// flags of HarfBuzz.
//
//   - Roboto "fi": the ligature set of f holds ffi and fi. On its fast
//     path, HarfBuzz skips ffi, whose second glyph is not i, and marks no
//     glyph after the ligature fi. go-text tried ffi, and marked f and i.
//   - NotoSansDuployan "= ": a reverse chaining lookup whose context does
//     not match marks the glyphs that it read, up to the space.
//   - NotoNastaliqUrdu: a chained context rule whose input does not match
//     the question mark marks no glyph.
func TestUnsafeToConcatFlags(t *testing.T) {
	data, err := os.ReadFile("../font/testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	roboto, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		face *font.Face
		text string
		want []GlyphMask
	}{
		{roboto, "fi", []GlyphMask{0}},
		{roboto, "fi ", []GlyphMask{0, 2}},
		{roboto, "fia", []GlyphMask{0, 2}},
	} {
		if got := concatFlags(t, tt.face, tt.text); !slices.Equal(got, tt.want) {
			t.Errorf("%q: got the flags %v, want %v", tt.text, got, tt.want)
		}
	}

	duployan := font.NewFace(openFontFile(t, "perf_reference/fonts/NotoSansDuployan-Regular.otf"))
	if got := concatFlags(t, duployan, "= "); got[len(got)-1] != GlyphUnsafeToConcat {
		t.Errorf("\"= \": got the flags %v for the space, want 2", got[len(got)-1])
	}
	nastaliq := font.NewFace(openFontFile(t, "perf_reference/fonts/NotoNastaliqUrdu-Regular.ttf"))
	if got, want := concatFlags(t, nastaliq, "مگر؟"), []GlyphMask{0, 3, 3, 2, 2}; !slices.Equal(got, want) {
		t.Errorf("مگر؟: got the flags %v, want %v", got, want)
	}
}
