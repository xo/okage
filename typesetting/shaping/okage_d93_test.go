// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package shaping

import (
	"testing"

	"github.com/xo/okage/typesetting/di"
	"github.com/xo/okage/typesetting/harfbuzz"
	"github.com/xo/okage/typesetting/language"
)

// okage: the tests of D93.

// TestSetUnsafeToConcat shapes texts in Roboto and Amiri with the flag
// unsafe-to-concat off and on. The glyphs and their positions must be the
// same. With the flag off, no glyph has it. With the flag on, each glyph
// that is unsafe to break is unsafe to concat, and some glyphs that are
// safe to break are unsafe to concat. A new shaper has the flag off.
func TestSetUnsafeToConcat(t *testing.T) {
	roboto := loadOpentypeFont(t, "../font/testdata/Roboto-Regular.ttf")
	amiri := loadOpentypeFont(t, "../font/testdata/Amiri-Regular.ttf")
	inputs := []Input{
		{Text: []rune("AVATAR Wave, To. office"), Face: roboto, Direction: di.DirectionLTR, Script: language.Latin},
		{Text: []rune("بسم الله الرحمن الرحيم"), Face: amiri, Direction: di.DirectionRTL, Script: language.Arabic},
	}
	var off, on HarfbuzzShaper
	on.SetUnsafeToConcat(true)
	for _, in := range inputs {
		in.RunEnd, in.Size, in.Language = len(in.Text), 16*64, language.NewLanguage("en")
		a, b := off.Shape(in), on.Shape(in)
		if len(a.Glyphs) != len(b.Glyphs) {
			t.Fatalf("%q: got %d glyphs with the flag, want %d", string(in.Text), len(b.Glyphs), len(a.Glyphs))
		}
		concat := 0
		for i, g := range b.Glyphs {
			if a.Glyphs[i].UnsafeToConcat() {
				t.Errorf("%q: glyph %d is unsafe to concat with the flag off", string(in.Text), i)
			}
			if g.UnsafeToBreak() && !g.UnsafeToConcat() {
				t.Errorf("%q: glyph %d is unsafe to break and safe to concat", string(in.Text), i)
			}
			if g.UnsafeToConcat() && !g.UnsafeToBreak() {
				concat++
			}
			ga, gb := a.Glyphs[i], g
			if ga.Mask&^harfbuzz.GlyphUnsafeToConcat != gb.Mask&^harfbuzz.GlyphUnsafeToConcat {
				t.Errorf("%q: glyph %d has the mask %#x with the flag, want %#x", string(in.Text), i, gb.Mask, ga.Mask)
			}
			ga.Mask, gb.Mask = 0, 0
			if ga != gb {
				t.Errorf("%q: glyph %d is %+v with the flag, want %+v", string(in.Text), i, gb, ga)
			}
		}
		if concat == 0 {
			t.Errorf("%q: no glyph is only unsafe to concat", string(in.Text))
		}
	}
	on.SetUnsafeToConcat(false)
	for _, g := range on.Shape(inputs[0]).Glyphs {
		if g.UnsafeToConcat() {
			t.Fatal("a glyph is unsafe to concat after SetUnsafeToConcat(false)")
		}
	}
}

// TestShapedReversed tests the runs that the shaper shapes against their
// direction, as ensureNativeDirection of harfbuzz decides.
func TestShapedReversed(t *testing.T) {
	sideways := di.DirectionTTB
	sideways.SetSideways(true)
	for _, c := range []struct {
		dir    di.Direction
		script language.Script
		text   string
		want   bool
	}{
		{di.DirectionLTR, language.Latin, "ab", false},
		{di.DirectionRTL, language.Latin, "ab", true},
		{di.DirectionRTL, language.Common, "ــ ", true},
		{di.DirectionRTL, language.Arabic, "ــ ", false},
		{di.DirectionLTR, language.Arabic, "'')", true},
		{di.DirectionLTR, language.Arabic, "'')[0", false},
		{di.DirectionLTR, language.Arabic, "0x", true},
		{di.DirectionLTR, language.Arabic, "\U0001F1EB\U0001F1F7", false},
		{di.DirectionLTR, language.Runic, "ᚠ", false},
		{di.DirectionRTL, language.Runic, "ᚠ", false},
		{di.DirectionTTB, language.Latin, "ab", false},
		{di.DirectionBTT, language.Latin, "ab", true},
		{sideways, language.Arabic, "'0", false},
		{sideways, language.Arabic, "'", true},
	} {
		in := Input{Direction: c.dir, Script: c.script}
		if got := shapedReversed(&in, []rune(c.text)); got != c.want {
			t.Errorf("%v, %v, %+q: got %v, want %v", c.dir, c.script, c.text, got, c.want)
		}
	}
}
