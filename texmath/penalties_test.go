// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"reflect"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// onFace lays out tex with face at size in the style st. The tests that
// compare two boxes lay out both with one face, so that the items hold the
// same face.
func onFace(tb testing.TB, face *font.Face, tex string, size float64, st Style) *Box {
	tb.Helper()
	b, err := LayoutTeX(tex, face, size, st)
	if err != nil {
		tb.Fatalf("LayoutTeX(%q): %v", tex, err)
	}
	return b
}

// equalBoxes reports whether a and b have the same metrics and the same
// items. It takes the place of the byte equality of the SVG text that the
// tests of the source compare.
func equalBoxes(a, b *Box) bool {
	return a.Width == b.Width && a.Height == b.Height && a.Depth == b.Depth &&
		a.Italic == b.Italic && reflect.DeepEqual(a.Items, b.Items)
}

// engineFor returns the state of a layout with face at size, for a test that
// calls a layout half itself, as the tests of the source build an engine.
func engineFor(face *font.Face, size float64) *layout {
	return &layout{face: face, size: size, upem: float64(face.Upem()), cfg: newConfig(nil), color: okage.ColorBlack}
}

func TestAPenaltyChangesNothingAtAll(t *testing.T) {
	for _, c := range []struct{ with, without string }{
		{`a+\allowbreak b`, `a+b`},
		{`a+\nobreak b`, `a+b`},
		{`a+\break b`, `a+b`},
		{`\allowbreak x`, `x`},
		{`x\allowbreak`, `x`},
		{`\frac{a\allowbreak +b}{c}`, `\frac{a+b}{c}`},
		{`x^{a\allowbreak b}`, `x^{ab}`},
		{`a+\relax\protect\leavevmode\noindent\ignorespaces\par\normalfont\normalcolor b`, `a+b`},
	} {
		if !sameTree(t, c.with, c.without) {
			t.Errorf("%q and %q do not parse alike", c.with, c.without)
		}
	}
}

// A penalty makes no node, so the box is the box without it. The source
// compares the bytes of the SVG text, and this compares the metrics and each
// item of the box.
func TestAPenaltyLeavesTheBoxAlone(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct{ with, without string }{
		{`a+\allowbreak b`, `a+b`},
		{`a+\nobreak b`, `a+b`},
		{`a+\break b`, `a+b`},
		{`\allowbreak x`, `x`},
		{`x\allowbreak`, `x`},
		{`\frac{a\allowbreak +b}{c}`, `\frac{a+b}{c}`},
		{`x^{a\allowbreak b}`, `x^{ab}`},
	} {
		if !equalBoxes(onFace(t, face, c.with, 32, Display), onFace(t, face, c.without, 32, Display)) {
			t.Errorf("%q and %q do not lay out alike", c.with, c.without)
		}
	}
}

func TestAPenaltyDoesNotEatItsNeighbour(t *testing.T) {
	if sameTree(t, `a+\allowbreak b`, `a+`) {
		t.Error(`\allowbreak swallowed the b`)
	}
	if sameTree(t, `a\allowbreak +b`, `a b`) {
		t.Error(`\allowbreak swallowed the +`)
	}
	face := testFace(t)
	if equalBoxes(onFace(t, face, `a+\allowbreak b`, 32, Display), onFace(t, face, `a+`, 32, Display)) {
		t.Error(`\allowbreak swallowed the b in the box`)
	}
	if equalBoxes(onFace(t, face, `a\allowbreak +b`, 32, Display), onFace(t, face, `a b`, 32, Display)) {
		t.Error(`\allowbreak swallowed the + in the box`)
	}
}

func TestMathstrutIsAVphantomParenthesis(t *testing.T) {
	if !sameTree(t, `\mathstrut`, `\vphantom{(}`) {
		t.Error(`\mathstrut is not \vphantom{(}`)
	}
	if !sameTree(t, `x\mathstrut`, `x\vphantom{(}`) {
		t.Error(`x\mathstrut is not x\vphantom{(}`)
	}
}

// \mathstrut has the height and the depth of a parenthesis, and no width.
// The source checks that it is taller than an x. Here the extents are also
// those of the glyph "(" of the face, computed again from the font.
func TestMathstrutHasTheExtentsOfAParenthesis(t *testing.T) {
	face := testFace(t)
	strut := onFace(t, face, `\mathstrut`, 32, Text)
	_, h, d := glyphMetrics(t, face, '(', 32)
	if strut.Width != 0 || !near(strut.Height, h) || !near(strut.Depth, d) {
		t.Errorf(`\mathstrut = %v×%v/%v, want 0×%v/%v`, strut.Width, strut.Height, strut.Depth, h, d)
	}
	if len(strut.Items) != 0 {
		t.Errorf(`\mathstrut draws %d items, want none`, len(strut.Items))
	}
	if x := onFace(t, face, `x`, 32, Text); strut.Height <= x.Height {
		t.Errorf(`\mathstrut height %v is not above the height %v of x`, strut.Height, x.Height)
	}
}

// A \mathstrut raises the height of its list, and does not make it wider.
func TestMathstrutRaisesTheListItSitsIn(t *testing.T) {
	face := testFace(t)
	with, without := onFace(t, face, `x\mathstrut`, 32, Text), onFace(t, face, `x`, 32, Text)
	if with.Height <= without.Height {
		t.Errorf(`x\mathstrut is %v high, x alone %v: the strut does nothing`, with.Height, without.Height)
	}
	if with.Width != without.Width {
		t.Errorf(`x\mathstrut is %v wide, x alone %v: the strut took width`, with.Width, without.Width)
	}
}

// Each command that carries no glyph and takes no argument makes no node, so
// a+X b lays out as a+b. An empty Ord atom between + and b would change the
// class Bin of +.
//
// okage: the size commands other than \normalsize set the size of b, as phase
// 26 of D78 adds, so they are not in the list.
func TestEveryTransparentCommandIsActuallyTransparent(t *testing.T) {
	face := testFace(t)
	base := onFace(t, face, `a+b`, 32, Display)
	for _, n := range []string{
		"allowbreak", "nobreak", "break",
		"limits", "nolimits", "displaylimits",
		"normalfont", "rmfamily", "sffamily", "ttfamily", "bfseries", "mdseries",
		"itshape", "slshape", "scshape", "upshape", "em", "normalcolor",
		"normalsize",
		"par", "relax", "protect", "leavevmode", "noindent", "ignorespaces",
	} {
		if !equalBoxes(onFace(t, face, `a+\`+n+` b`, 32, Display), base) {
			t.Errorf(`a+\%s b does not lay out as a+b: it is not transparent`, n)
		}
	}
	// And next to an Ord, an Open, a script and an operator.
	for _, c := range []struct{ with, without string }{
		{`x\relax y`, `xy`},
		{`\relax (x)`, `(x)`},
		{`x^{2\relax }`, `x^{2}`},
		{`\sin\relax x`, `\sin x`},
	} {
		if !equalBoxes(onFace(t, face, c.with, 32, Display), onFace(t, face, c.without, 32, Display)) {
			t.Errorf("%q does not lay out as %q", c.with, c.without)
		}
	}
}
