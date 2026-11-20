// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// The first tests read the rows of the tree. The tests after them measure
// the overlay and read the place of its glyphs, as the tests of the source
// read the translations of the SVG text. Each width is the advance of a
// glyph of the face, computed again from the font.

func overlay(t *testing.T, tex string) Overlay {
	t.Helper()
	o, ok := mustParse(t, tex)[0].(Overlay)
	if !ok {
		t.Fatalf("%s is not an Overlay", tex)
	}
	return o
}

func TestATrailingCrAddsNoRow(t *testing.T) {
	for _, c := range []struct {
		tex  string
		rows int
	}{
		{`\ooalign{a\cr b}`, 2},
		{`\ooalign{a\cr b\cr}`, 2},
		{`\ooalign{a\cr b\crcr}`, 2},
		{`\ooalign{a\cr\cr b}`, 2},
		{`\ooalign{a\crcr b\cr}`, 2},
		{`\ooalign{a}`, 1},
		{`\ooalign{a\cr b\cr c}`, 3},
		{`\ooalign{{ab}\cr c}`, 2},
		{`\ooalign{}`, 0},
	} {
		if got := len(overlay(t, c.tex).Rows); got != c.rows {
			t.Errorf("%s has %d rows, want %d", c.tex, got, c.rows)
		}
	}
}

func TestHidewidthMarksTheRow(t *testing.T) {
	for _, c := range []struct {
		tex         string
		left, right bool
	}{
		{`\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`, true, true},
		{`\ooalign{$\cup$\cr\hidewidth$\cdot$}`, true, false},
		{`\ooalign{$\cup$\cr$\cdot$}`, false, false},
	} {
		o := overlay(t, c.tex)
		if r := o.Rows[1]; r.HideLeft != c.left || r.HideRight != c.right {
			t.Errorf("%s: row 2 hides %v/%v, want %v/%v", c.tex, r.HideLeft, r.HideRight, c.left, c.right)
		}
		if r := o.Rows[0]; r.HideLeft || r.HideRight {
			t.Errorf("%s: row 1 is hidden", c.tex)
		}
	}
}

func TestABareHidewidthIsConsumedAndNotAHugeNegativeKern(t *testing.T) {
	l := mustParse(t, `a\hidewidth b`)
	if a, ok := l[1].(Atom); !ok || a != (Atom{Class: Ord}) {
		t.Errorf(`a\hidewidth b = %#v, want an empty Ord atom between a and b`, l)
	}
}

func TestOoalignWithoutAGroupIsAnError(t *testing.T) {
	for _, tex := range []string{
		`\ooalign`, `\ooalign a`, `\ooalign{a\cr b`,
		`\ooalign{a\cr\nosuchmaththing}`, `\ooalign{{a\cr b}\cr c}`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%s did not fail", tex)
		}
	}
}

func TestTheCorpusOoalignFormRenders(t *testing.T) {
	for _, tex := range []string{
		`\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`,
		`a\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}b`,
		`x^{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
		`\mathbin{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
		`\ooalign{\hidewidth$\cup$\hidewidth\cr\hidewidth$\cdot$\hidewidth}`,
	} {
		mustParse(t, tex)
	}
}

// ooDims returns the width, the height and the depth of tex at the size 32 in
// display style.
func ooDims(t *testing.T, face *font.Face, tex string) (w, h, d float64) {
	t.Helper()
	b := onFace(t, face, tex, 32, Display)
	return b.Width, b.Height, b.Depth
}

// The rows overlay: the width is the largest of the rows, never their sum,
// and so are the height and the depth.
func TestOoalignOverlaysRatherThanStacksSideways(t *testing.T) {
	face := testFace(t)
	wa, ha, da := ooDims(t, face, `a`)
	wm, hm, dm := ooDims(t, face, `\cup`)
	if cw, _, _ := glyphMetrics(t, face, '∪', 32); cw != wm {
		t.Fatalf(`\cup is %v wide, its glyph %v`, wm, cw)
	}
	w, h, d := ooDims(t, face, `\ooalign{a\cr\cup}`)
	if w != max(wa, wm) || w == wa+wm {
		t.Errorf("width %v, want %v, the wider of the two rows", w, max(wa, wm))
	}
	if h != max(ha, hm) || d != max(da, dm) {
		t.Errorf("extents h=%v d=%v, want %v and %v", h, d, max(ha, hm), max(da, dm))
	}
}

// Every row is at y = 0. That is the overlay.
func TestOoalignPutsEveryRowOnTheSameBaseline(t *testing.T) {
	g := itemsOf(mustLayout(t, `\ooalign{a\cr b\cr c}`, 32, Display), ItemGlyph)
	if len(g) != 3 {
		t.Fatalf("%d glyphs, want 3", len(g))
	}
	for i, it := range g {
		if it.Y != 0 || it.X != 0 {
			t.Errorf("row %d is at %v,%v: this is not an overlay", i, it.X, it.Y)
		}
	}
}

// A row with \hidewidth on each side adds nothing to the width, so the
// \cup with a \cdot on it measures as the \cup alone.
func TestHidewidthRemovesTheRowsWidthContribution(t *testing.T) {
	face := testFace(t)
	wc, hc, dc := ooDims(t, face, `\cup`)
	if w, h, d := ooDims(t, face, `\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`); w != wc || h != hc || d != dc {
		t.Errorf(`the overlay is %v×%v/%v, \cup alone %v×%v/%v`, w, h, d, wc, hc, dc)
	}
	wd, _, _ := ooDims(t, face, `\cdot`)
	if wd >= wc {
		t.Fatalf(`\cdot (%v) is not narrower than \cup (%v)`, wd, wc)
	}
	if w, _, _ := ooDims(t, face, `\ooalign{$\cdot$\cr$\cup$}`); w != wc {
		t.Errorf("an unhidden wider row gave %v, want %v", w, wc)
	}
	if w, _, _ := ooDims(t, face, `\ooalign{$\cdot$\cr\hidewidth$\cup$\hidewidth}`); w != wd {
		t.Errorf("a hidden wider row gave %v, want %v", w, wd)
	}
}

// \hidewidth on each side centers the row, and on the left only sets it at
// the right. The x of the glyph of the second row is (wc-wd)/2, wc-wd or 0,
// with the advances of the glyphs of the face.
func TestHidewidthCentresOrRightAlignsTheRow(t *testing.T) {
	face := testFace(t)
	wc, _, _ := glyphMetrics(t, face, '∪', 32)
	wd, _, _ := glyphMetrics(t, face, '⋅', 32)
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}`, (wc - wd) / 2},
		{`\ooalign{$\cup$\cr\hidewidth$\cdot$}`, wc - wd},
		{`\ooalign{$\cup$\cr$\cdot$}`, 0},
	} {
		g := itemsOf(onFace(t, face, c.tex, 32, Display), ItemGlyph)
		if len(g) != 2 || g[0].X != 0 || g[1].X != c.want {
			t.Errorf("%s: glyphs %+v, want the second at x = %v", c.tex, g, c.want)
		}
	}
}

// A trailing \cr adds no row: the box draws two glyphs, and measures as the
// overlay without it.
func TestATrailingCrMeasuresAlike(t *testing.T) {
	face := testFace(t)
	w1, h1, d1 := ooDims(t, face, `\ooalign{a\cr b}`)
	for _, tex := range []string{`\ooalign{a\cr b\cr}`, `\ooalign{a\cr b\crcr}`, `\ooalign{a\crcr b\cr}`, `\ooalign{a\cr\cr b}`} {
		b := onFace(t, face, tex, 32, Display)
		if b.Width != w1 || b.Height != h1 || b.Depth != d1 || len(b.Items) != 2 {
			t.Errorf("%s = %v×%v/%v with %d items, want %v×%v/%v with 2", tex, b.Width, b.Height, b.Depth, len(b.Items), w1, h1, d1)
		}
	}
}

// A bare \hidewidth is an empty Ord atom, as in the source, and not a kern
// of -1000pt: between two Ord atoms it adds no width.
func TestABareHidewidthAddsNoWidth(t *testing.T) {
	face := testFace(t)
	if got, want := onFace(t, face, `a\hidewidth b`, 32, Display).Width, onFace(t, face, `ab`, 32, Display).Width; got != want {
		t.Errorf(`a\hidewidth b is %v wide, ab %v`, got, want)
	}
}

// \ooalign{} is an empty box, and when every row is hidden the widest row
// sets the width.
func TestOoalignEdgeCasesInTheBox(t *testing.T) {
	face := testFace(t)
	if b := onFace(t, face, `\ooalign{}`, 32, Display); b.Width != 0 || b.Height != 0 || b.Depth != 0 || len(b.Items) != 0 {
		t.Errorf(`\ooalign{} = %+v, want an empty box`, b)
	}
	wc, _, _ := glyphMetrics(t, face, '∪', 32)
	if w, _, _ := ooDims(t, face, `\ooalign{\hidewidth$\cup$\hidewidth\cr\hidewidth$\cdot$\hidewidth}`); w != wc {
		t.Errorf("every row hidden gave the width %v, want %v", w, wc)
	}
	if g := itemsOf(onFace(t, face, `\ooalign{{ab}\cr c}`, 32, Display), ItemGlyph); len(g) != 3 {
		t.Errorf(`\ooalign{{ab}\cr c} draws %d glyphs, want 3`, len(g))
	}
	for _, tex := range []string{
		`a\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}b`,
		`x^{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
		`\mathbin{\ooalign{$\cup$\cr\hidewidth$\cdot$\hidewidth}}`,
	} {
		onFace(t, face, tex, 32, Display)
	}
}
