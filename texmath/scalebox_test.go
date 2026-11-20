// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// The first tests read the factors of the tree. The tests after them measure
// the scaled box and read the transforms of its items, as the tests of the
// source read the SVG text. The source compares within 0.01 pixels, and these
// compare the exact products of the factors and the box of abc.

func scale(t *testing.T, tex string) Scale {
	t.Helper()
	s, ok := mustParse(t, tex)[0].(Scale)
	if !ok {
		t.Fatalf("%s is not a Scale", tex)
	}
	return s
}

func TestScaleboxScalesByItsFactors(t *testing.T) {
	for _, c := range []struct {
		tex  string
		x, y float64
	}{
		{`\scalebox{1}{abc}`, 1, 1},
		{`\scalebox{0.5}{abc}`, 0.5, 0.5},
		{`\scalebox{2}{abc}`, 2, 2},
		{`\scalebox{0.5}[1]{abc}`, 0.5, 1},
		{`\scalebox{1}[0.5]{abc}`, 1, 0.5},
		{`\scalebox{2}[0.5]{abc}`, 2, 0.5},
		{`\scalebox{-1}[1]{abc}`, -1, 1},
		{`\scalebox{-2}[1]{abc}`, -2, 1},
		{`\scalebox{1}[-1]{abc}`, 1, -1},
		{`\reflectbox{abc}`, -1, 1},
		{`\scalebox{2}[x]{abc}`, 2, 2},
	} {
		if s := scale(t, c.tex); s.X != c.x || s.Y != c.y {
			t.Errorf("%s = %g/%g, want %g/%g", c.tex, s.X, s.Y, c.x, c.y)
		}
	}
	if !sameTree(t, `\reflectbox{abc}`, `\scalebox{-1}[1]{abc}`) {
		t.Error(`\reflectbox is not \scalebox{-1}[1]`)
	}
}

func TestScaleboxRefusesANonNumericFactor(t *testing.T) {
	for _, tex := range []string{`\scalebox{x}{abc}`, `\scalebox{}{abc}`, `\scalebox{0.5x}{abc}`} {
		_, err := Parse(tex)
		if err == nil || !strings.Contains(err.Error(), "not a number") {
			t.Errorf("%s: %v, want a factor that is not a number", tex, err)
		}
	}
}

func TestScaleboxReportsMalformedArguments(t *testing.T) {
	_, err := Parse(`\scalebox{2}`)
	if err == nil || !strings.Contains(err.Error(), "expected an atom") {
		t.Errorf(`\scalebox{2}: %v`, err)
	}
}

func TestAnUnclosedBracketIsTypesetNotRefused(t *testing.T) {
	l := mustParse(t, `\scalebox{2}[1`)
	if s := as[Scale](t, l[0]); s.X != 2 || s.Y != 2 {
		t.Errorf(`\scalebox{2}[1 = %#v: the [ is the content`, s)
	}
	if s := scale(t, `\scalebox{2}[1]{[}`); s.Y != 1 {
		t.Errorf(`\scalebox{2}[1]{[}: vertical factor %g, want 1`, s.Y)
	}
}

// okage: a factor that is not finite, or past 100, is ErrMathLimit. The source
// takes NaN and Inf into its SVG text.
func TestScaleboxFactorLimit(t *testing.T) {
	for _, tex := range []string{
		`\scalebox{NaN}{x}`, `\scalebox{Inf}{x}`, `\scalebox{-Inf}{x}`,
		`\scalebox{1e300}{x}`, `\scalebox{1}[NaN]{x}`, `\scalebox{101}{x}`,
	} {
		if _, err := Parse(tex); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("%s: %v, want ErrMathLimit", tex, err)
		}
	}
	scale(t, `\scalebox{-100}[100]{x}`)
}

// scaleDims returns the width, the height and the depth of tex at the size 10
// in text style.
func scaleDims(t *testing.T, tex string) (w, h, d float64) {
	t.Helper()
	b := mustLayout(t, tex, 10, Text)
	return b.Width, b.Height, b.Depth
}

// The box is |h| times as wide, and v times as high and as deep
// (graphics.sty:526-536).
func TestScaleboxScalesTheBox(t *testing.T) {
	w0, h0, d0 := scaleDims(t, `abc`)
	for _, c := range []struct {
		tex        string
		fw, fh, fd float64
	}{
		{`\scalebox{1}{abc}`, 1, 1, 1},
		{`\scalebox{0.5}{abc}`, 0.5, 0.5, 0.5},
		{`\scalebox{2}{abc}`, 2, 2, 2},
		{`\scalebox{0.5}[1]{abc}`, 0.5, 1, 1},
		{`\scalebox{1}[0.5]{abc}`, 1, 0.5, 0.5},
		{`\scalebox{2}[0.5]{abc}`, 2, 0.5, 0.5},
		{`\scalebox{-1}[1]{abc}`, 1, 1, 1},
		{`\scalebox{-2}[1]{abc}`, 2, 1, 1},
	} {
		w, h, d := scaleDims(t, c.tex)
		if w != float64(c.fw*w0) || h != float64(c.fh*h0) || d != float64(c.fd*d0) {
			t.Errorf("%s = %v/%v/%v, want %vx/%vx/%vx of %v/%v/%v", c.tex, w, h, d, c.fw, c.fh, c.fd, w0, h0, d0)
		}
	}
}

// A negative vertical factor swaps the height and the depth
// (graphics.sty:526-528).
func TestANegativeVerticalFactorSwapsHeightAndDepth(t *testing.T) {
	_, h0, d0 := scaleDims(t, `abc`)
	if h0 <= d0 {
		t.Fatalf("the height %v and the depth %v of abc are not apart", h0, d0)
	}
	if _, h, d := scaleDims(t, `\scalebox{1}[-1]{abc}`); h != d0 || d != h0 {
		t.Errorf(`\scalebox{1}[-1]{abc} = h %v d %v, want %v and %v`, h, d, d0, h0)
	}
	if _, h, d := scaleDims(t, `\scalebox{1}[-2]{abc}`); h != 2*d0 || d != 2*h0 {
		t.Errorf(`\scalebox{1}[-2]{abc} = h %v d %v, want %v and %v`, h, d, 2*d0, 2*h0)
	}
}

// \reflectbox is \scalebox{-1}[1], and it mirrors: each item carries the
// scale -1 in its transform. The source reads "scale(-1" in the SVG text.
func TestReflectboxIsScaleboxMinusOne(t *testing.T) {
	face := testFace(t)
	a := onFace(t, face, `\reflectbox{abc}`, 10, Text)
	b := onFace(t, face, `\scalebox{-1}[1]{abc}`, 10, Text)
	if !equalBoxes(a, b) {
		t.Error(`\reflectbox does not lay out as \scalebox{-1}[1]`)
	}
	for _, it := range a.Items {
		if it.Transform.SX != -1 || it.Transform.SY != 1 {
			t.Errorf(`\reflectbox: the transform %+v does not mirror`, it.Transform)
		}
	}
}

// A mirror about x = 0 puts the content left of the origin, so the transform
// moves it back by the scaled width (graphics.sty:534-536). The ink of the
// mirrored box is the ink of abc reflected in the middle of its width. A
// positive factor carries no translation.
func TestAMirroredBoxIsShiftedBackOverItsWidth(t *testing.T) {
	face := testFace(t)
	plain := onFace(t, face, `abc`, 10, Text)
	mirror := onFace(t, face, `\reflectbox{abc}`, 10, Text)
	for _, it := range mirror.Items {
		if it.Transform.TX != float32(plain.Width) {
			t.Errorf("the mirrored item moves by %v, want the width %v", it.Transform.TX, plain.Width)
		}
	}
	p, m := plain.Path().Bounds(), mirror.Path().Bounds()
	w := float32(plain.Width)
	if math.Abs(float64(m.Left()-(w-p.Right()))) > 1e-4 || math.Abs(float64(m.Right()-(w-p.Left()))) > 1e-4 {
		t.Errorf("the mirrored ink spans %v to %v, want %v to %v", m.Left(), m.Right(), w-p.Right(), w-p.Left())
	}
	for _, it := range onFace(t, face, `\scalebox{2}{abc}`, 10, Text).Items {
		if it.Transform.SX != 2 || it.Transform.SY != 2 || it.Transform.TX != 0 || it.Transform.TY != 0 {
			t.Errorf(`\scalebox{2}: the transform %+v, want a scale of 2 and no translation`, it.Transform)
		}
	}
}

// A scaled box placed in a list keeps its transform, and the place of the
// box goes after it: the second x of x\scalebox{2}{x} is scaled by 2 about
// the end of the first one.
func TestAScaledBoxIsPlacedAfterItsTransform(t *testing.T) {
	face := testFace(t)
	x := onFace(t, face, `x`, 10, Text)
	b := onFace(t, face, `x\scalebox{2}{x}`, 10, Text)
	g := itemsOf(b, ItemGlyph)
	if len(g) != 2 {
		t.Fatalf("%d glyphs, want 2", len(g))
	}
	tr := g[1].Transform
	if g[1].X != 0 || g[1].Y != 0 || tr.SX != 2 || tr.SY != 2 || tr.TX != float32(x.Width) || tr.TY != 0 {
		t.Errorf("the scaled x is at %v,%v with %+v, want 0,0 with a scale of 2 moved by %v", g[1].X, g[1].Y, tr, x.Width)
	}
	if b.Width != x.Width+float64(2*x.Width) {
		t.Errorf("the width is %v, want %v", b.Width, x.Width+float64(2*x.Width))
	}
}

// The optional factor is told from its absence: a [ scaled by 2 is twice as
// high as a [ scaled by 2 and 1.
func TestAnUnclosedBracketScalesTheBracket(t *testing.T) {
	_, h2, _ := scaleDims(t, `\scalebox{2}{[}`)
	_, h1, _ := scaleDims(t, `\scalebox{2}[1]{[}`)
	if h2 != 2*h1 {
		t.Errorf(`\scalebox{2}{[} is %v high, \scalebox{2}[1]{[} is %v`, h2, h1)
	}
	mustLayout(t, `\scalebox{2}[1`, 10, Text)
}
