// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// The tests of the source that read the SVG text count the <rect> elements
// of the rules and the <path> elements of the glyphs. The tests here count
// the items of the kinds ItemRule and ItemGlyph.

// fraction returns the Fraction of tex, and the style around it, or -1.
func fraction(t *testing.T, tex string) (Fraction, int) {
	t.Helper()
	n := mustParse(t, tex)[0]
	style := -1
	if s, ok := n.(StyleChange); ok {
		style, n = int(s.Style), s.Body
	}
	f, ok := n.(Fraction)
	if !ok {
		t.Fatalf("%s is %#v, not a Fraction", tex, n)
	}
	return f, style
}

func TestGenfrac(t *testing.T) {
	for _, c := range []struct {
		tex         string
		noRule      bool
		open, close rune
		style       int
	}{
		{`\genfrac[]{0pt}{}{n}{k}`, true, '[', ']', -1},
		{`\genfrac{\{}{\}}{0pt}{}{n}{k}`, true, '{', '}', -1},
		{`\genfrac{}{}{}{}{a}{b}`, false, 0, 0, -1},
		{`\genfrac{}{}{}0{a}{b}`, false, 0, 0, int(Display)},
		{`\genfrac()\z@{2}{a}{b}`, true, '(', ')', int(Script)},
		{`\genfrac{}{}{}{1}{a}{b}`, false, 0, 0, int(Text)},
		// okage: style 3 is scriptscript. The source sets it as script.
		{`\genfrac{}{}{}{3}{a}{b}`, false, 0, 0, int(ScriptScript)},
		{`\genfrac{}{}{0.0pt}{}{n}{k}`, true, 0, 0, -1},
		{`\genfrac{}{}{0}{}{n}{k}`, true, 0, 0, -1},
		{`\genfrac{}{}{1pt}{}{n}{k}`, false, 0, 0, -1},
	} {
		f, style := fraction(t, c.tex)
		if f.NoRule != c.noRule || f.Open != c.open || f.Close != c.close || style != c.style {
			t.Errorf("%s = %#v in style %d, want rule %v, %q %q, style %d",
				c.tex, f, style, !c.noRule, c.open, c.close, c.style)
		}
	}
	if !sameTree(t, `\binom{n}{k}`, `\genfrac(){0pt}{}{n}{k}`) {
		t.Error(`\binom is not \genfrac(){0pt}{}`)
	}
	if !sameTree(t, `{n \choose k}`, `{\binom{n}{k}}`) {
		t.Error(`\choose is not \binom`)
	}
}

func TestGenfracMalformed(t *testing.T) {
	for _, c := range []struct{ name, tex string }{
		{"nothing at all", `\genfrac`},
		{"unknown left delimiter", `\genfrac{\foo}{}{}{}{a}{b}`},
		{"unknown right delimiter", `\genfrac{}{\foo}{}{}{a}{b}`},
		{"unclosed brace on the delimiter", `\genfrac{(`},
		{"left delimiter alone", `\genfrac(`},
		{"missing numerator", `\genfrac{}{}{}{}`},
		{"missing denominator", `\genfrac{}{}{}{}{a}`},
	} {
		if _, err := Parse(c.tex); err == nil {
			t.Errorf("%s: %q parsed without error", c.name, c.tex)
		}
	}
	// okage: a thickness past the limit of a length is ErrMathLimit.
	for _, tex := range []string{`\genfrac{}{}{1e400pt}{}{a}{b}`, `\genfrac{}{}{99999pt}{}{a}{b}`} {
		if _, err := Parse(tex); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("%s: %v, want ErrMathLimit", tex, err)
		}
	}
}

func TestReadArgText(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`{0pt}`, "0pt"},
		{`{{0pt}}`, "0pt"},
		{`{0pt`, "0pt"},
		{`0`, "0"},
	} {
		if got, _ := readArgText(tokenize(c.src)); got != c.want {
			t.Errorf("readArgText(%q) = %q, want %q", c.src, got, c.want)
		}
	}
	if got, rest := readArgText(nil); got != "" || rest != nil {
		t.Errorf("readArgText(nil) = %q, %v; want \"\", nil", got, rest)
	}
}

func TestDfracAndTfracAreStyledFractions(t *testing.T) {
	if _, s := fraction(t, `\dfrac{a}{b}`); s != int(Display) {
		t.Errorf(`\dfrac style %d`, s)
	}
	if _, s := fraction(t, `\tfrac{a}{b}`); s != int(Text) {
		t.Errorf(`\tfrac style %d`, s)
	}
	if f, _ := fraction(t, `a \atop b`); !f.NoRule || f.Open != 0 {
		t.Errorf(`\atop = %#v`, f)
	}
}

// \triangleleft and \triangleright are binary operators at U+25C1 and
// U+25B7 (unicode-math-table.tex:704/694, fontmath.ltx:264-265). They draw
// the two operands and the triangle.
func TestTriangleBinaryOperators(t *testing.T) {
	for _, tex := range []string{`x \triangleright y`, `x \triangleleft y`} {
		if n := len(itemsOf(mustLayout(t, tex, 12, Text), ItemGlyph)); n < 3 {
			t.Errorf("%q drew %d glyphs, want the two operands and the triangle", tex, n)
		}
	}
}

// \genfrac draws the numerator and the denominator, and a rule for an empty
// thickness only: 0pt is no rule, which is how \binom is defined.
func TestGenfracLayout(t *testing.T) {
	for _, c := range []struct{ name, tex string }{
		{"brackets, no rule", `\genfrac[]{0pt}{}{n}{k}`},
		{"braces, no rule", `\genfrac{\{}{\}}{0pt}{}{n}{k}`},
		{"no delimiter, rule", `\genfrac{}{}{}{}{a}{b}`},
		{"display style", `\genfrac{}{}{}0{a}{b}`},
		{"style script", `\genfrac()\z@{2}{a}{b}`},
	} {
		if n := len(itemsOf(mustLayout(t, c.tex, 12, Text), ItemGlyph)); n < 2 {
			t.Errorf("%s: %d glyphs, want at least the numerator and the denominator", c.name, n)
		}
	}
	withRule := len(itemsOf(mustLayout(t, `\genfrac{}{}{}{}{n}{k}`, 12, Text), ItemRule))
	noRule := len(itemsOf(mustLayout(t, `\genfrac{}{}{0pt}{}{n}{k}`, 12, Text), ItemRule))
	if withRule != 1 || noRule != 0 {
		t.Errorf("%d rules with an empty thickness, %d with 0pt: want 1 and 0", withRule, noRule)
	}
}

// The style is the digit of TeX: 0 display, 1 text, 2 script and 3
// scriptscript. A fraction in text style is smaller than the same one in
// display style, which is why \dfrac and \tfrac are \genfrac{}{}{}0 and
// \genfrac{}{}{}1 (amsmath.sty:238-239).
//
// okage: style 3 is scriptscript, so it is smaller than style 2. The source
// sets it in script style, and only checks that it is not larger.
func TestGenfracStyles(t *testing.T) {
	var h [4]float64
	for i, sty := range []string{"0", "1", "2", "3"} {
		b := mustLayout(t, `\genfrac{}{}{}`+sty+`{n}{k}`, 12, Text)
		h[i] = b.Height + b.Depth
	}
	if !(h[0] > h[1] && h[1] > h[2] && h[2] > h[3]) {
		t.Errorf("heights %v: each style must be smaller than the one before", h)
	}
}

// \z@ is the zero of amsmath, which \binom passes (\genfrac()\z@{},
// amsmath.sty:240). Each spelling of zero draws no rule.
func TestGenfracZeroThicknessSpellings(t *testing.T) {
	for _, zero := range []string{`{0pt}`, `{0}`, `{0.0pt}`, `\z@`} {
		b := mustLayout(t, `\genfrac{}{}`+zero+`{}{n}{k}`, 12, Text)
		if n := len(itemsOf(b, ItemRule)); n != 0 {
			t.Errorf("thickness %s: %d rules, want none", zero, n)
		}
	}
}

// A fraction in text style sets its parts one style down, in script style
// (Appendix G, rule 15b), so it is shorter and narrower than the same
// fraction in display style.
//
// okage: the source also checks that the inline fraction fits in the leading
// of 13.6pt of 11pt text. With the shifts of Appendix G, the numerator of a
// fraction in text style is at least FractionNumeratorShiftUp over the
// baseline, and the denominator FractionDenominatorShiftDown under it, which
// are 0.585 em in STIX Two Math. So the fraction is 16.65pt tall at 11pt, and
// does not fit. The test checks the exact metrics, computed again from the
// font. The source takes the gaps only, and its fraction fits.
func TestInlineFractionIsShorterThanDisplay(t *testing.T) {
	face := testFace(t)
	const size = 11.0
	inl := mustLayout(t, `\frac{a}{b}`, size, Text)
	dis := mustLayout(t, `\frac{a}{b}`, size, Display)
	if hi, hd := inl.Height+inl.Depth, dis.Height+dis.Depth; hi >= hd {
		t.Errorf("heights %.3f (inline) and %.3f (display): an inline fraction must be shorter", hi, hd)
	}
	if inl.Width >= dis.Width {
		t.Errorf("widths %.3f (inline) and %.3f (display): the parts must shrink", inl.Width, dis.Width)
	}
	ss := scriptSize(face, size)
	nw, nh, nd := glyphMetrics(t, face, mathItalic('a'), ss)
	dw, dh, dd := glyphMetrics(t, face, mathItalic('b'), ss)
	axis := mathConst(face, font.AxisHeight, size)
	rt := mathConst(face, font.FractionRuleThickness, size)
	up := max(mathConst(face, font.FractionNumeratorShiftUp, size), axis+rt/2+mathConst(face, font.FractionNumeratorGapMin, size)+nd)
	down := max(mathConst(face, font.FractionDenominatorShiftDown, size), -axis+rt/2+mathConst(face, font.FractionDenominatorGapMin, size)+dh)
	if !near(inl.Height, up+nh) || !near(inl.Depth, down+dd) {
		t.Errorf("inline fraction %.6f/%.6f, want %.6f/%.6f", inl.Height, inl.Depth, up+nh, down+dd)
	}
	if want := max(nw, dw) + float64(2*float64(size*0.12)); !near(inl.Width, want) {
		t.Errorf("inline width %.6f, want %.6f", inl.Width, want)
	}
}
