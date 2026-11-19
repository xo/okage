// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"fmt"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/language"
	"golang.org/x/image/math/fixed"
)

// okage: the tests of D70.

// monoText is the index of the text of unsafeTexts in a monospace font.
const monoText = 1

// unsafeTexts are the texts of TestUnsafeToBreak: a font, a direction and
// a text, with the script and the language of the text.
func unsafeTexts(t *testing.T) []Input {
	t.Helper()
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(t, "../font/testdata/Amiri-Regular.ttf")
	mono := loadOpentypeFont(t, "../font/testdata/UbuntuMono-R.ttf")
	latin := []rune("AVA Wave, To. Tea office ffl fi \"Yes,\" LT. " + benchParagraphLatin[:240])
	arabic := []rune(benchParagraphArabic)[:220]
	in := func(face *font.Face, text []rune, dir di.Direction, script language.Script, lang string) Input {
		return Input{
			Text: text, RunEnd: len(text), Direction: dir, Face: face, Size: 16 * 64,
			Script: script, Language: language.NewLanguage(lang),
		}
	}
	return []Input{
		in(roboto, latin, di.DirectionLTR, language.Latin, "en"),
		in(mono, latin, di.DirectionLTR, language.Latin, "en"),
		in(amiri, latin, di.DirectionLTR, language.Latin, "en"),
		in(amiri, arabic, di.DirectionRTL, language.Arabic, "ar"),
	}
}

// TestUnsafeToBreak tests the flags of the texts of unsafeTexts with
// checkUnsafeToBreak.
func TestUnsafeToBreak(t *testing.T) {
	var shaper HarfbuzzShaper
	for i, input := range unsafeTexts(t) {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			safe, unsafe, err := checkUnsafeToBreak(&shaper, input)
			if err != nil {
				t.Fatal(err)
			}
			// The monospace font has no kerning, so only the other fonts
			// have clusters that are unsafe to break.
			if safe == 0 || (unsafe == 0 && i != monoText) {
				t.Fatalf("got %d clusters safe to break and %d unsafe, want some of each", safe, unsafe)
			}
		})
	}
}

// FuzzUnsafeToBreak tests the flags of a text with checkUnsafeToBreak, in
// a Latin font and in an Arabic font. Each byte of the input picks a
// character of an alphabet that the font has. HarfBuzz gives other glyphs
// for a character that the font does not have, such as U+01EC, when a mark
// is elsewhere in the text, and it does not mark such a cluster as unsafe
// to break. The alphabets leave such characters out. The Arabic alphabet
// also leaves out the marks, such as U+064E, because okage does not place
// a mark on some ligatures as HarfBuzz does, as "typesetting" in
// docs/BACKLOG.md says.
func FuzzUnsafeToBreak(f *testing.F) {
	roboto := loadOpentypeFont(f, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(f, "../font/testdata/Amiri-Regular.ttf")
	latin := []rune("AVWTYLPFKkoaecfilty0147.,;:'\" -()/")
	arabic := []rune("ابتجحدرسعفقكلمنهويةءأإـ،؟ ١٢")
	f.Add([]byte("AVA Wave, To. office"), false)
	f.Add([]byte("bismillah ar-rahman"), true)
	f.Fuzz(func(t *testing.T, data []byte, isArabic bool) {
		alphabet := latin
		input := Input{
			Direction: di.DirectionLTR, Face: roboto, Size: 16 * 64,
			Script: language.Latin, Language: language.NewLanguage("en"),
		}
		if isArabic {
			alphabet = arabic
			input.Direction, input.Face = di.DirectionRTL, amiri
			input.Script, input.Language = language.Arabic, language.NewLanguage("ar")
		}
		runes := make([]rune, 0, min(len(data), 64))
		for _, b := range data[:min(len(data), 64)] {
			runes = append(runes, alphabet[int(b)%len(alphabet)])
		}
		input.Text, input.RunEnd = runes, len(runes)
		var shaper HarfbuzzShaper
		if _, _, err := checkUnsafeToBreak(&shaper, input); err != nil {
			t.Fatalf("%q: %v", string(runes), err)
		}
	})
}

// checkUnsafeToBreak shapes the input whole, and then in two parts at the
// start of each cluster whose glyphs do not have the flag unsafe to break.
// The glyphs of the two parts must be the glyphs of the whole text, as the
// option --verify of hb-shape of HarfBuzz does. Each part is shaped with
// the rest of the text as its context. checkUnsafeToBreak returns the
// number of clusters that are safe to break and the number that are not.
func checkUnsafeToBreak(shaper *HarfbuzzShaper, input Input) (safe, unsafe int, err error) {
	whole := shaper.Shape(input)
	for j, g := range whole.Glyphs {
		if j > 0 && whole.Glyphs[j-1].ClusterIndex == g.ClusterIndex {
			continue
		}
		r := g.ClusterIndex
		if r == input.RunStart {
			continue
		}
		if g.UnsafeToBreak() {
			unsafe++
			continue
		}
		safe++
		first, second := input, input
		first.RunEnd, second.RunStart = r, r
		a, b := shaper.Shape(first), shaper.Shape(second)
		if err := sameGlyphs(whole.Glyphs, a.Glyphs, input.RunStart, r); err != nil {
			return safe, unsafe, fmt.Errorf("break at rune %d, first part: %w", r, err)
		}
		if err := sameGlyphs(whole.Glyphs, b.Glyphs, r, input.RunEnd); err != nil {
			return safe, unsafe, fmt.Errorf("break at rune %d, second part: %w", r, err)
		}
	}
	return safe, unsafe, nil
}

// sameGlyphs returns an error when the glyphs of whole whose cluster is in
// [start, end) differ from part.
func sameGlyphs(whole, part []Glyph, start, end int) error {
	k := 0
	for _, g := range whole {
		if g.ClusterIndex < start || g.ClusterIndex >= end {
			continue
		}
		if k == len(part) {
			return fmt.Errorf("got %d glyphs, want more", len(part))
		}
		p := part[k]
		if p.GlyphID != g.GlyphID || p.ClusterIndex != g.ClusterIndex || p.Advance != g.Advance ||
			p.XOffset != g.XOffset || p.YOffset != g.YOffset {
			return fmt.Errorf("glyph %d: got %d at rune %d, advance %d, offset %d,%d, want %d at rune %d, advance %d, offset %d,%d",
				k, p.GlyphID, p.ClusterIndex, p.Advance, p.XOffset, p.YOffset,
				g.GlyphID, g.ClusterIndex, g.Advance, g.XOffset, g.YOffset)
		}
		k++
	}
	if k != len(part) {
		return fmt.Errorf("got %d glyphs, want %d", len(part), k)
	}
	return nil
}

// TestUnsafeToBreakKerning tests the flag of a pair of glyphs that the
// font kerns, and of a space between two words.
func TestUnsafeToBreakKerning(t *testing.T) {
	var shaper HarfbuzzShaper
	text := []rune("AV ab")
	out := shaper.Shape(Input{
		Text: text, RunEnd: len(text), Direction: di.DirectionLTR,
		Face: loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf"), Size: 16 * 64,
		Script: language.Latin, Language: language.NewLanguage("en"),
	})
	want := []bool{false, true, false, false, false}
	if len(out.Glyphs) != len(want) {
		t.Fatalf("got %d glyphs, want %d", len(out.Glyphs), len(want))
	}
	for i, g := range out.Glyphs {
		if g.UnsafeToBreak() != want[i] {
			t.Errorf("glyph %d: got unsafe to break %t, want %t", i, g.UnsafeToBreak(), want[i])
		}
	}
}

// TestShapeInto makes sure that ShapeInto gives the output of Shape, in the
// memory of its buffer.
func TestShapeInto(t *testing.T) {
	var shaper HarfbuzzShaper
	var buf []Glyph
	for i, input := range unsafeTexts(t) {
		want := shaper.Shape(input)
		got := shaper.ShapeInto(input, buf)
		if len(got.Glyphs) != len(want.Glyphs) {
			t.Fatalf("text %d: got %d glyphs, want %d", i, len(got.Glyphs), len(want.Glyphs))
		}
		for j := range want.Glyphs {
			if got.Glyphs[j] != want.Glyphs[j] {
				t.Fatalf("text %d, glyph %d: got %+v, want %+v", i, j, got.Glyphs[j], want.Glyphs[j])
			}
		}
		if got.Advance != want.Advance || got.Size != want.Size || got.LineBounds != want.LineBounds ||
			got.GlyphBounds != want.GlyphBounds || got.Direction != want.Direction ||
			got.Runes != want.Runes || got.Face != want.Face || got.VisualIndex != want.VisualIndex {
			t.Fatalf("text %d: got %+v, want %+v", i, got, want)
		}
		buf = buf[:cap(buf)]
		if out := shaper.ShapeInto(input, buf); len(buf) >= len(out.Glyphs) && &out.Glyphs[0] != &buf[0] {
			t.Fatalf("text %d: the glyphs are not in the memory of the buffer", i)
		}
		buf = shaper.ShapeInto(input, buf).Glyphs
	}

	input := unsafeTexts(t)[0]
	buf = shaper.ShapeInto(input, nil).Glyphs
	shape := testing.AllocsPerRun(10, func() { _ = shaper.Shape(input) })
	into := testing.AllocsPerRun(10, func() { buf = shaper.ShapeInto(input, buf).Glyphs })
	if into >= shape {
		t.Fatalf("ShapeInto allocates %v times, Shape %v times, want fewer", into, shape)
	}
}

// BenchmarkShape measures the shaper on a paragraph, with Shape and with
// ShapeInto.
func BenchmarkShape(b *testing.B) {
	text := []rune(benchParagraphLatin)
	input := Input{
		Text: text, RunEnd: len(text), Direction: di.DirectionLTR,
		Face: loadOpentypeFont(b, "../font/testdata/Roboto-Regular.ttf"), Size: fixed.I(16),
		Script: language.Latin, Language: language.NewLanguage("en"),
	}
	b.Run("Shape", func(b *testing.B) {
		var shaper HarfbuzzShaper
		b.ReportAllocs()
		for b.Loop() {
			_ = shaper.Shape(input)
		}
	})
	b.Run("ShapeInto", func(b *testing.B) {
		var shaper HarfbuzzShaper
		var buf []Glyph
		b.ReportAllocs()
		for b.Loop() {
			buf = shaper.ShapeInto(input, buf).Glyphs
		}
	})
}
