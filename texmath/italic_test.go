package texmath

import (
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// This file tests the italic correction, the top accent attachment and the
// math kerning of the MATH table, which phase 26 of D78 adds, and the options
// that turn each one off.

// glyphItem returns the glyph item of b whose glyph is g, and fails when there
// is none.
func glyphItem(tb testing.TB, b *Box, g font.GID) Item {
	tb.Helper()
	for _, it := range b.Items {
		if it.Kind == ItemGlyph && it.Glyph == g {
			return it
		}
	}
	tb.Fatalf("no item of the glyph %d in %+v", g, b.Items)
	return Item{}
}

// TestItalicCorrection checks the italic correction of a superscript, of the
// scripts and the limits of an operator, of a list, and of Box.Italic.
func TestItalicCorrection(t *testing.T) {
	const size = 10
	face := testFace(t)
	ss := scriptSize(face, size)
	f := mathItalic('f')
	wf, _, _ := glyphMetrics(t, face, f, size)
	icf := italicOf(t, face, f, size)
	if icf == 0 {
		t.Fatal("the italic f of STIX Two Math has no italic correction")
	}
	two := glyphOf(t, face, '2')
	if got := glyphItem(t, mustLayout(t, `f^2`, size, Text), two).X; !near(got, wf+icf) {
		t.Errorf("f^2: the superscript is at %v, want %v", got, wf+icf)
	}
	// The math kerning moves the subscript of f to the left, so it is off
	// here.
	sub, err := LayoutTeX(`f_2`, face, size, Text, WithMathKern(false))
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphItem(t, sub, two).X; !near(got, wf) {
		t.Errorf("f_2: the subscript is at %v, want %v", got, wf)
	}
	off, err := LayoutTeX(`f^2`, face, size, Text, WithItalicCorrection(false))
	if err != nil {
		t.Fatal(err)
	}
	if got := glyphItem(t, off, two).X; !near(got, wf) {
		t.Errorf("f^2 without the italic correction: the superscript is at %v, want %v", got, wf)
	}
	// A list ends with the correction, and Box.Italic gives it.
	b := mustLayout(t, `f`, size, Text)
	if !near(b.Width, wf+icf) || !near(b.Italic, icf) {
		t.Errorf("f is %v wide with the italic correction %v, want %v and %v", b.Width, b.Italic, wf+icf, icf)
	}
	if b, err := LayoutTeX(`f`, face, size, Text, WithItalicCorrection(false)); err != nil || b.Width != wf || b.Italic != 0 {
		t.Errorf("f without the italic correction: %+v, %v", b, err)
	}
	if b := mustLayout(t, `f+`, size, Text); b.Italic != 0 {
		t.Errorf("f+ has the italic correction %v, want that of +", b.Italic)
	}
	// The subscript of an integral is under its overhang, and the
	// superscript the correction to the right.
	b = mustLayout(t, `\int_0^1`, size, Text)
	wi, _, _ := glyphMetrics(t, face, '∫', size)
	ici := italicOf(t, face, '∫', size)
	if x := glyphItem(t, b, glyphOf(t, face, '0')).X; !near(x, wi) {
		t.Errorf(`\int_0^1: the subscript is at %v, want %v`, x, wi)
	}
	if x := glyphItem(t, b, glyphOf(t, face, '1')).X; !near(x, wi+ici) {
		t.Errorf(`\int_0^1: the superscript is at %v, want %v`, x, wi+ici)
	}
	// The limits of an integral are half of the correction apart from the
	// middle.
	b = mustLayout(t, `\int\limits_0^1`, size, Text)
	w0, _, _ := glyphMetrics(t, face, '0', ss)
	w1, _, _ := glyphMetrics(t, face, '1', ss)
	if x := glyphItem(t, b, glyphOf(t, face, '0')).X; !near(x, (b.Width-w0)/2-ici/2) {
		t.Errorf(`\int\limits_0^1: the lower limit is at %v, want %v`, x, (b.Width-w0)/2-ici/2)
	}
	if x := glyphItem(t, b, glyphOf(t, face, '1')).X; !near(x, (b.Width-w1)/2+ici/2) {
		t.Errorf(`\int\limits_0^1: the upper limit is at %v, want %v`, x, (b.Width-w1)/2+ici/2)
	}
}

// TestAccentAttachment checks that an accent takes the combining mark, and
// that its attachment point is over the attachment point of the nucleus.
func TestAccentAttachment(t *testing.T) {
	const size = 10
	face := testFace(t)
	s := size / float64(face.Upem())
	f := glyphOf(t, face, mathItalic('f'))
	hat := glyphOf(t, face, 0x302)
	fa, ok1 := face.MathTopAccentAttachment(f)
	ha, ok2 := face.MathTopAccentAttachment(hat)
	if !ok1 || !ok2 {
		t.Fatal("STIX Two Math has no top accent attachment for f or U+0302")
	}
	b := mustLayout(t, `\hat{f}`, size, Text)
	if x, want := glyphItem(t, b, hat).X, float64(float64(fa)*s)-float64(float64(ha)*s); !near(x, want) {
		t.Errorf(`\hat{f}: the accent is at %v, want %v`, x, want)
	}
	// Off, the spacing accent is centered on the nucleus, as in the source.
	off, err := LayoutTeX(`\hat{f}`, face, size, Text, WithAccentAttachment(false))
	if err != nil {
		t.Fatal(err)
	}
	spacing := glyphOf(t, face, 'ˆ')
	wh, _, _ := gidMetrics(face, spacing, size)
	if x := glyphItem(t, off, spacing).X; !near(x, (off.Width-wh)/2) {
		t.Errorf(`\hat{f} without the attachment: the accent is at %v, want %v`, x, (off.Width-wh)/2)
	}
	// A nucleus of more than one glyph has its attachment in the middle.
	b = mustLayout(t, `\vec{ab}`, size, Text)
	vec := glyphOf(t, face, '⃗')
	va, _ := face.MathTopAccentAttachment(vec)
	if x, want := glyphItem(t, b, vec).X, b.Width/2-float64(float64(va)*s); !near(x, want) {
		t.Errorf(`\vec{ab}: the accent is at %v, want %v`, x, want)
	}
}

// TestMathKern checks that the math kerning of the corners moves a subscript
// under a slanted capital, and that the option turns it off.
func TestMathKern(t *testing.T) {
	const size = 10
	face := testFace(t)
	s := size / float64(face.Upem())
	ss := scriptSize(face, size)
	V := mathItalic('V')
	g := glyphOf(t, face, V)
	wv, _, dv := glyphMetrics(t, face, V, size)
	one := glyphOf(t, face, '1')
	_, h1, _ := glyphMetrics(t, face, '1', ss)
	b := mustLayout(t, `V_1`, size, Text)
	it := glyphItem(t, b, one)
	v := it.Y
	kern := func(h float64) float64 {
		return float64(float64(face.MathKern(g, font.MathKernBottomRight, float32(h/s))) * s)
	}
	want := wv + min(kern(h1-v), kern(-dv))
	if want >= wv {
		t.Fatal("STIX Two Math has no kern under the italic V")
	}
	if !near(it.X, want) {
		t.Errorf("V_1: the subscript is at %v, want %v", it.X, want)
	}
	off, err := LayoutTeX(`V_1`, face, size, Text, WithMathKern(false))
	if err != nil {
		t.Fatal(err)
	}
	if x := glyphItem(t, off, one).X; !near(x, wv) {
		t.Errorf("V_1 without the math kerning: the subscript is at %v, want %v", x, wv)
	}
}
