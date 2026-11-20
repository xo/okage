// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"math"
	"testing"
)

// This file ports the layout parts of nesteddelims_test.go of the source. A
// math delimiter in math, a bare $ or \( \) \[ \], is redundant: the engine
// lifted text into a string that this package reads as math, and in the
// document the argument of the wrapper is text. latex.ltx:11298 is
// \relax\ifmmode\@badmath\else$\fi: outside math, \( is $.
//
// The source compares the SVG text. These tests compare the items and the
// metrics of the boxes, which is the same claim.

// nestedSame reports whether a and b have the same metrics and the same
// items, kind, glyph and place.
func nestedSame(a, b *Box) bool {
	if a.Width != b.Width || a.Height != b.Height || a.Depth != b.Depth || len(a.Items) != len(b.Items) {
		return false
	}
	for i, x := range a.Items {
		y := b.Items[i]
		if x.Kind != y.Kind || x.Glyph != y.Glyph || x.X != y.X || x.Y != y.Y || x.W != y.W || x.H != y.H {
			return false
		}
	}
	return true
}

// The tests compare with the form without the delimiters, because that is the
// claim. A shape that renders would pass for a delimiter that ate its
// neighbor, or for one set as a glyph, which is what a bare $ was.
func TestANestedMathsDelimiterIsConsumedAndNotTypeset(t *testing.T) {
	base := mustLayout(t, `a+b`, 32, Display)
	for _, tex := range []string{
		`$a+b$`, `\(a+b\)`, `\[a+b\]`,
		`$a+b`, `a+b$`, // one without its pair is not an atom either
		`\(a+b`, `a+b\)`,
	} {
		if got := mustLayout(t, tex, 32, Display); !nestedSame(got, base) {
			t.Errorf("%q does not lay out as %q", tex, `a+b`)
		}
	}
}

// A bare $ used to be set as a glyph: $a+b$ measured 104.22 against 72.22
// for a+b in the source. The width is the number that was wrong, so the test
// checks it.
func TestABareDollarAddsNoWidth(t *testing.T) {
	w := func(tex string) float64 { return mustLayout(t, tex, 32, Text).Width }
	if bare, dollared := w(`a+b`), w(`$a+b$`); bare != dollared {
		t.Errorf(`$a+b$ is %.2f wide, a+b is %.2f: the dollars are set`, dollared, bare)
	}
	// A literal dollar is \$, from the symbol table, and it keeps its ink.
	if w(`\$`) <= 0 {
		t.Error(`\$ has no width: the literal dollar was lost with the delimiters`)
	}
	if w(`a\$b`) <= w(`ab`) {
		t.Errorf(`a\$b (%.2f) is not wider than ab (%.2f)`, w(`a\$b`), w(`ab`))
	}
}

// The delimiters must not eat their neighbors: the equality above would hold
// as well if they did, on both sides.
func TestANestedDelimiterDoesNotEatItsNeighbour(t *testing.T) {
	l := func(tex string) *Box { return mustLayout(t, tex, 32, Display) }
	if nestedSame(l(`$a+b$`), l(`a+`)) {
		t.Error(`the closing $ swallowed the b`)
	}
	if nestedSame(l(`\(a+b\)`), l(`+b`)) {
		t.Error(`the opening \( swallowed the a`)
	}
}

// \text is an alphabet here, so a $ in it returns to math for its span, and
// the closing $ puts the alphabet back. The width tells the three possible
// behaviors apart, because it is the glyph that is chosen.
//
// okage: the widths are computed again from the font. The x of math is the
// math italic x, U+1D465, and the x of \text is the upright x of the math
// face, as the source sets \text.
func TestANestedDollarInsideTextSetsTheMathGlyph(t *testing.T) {
	face := testFace(t)
	mathsW, _, _ := glyphMetrics(t, face, mathItalic('x'), 32)
	mathsW += italicOf(t, face, mathItalic('x'), 32) // phase 26 of D78
	textW, _, _ := glyphMetrics(t, face, 'x', 32)
	w := func(tex string) float64 { return mustLayout(t, tex, 32, Text).Width }
	if got := w(`x`); got != mathsW {
		t.Fatalf("x is %.3f wide, the math italic x is %.3f", got, mathsW)
	}
	if got := w(`\text{x}`); got != textW {
		t.Fatalf(`\text{x} is %.3f wide, the upright x is %.3f`, got, textW)
	}
	if mathsW == textW {
		t.Fatal("the two x are the same width: this test cannot tell the faces apart")
	}
	if got := w(`\text{$x$}`); got != mathsW {
		t.Errorf(`\text{$x$} is %.3f wide, the math x is %.3f: the span did not return to math`, got, mathsW)
	}
	if got, want := w(`\text{$x$x}`), mathsW+textW; !near(got, want) {
		t.Errorf(`\text{$x$x} is %.3f wide, want %.3f (one math x, then one text x): `+
			`the alphabet was not put back after the closing $`, got, want)
	}
}

// The forms of the corpus that showed the fault, end to end. The scaled box
// does not carry the ink of the dollars: it is half of a+b, not half of
// $a+b$.
func TestTheCorpusWrapperFormsRender(t *testing.T) {
	for _, tex := range []string{
		`\scalebox{0.5}{$a+b$}`,
		`\raisebox{1pt}{\(a+b\)}`,
		`\raisebox{-2pt}{\text{$x$}}`,
		`\scalebox{1.3}{\ensuremath{$\alpha$}}`,
	} {
		for _, st := range allStyles {
			mustLayout(t, tex, 32, st)
		}
	}
	plain := mustLayout(t, `a+b`, 32, Text)
	scaled := mustLayout(t, `\scalebox{0.5}{$a+b$}`, 32, Text)
	if want := 0.5 * plain.Width; math.Abs(scaled.Width-want) > 1e-9 {
		t.Errorf(`\scalebox{0.5}{$a+b$} is %.2f wide, want %.2f (half of a+b)`, scaled.Width, want)
	}
}
