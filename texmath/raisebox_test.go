// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"testing"

	"github.com/xo/okage"
)

// The first tests read the tree. The tests after them measure the box of
// \raisebox and read the place of its items, as the tests of the source read
// the translation of the SVG text. They compare the exact lengths: one point
// is one unit of the size.

func TestADeclaredHeightReplacesAndOneBracketLeavesTheDepthAlone(t *testing.T) {
	pt := func(v float64) Length { return Length{Value: v, Unit: Pt} }
	for _, c := range []struct {
		tex  string
		want Raise
	}{
		{`\raisebox{10pt}{x}`, Raise{Lift: pt(10)}},
		{`\raisebox{10pt}[3pt]{x}`, Raise{Lift: pt(10), Height: pt(3), HasHeight: true}},
		{`\raisebox{10pt}[3pt][4pt]{x}`, Raise{Lift: pt(10), Height: pt(3), Depth: pt(4), HasHeight: true, HasDepth: true}},
		{`\raisebox{10pt}[]{x}`, Raise{Lift: pt(10)}},
		{`\raisebox{1ex}{x}`, Raise{Lift: Length{1, Ex}}},
		{`\raisebox{-0.65ex}{x}`, Raise{Lift: Length{-0.65, Ex}}},
		{`\raisebox{1em}{x}`, Raise{Lift: Length{1, Em}}},
	} {
		got, ok := mustParse(t, c.tex)[0].(Raise)
		if !ok {
			t.Errorf("%s is not a Raise", c.tex)
			continue
		}
		got.Body = nil
		if got != c.want {
			t.Errorf("%s = %#v, want %#v", c.tex, got, c.want)
		}
	}
}

func TestARaiseboxWithoutALiftIsAnError(t *testing.T) {
	for _, tex := range []string{`\raisebox x`, `\raisebox{2pt}{`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
	// An unclosed [ is a character, not a failure.
	for _, tex := range []string{
		`\raisebox{2pt}[3pt{x}`,
		`\raisebox{2pt}[3pt`,
		`\raisebox{2pt}[{x}`,
		`\raisebox{2pt}[3pt][4pt{x}`,
	} {
		mustParse(t, tex)
	}
}

func TestARaiseboxComposes(t *testing.T) {
	for _, tex := range []string{
		`x^{\raisebox{1pt}{a}}`,
		`\frac{\raisebox{2pt}{a}}{b}`,
		`\raisebox{2pt}{\raisebox{2pt}{a}}`,
		`\raisebox{2pt}{$a+b$}`,
		`a\raisebox{1pt}{b}c`,
	} {
		mustParse(t, tex)
	}
}

// okage: a length past the limit is ErrMathLimit.
func TestARaiseboxLengthLimit(t *testing.T) {
	for _, tex := range []string{`\raisebox{99999pt}{x}`, `\raisebox{1pt}[99999em]{x}`} {
		if _, err := Parse(tex); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("%s: %v, want ErrMathLimit", tex, err)
		}
	}
}

// The sign: y grows down, so a raise is a negative y of the glyph.
func TestARaiseboxShiftsUpwardsNotDownwards(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`\raisebox{10pt}{x}`, -10},
		{`\raisebox{-10pt}{x}`, 10},
	} {
		g := itemsOf(onFace(t, face, c.tex, 32, Text), ItemGlyph)
		if len(g) != 1 || g[0].Y != c.want || g[0].X != 0 {
			t.Errorf("%s: glyphs %+v, want one at y = %v", c.tex, g, c.want)
		}
	}
}

// The raise moves the extents and not the width: h+lift and d-lift. The
// depth of the raised box goes negative, and the list clamps it at zero, so
// the box shows no depth.
func TestARaiseboxMovesTheExtentsAndNotTheWidth(t *testing.T) {
	face := testFace(t)
	base := onFace(t, face, `x`, 32, Text)
	up := onFace(t, face, `\raisebox{10pt}{x}`, 32, Text)
	if up.Width != base.Width {
		t.Errorf("width %v, want %v unchanged", up.Width, base.Width)
	}
	if up.Height != base.Height+10 {
		t.Errorf("height %v, want %v", up.Height, base.Height+10)
	}
	if up.Depth != 0 {
		t.Errorf("depth %v, want 0: the list clamps the negative depth", up.Depth)
	}
	if down := onFace(t, face, `\raisebox{-10pt}{x}`, 32, Text); down.Depth != base.Depth+10 {
		t.Errorf("lowered depth %v, want %v", down.Depth, base.Depth+10)
	}
}

// A declared height replaces the height, and is not added to it. With one
// bracket the depth keeps what the raise left, which raiseBox shows before
// the list clamps it. An empty bracket is not a zero.
func TestADeclaredHeightReplacesTheBox(t *testing.T) {
	face := testFace(t)
	base := onFace(t, face, `x`, 32, Text)
	if b := onFace(t, face, `\raisebox{10pt}[3pt]{x}`, 32, Text); b.Height != 3 {
		t.Errorf("height %v, want 3: replaced, not added to the raise", b.Height)
	}
	if b := onFace(t, face, `\raisebox{10pt}[3pt][4pt]{x}`, 32, Text); b.Height != 3 || b.Depth != 4 {
		t.Errorf("two brackets: h %v d %v, want 3 and 4", b.Height, b.Depth)
	}
	if b := onFace(t, face, `\raisebox{10pt}[]{x}`, 32, Text); b.Height != base.Height+10 {
		t.Errorf("[] gave the height %v, want %v", b.Height, base.Height+10)
	}
	l := engineFor(face, 32)
	x := l.mustGlyph('x', 32, Ord)
	one := l.raiseBox(x, 10, 3, 0, true, false)
	if one.h != 3 || one.d != x.d-10 {
		t.Errorf("one bracket: h %v d %v, want 3 and %v", one.h, one.d, x.d-10)
	}
	x = l.mustGlyph('x', 32, Ord)
	if two := l.raiseBox(x, 10, 3, 4, true, true); two.h != 3 || two.d != 4 {
		t.Errorf("two brackets: h %v d %v, want 3 and 4", two.h, two.d)
	}
}

// 1ex is the height of the upright glyph x of the face, computed again from
// the font, and not the height of the italic x that math sets.
func TestARaiseboxTakesTheUnitsThePapersWrite(t *testing.T) {
	face := testFace(t)
	_, ex, _ := glyphMetrics(t, face, 'x', 32)
	base := onFace(t, face, `x`, 32, Text)
	if b := onFace(t, face, `\raisebox{1ex}{x}`, 32, Text); !near(b.Height, base.Height+ex) {
		t.Errorf(`\raisebox{1ex}{x} height %v, want %v`, b.Height, base.Height+ex)
	}
	if b := onFace(t, face, `\raisebox{2ex}{x}`, 32, Text); !near(b.Height, base.Height+float64(2*ex)) {
		t.Errorf(`\raisebox{2ex}{x} height %v, want %v`, b.Height, base.Height+float64(2*ex))
	}
	if b := onFace(t, face, `\raisebox{1em}{x}`, 32, Text); b.Height != base.Height+32 {
		t.Errorf(`\raisebox{1em}{x} height %v, want %v`, b.Height, base.Height+32)
	}
	for _, tex := range []string{
		`\raisebox{.7ex}{x}`, `\raisebox{8pt}{x}`, `\raisebox{-0.65ex}{x}`,
		`\raisebox{-2pt}{x}`, `\raisebox{0.2ex}{x}`,
	} {
		onFace(t, face, tex, 32, Text)
	}
}

// A \raisebox lays out inside a script, inside a fraction, and nested. The
// nested raises add up.
func TestARaiseboxComposesInTheBox(t *testing.T) {
	face := testFace(t)
	for _, tex := range []string{
		`x^{\raisebox{1pt}{a}}`,
		`\frac{\raisebox{2pt}{a}}{b}`,
		`\raisebox{2pt}{$a+b$}`,
		`a\raisebox{1pt}{b}c`,
	} {
		onFace(t, face, tex, 32, Display)
	}
	g := itemsOf(onFace(t, face, `\raisebox{2pt}{\raisebox{2pt}{a}}`, 32, Text), ItemGlyph)
	if len(g) != 1 || g[0].Y != -4 {
		t.Errorf("nested raises: %+v, want one glyph at y = -4", g)
	}
}

// raiseBox keeps a negative depth, as TeX allows for \hbox{\raise l \box},
// and the list clamps it at zero.
func TestRaiseBoxItselfCarriesANegativeDepth(t *testing.T) {
	face := testFace(t)
	l := engineFor(face, 32)
	x := l.mustGlyph('x', 32, Ord)
	xw, xh, xd := x.w, x.h, x.d
	up := l.raiseBox(x, 10, 0, 0, false, false)
	if up.d != xd-10 || up.d >= 0 {
		t.Errorf("raiseBox depth = %v, want %v, below zero", up.d, xd-10)
	}
	if up.h != xh+10 || up.w != xw {
		t.Errorf("raiseBox = %v×%v, want %v×%v", up.w, up.h, xw, xh+10)
	}
	if b := l.hlist([]entry{{b: up, st: Text, atom: true}}); b.d != 0 {
		t.Errorf("hlist depth = %v, want 0", b.d)
	}
}
