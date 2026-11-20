// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// A large operator grows in display style. TeX takes one step up the
// charlist of the font:
//
//	if (cur_style<text_style)and(char_tag(cur_i)=list_tag) then {make it larger}
//	  begin c:=rem_byte(cur_i); …; character(nucleus(q)):=c; end   tex.web:14685-14691
//
// A font with a MATH table says the same with the vertical variants of the
// glyph and the constant DisplayOperatorMinHeight.
func TestDisplayGrowsALargeOperator(t *testing.T) {
	for _, tex := range []string{`\int`, `\sum`, `\prod`, `\oint`, `\bigcup`} {
		inline := mustLayout(t, tex, 32, Text)
		display := mustLayout(t, tex, 32, Display)
		in := inline.Height + inline.Depth
		dp := display.Height + display.Depth
		if dp <= in {
			t.Errorf("%s: display %v is not taller than inline %v", tex, dp, in)
		}
	}
}

// The display \int is the first vertical variant of its glyph that reaches
// DisplayOperatorMinHeight, centered on the math axis. The values are
// computed again from the font.
//
// okage: the operator is centered on the axis, as Appendix G, rule 13,
// says. The source sets the variant on the baseline.
func TestDisplayOperatorIsTheVariantOnTheAxis(t *testing.T) {
	const size = 32
	face := testFace(t)
	target := mathConst(face, font.DisplayOperatorMinHeight, size)
	var g font.GID
	found := false
	for _, v := range face.MathGlyphVariants(glyphOf(t, face, '∫'), true) {
		g = v.Glyph
		if float64(float64(v.Advance)*(size/float64(face.Upem()))) >= target {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no variant of ∫ reaches DisplayOperatorMinHeight")
	}
	w, h, d := gidMetrics(face, g, size)
	w += gidItalic(face, g, size) // the italic correction follows it
	dy := (h-d)/2 - mathConst(face, font.AxisHeight, size)
	b := mustLayout(t, `\int`, size, Display)
	glyphs := itemsOf(b, ItemGlyph)
	if len(glyphs) != 1 || glyphs[0].Glyph != g {
		t.Fatalf(`display \int draws %+v, want the variant %d`, glyphs, g)
	}
	if !near(b.Width, w) || !near(b.Height, h-dy) || !near(b.Depth, max(d+dy, 0)) || !near(glyphs[0].Y, dy) {
		t.Errorf(`display \int: %v %v %v, glyph at y %v, want %v %v %v at %v`,
			b.Width, b.Height, b.Depth, glyphs[0].Y, w, h-dy, max(d+dy, 0), dy)
	}
}

// An ordinary atom does not grow: only a large operator takes the variant of
// display style.
func TestDisplayLeavesOrdinaryAtomsAlone(t *testing.T) {
	for _, tex := range []string{`x`, `\alpha`, `+`, `\leq`} {
		inline := mustLayout(t, tex, 32, Text)
		display := mustLayout(t, tex, 32, Display)
		if got, want := display.Height+display.Depth, inline.Height+inline.Depth; got != want {
			t.Errorf("%s: display %v != inline %v: an ordinary atom does not grow", tex, got, want)
		}
	}
}

// An operator that already reaches DisplayOperatorMinHeight stays as it is.
// No glyph of STIX Two Math does, so the contract is checked here with a box
// tall enough. It keeps a MATH table without the constant, where
// MathConstant gives 0, from changing the glyph for its first variant.
func TestDisplayOperatorKeepsAnOperatorAlreadyTallEnough(t *testing.T) {
	face := testFace(t)
	l := &layout{face: face, size: 32, upem: float64(face.Upem()), cfg: newConfig(nil), color: okage.ColorBlack}
	tall := newBox(Op)
	tall.h = l.mc(font.DisplayOperatorMinHeight, 32) + 1
	if got := l.displayOperator('∫', 32, Op, tall); got != tall {
		t.Errorf("an operator %v tall was replaced, although the target is %v",
			tall.h, l.mc(font.DisplayOperatorMinHeight, 32))
	}
}
