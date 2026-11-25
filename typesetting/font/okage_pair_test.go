// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font_test

// okage: this file is not in go-text. It tests KernPair and LigaturePair
// of D78 section 8 against the shaper, which applies GPOS and GSUB in full
// (D95).

import (
	"bytes"
	"os"
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/language"
	"github.com/xo/okage/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// pairFace parses the font file at name.
func pairFace(t *testing.T, name string) *font.Face {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return face
}

// shapePair shapes the two runes of s with face at the size of its em, with
// only the features on.
func shapePair(face *font.Face, s string, features ...shaping.FontFeature) shaping.Output {
	text := []rune(s)
	return (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{
		Text: text, RunEnd: len(text), Direction: di.DirectionLTR, Face: face,
		Size: fixed.Int26_6(int32(face.Upem()) * 64), Script: language.Latin,
		FontFeatures: features,
	})
}

// TestKernPair compares KernPair with the change of the advance that the
// shaper gives a pair without ligatures.
func TestKernPair(t *testing.T) {
	noLiga := []shaping.FontFeature{
		{Tag: ot.MustNewTag("liga"), Value: 0},
		{Tag: ot.MustNewTag("clig"), Value: 0},
	}
	for _, name := range []string{
		"../../svg/testdata/fonts/NotoSans-Regular.ttf",
		"../../svg/testdata/fonts/SourceSansPro-Regular.ttf",
		"../../svg/testdata/fonts/NotoSerif-Regular.ttf",
	} {
		face := pairFace(t, name)
		kerned := 0
		for _, pair := range []string{"AV", "To", "Ty", "LT", "VA", "Wa", "ab", "xy", "P.", "F,", "11", "r."} {
			out := shapePair(face, pair, noLiga...)
			if len(out.Glyphs) != 2 {
				t.Fatalf("%s: %q gives %d glyphs", name, pair, len(out.Glyphs))
			}
			left, right := out.Glyphs[0].GlyphID, out.Glyphs[1].GlyphID
			want := float32(out.Glyphs[0].XAdvance)/64 - face.HorizontalAdvance(left)
			got := face.KernPair(left, right)
			if got != want {
				t.Errorf("%s: KernPair(%q) = %v, want %v", name, pair, got, want)
			}
			if got != 0 {
				kerned++
			}
		}
		if kerned == 0 {
			t.Errorf("%s: no pair has a kern", name)
		}
	}
}

// TestLigaturePair compares LigaturePair with the ligatures of the shaper.
func TestLigaturePair(t *testing.T) {
	face := pairFace(t, "../../svg/testdata/fonts/SourceSansPro-Regular.ttf")
	ligatures := 0
	for _, pair := range []string{"fi", "fl", "ff", "ab", "AV", "fj"} {
		plain := shapePair(face, pair, shaping.FontFeature{Tag: ot.MustNewTag("liga"), Value: 0})
		if len(plain.Glyphs) != 2 {
			t.Fatalf("%q without liga gives %d glyphs", pair, len(plain.Glyphs))
		}
		out := shapePair(face, pair)
		got, ok := face.LigaturePair(plain.Glyphs[0].GlyphID, plain.Glyphs[1].GlyphID)
		switch {
		case len(out.Glyphs) == 1:
			ligatures++
			if !ok || got != out.Glyphs[0].GlyphID {
				t.Errorf("LigaturePair(%q) = %d, %v, want %d", pair, got, ok, out.Glyphs[0].GlyphID)
			}
		case ok:
			t.Errorf("LigaturePair(%q) = %d, want none", pair, got)
		}
	}
	if ligatures == 0 {
		t.Error("no pair has a ligature")
	}
	// A font without GSUB, or a pair of glyphs that the font does not hold,
	// has no ligature.
	if _, ok := face.LigaturePair(0, 0); ok {
		t.Error("the pair of .notdef has a ligature")
	}
}
