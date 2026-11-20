// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import "testing"

// fractionRule returns the rule of a fraction: the one rule item of b.
func fractionRule(t *testing.T, tex string, b *Box) Item {
	t.Helper()
	rules := itemsOf(b, ItemRule)
	if len(rules) != 1 {
		t.Fatalf("%s: %d rules, want 1", tex, len(rules))
	}
	return rules[0]
}

// TeX puts beside a fraction the two delimiters that its noad names, and
// \frac names none: var_delimiter then returns an empty box of the width
// \nulldelimiterspace (tex.web:13930), and the two boxes go outside the vlist
// that holds the rule. The rule is in that vlist, so it takes its width: the
// wider of the numerator and the denominator. The padding stays blank on
// both sides.
//
// The source reads the widest <rect> of the SVG text. Here the rule is the
// one rule item, and the values are exact: \nulldelimiterspace is 0.12 em,
// one on each side, and the rule is as wide as the wider part, with the
// widths of the glyphs from the font.
func TestFractionRuleSpansTheContentNotThePadding(t *testing.T) {
	const size = 10
	const want = 2 * 1.2 // 2 x \nulldelimiterspace, 0.12 em at 10
	face := testFace(t)
	for _, tex := range []string{`\frac{a}{b}`, `\frac{a+b}{c+d}`, `\frac{x}{yyyyy}`, `\tfrac{a}{b}`} {
		b := mustLayout(t, tex, size, Text)
		r := fractionRule(t, tex, b)
		if got := b.Width - r.W; !near(got, want) {
			t.Errorf("%s: box %v, rule %v: the blank is %v, want %v", tex, b.Width, r.W, got, want)
		}
		// It is centered: the blank is the same on each side.
		if got := b.Width - r.W - r.X; !near(got, r.X) {
			t.Errorf("%s: the rule starts at %v and leaves %v on the right", tex, r.X, got)
		}
	}
	// \frac{x}{yyyyy}: the rule is as wide as the five letters y of the
	// denominator, at the script size of a fraction in text style.
	ss := scriptSize(face, size)
	wy, _, _ := glyphMetrics(t, face, mathItalic('y'), ss)
	wy += italicOf(t, face, mathItalic('y'), ss) // the italic correction follows each y
	r := fractionRule(t, `\frac{x}{yyyyy}`, mustLayout(t, `\frac{x}{yyyyy}`, size, Text))
	if want := wy + wy + wy + wy + wy; !near(r.W, want) {
		t.Errorf(`\frac{x}{yyyyy}: the rule is %v wide, want %v`, r.W, want)
	}
}

// The padding is a constant of the size, not a part of the content: two
// fractions of very different widths leave the same blank.
func TestFractionPaddingDoesNotScaleWithContent(t *testing.T) {
	blank := func(tex string) float64 {
		b := mustLayout(t, tex, 10, Text)
		return b.Width - fractionRule(t, tex, b).W
	}
	narrow, wide := blank(`\frac{a}{b}`), blank(`\frac{abcdefgh}{ijklmnop}`)
	if !near(narrow, wide) {
		t.Errorf("a narrow fraction leaves %v blank and a wide one %v", narrow, wide)
	}
}
