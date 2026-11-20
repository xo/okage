// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"testing"
)

// This file ports the tests of mathchar_test.go of the source that lay out.
// TestMathCharMapsMinusAndAsterisk, which reads the parser, is in
// censussymbols_test.go.

// * sets as \ast, because fontmath.ltx points both at the same slot of cmsy
// (fontmath.ltx:150).
func TestAsteriskIsAst(t *testing.T) {
	if !widecheckSameItems(mustLayout(t, `a*b`, 10, Text), mustLayout(t, `a\ast b`, 10, Text)) {
		t.Error(`* does not set as \ast`)
	}
}

// The minus sign matches the plus: the same advance. A text hyphen is
// narrower, so a-b was narrower than a+b, where TeX sets the two to the same
// width.
func TestMinusMatchesPlus(t *testing.T) {
	minus := mustLayout(t, `a-b`, 10, Text)
	plus := mustLayout(t, `a+b`, 10, Text)
	if minus.Width != plus.Width {
		t.Errorf("a-b is %v wide, a+b %v: the minus sign matches the plus", minus.Width, plus.Width)
	}
}

// The space follows from the class, so the class is measured through the
// space it gives: a group is an Ord, so a{x}b has no space, and the
// difference with axb is the space of the class (tex.web:15062: Ord-Rel is
// thick, Bin has a medium space on each side, Ord-Ord none). thin is 3mu,
// med 4mu and thick 5mu (tex.web §764). A mu is the size over 18.
//
// The source compares with a tolerance of 0.001pt for its rounding. Here the
// difference is exact.
func TestClassSpacingAgainstABracedAtom(t *testing.T) {
	const size = 10
	mu := float64(size) / 18
	for _, c := range []struct {
		tex, braced string
		want        float64 // the space that the two joints add
		why         string
	}{
		{`a/b`, `a{/}b`, 0, "/ is Ord (fontmath.ltx:171): a/b sets as one word"},
		{`a:b`, `a{:}b`, 2 * 5 * mu, ": is Rel (fontmath.ltx:155): a thick space on each side"},
		{`a+b`, `a{+}b`, 2 * 4 * mu, "+ is Bin (fontmath.ltx:151): a medium space on each side"},
		{`a-b`, `a{-}b`, 2 * 4 * mu, "- is Bin (fontmath.ltx:153): a medium space on each side"},
		{`a,b`, `a{,}b`, 3 * mu, ", is Punct (fontmath.ltx:147): a thin space after it only"},
		{`a;b`, `a{;}b`, 3 * mu, "; is Punct (fontmath.ltx:157): a thin space after it only"},
		{`a\in b`, `a{\in}b`, 2 * 5 * mu, `\in is Rel: a thick space on each side`},
		{`a\times b`, `a{\times}b`, 2 * 4 * mu, `\times is Bin: a medium space on each side`},
		{`n!`, `n{!}`, 0, "! is Close (fontmath.ltx:149): nothing after an Ord"},
		{`n?`, `n{?}`, 0, "? is Close (fontmath.ltx:169): nothing after an Ord"},
	} {
		spaced := mustLayout(t, c.tex, size, Text)
		tight := mustLayout(t, c.braced, size, Text)
		if got := spaced.Width - tight.Width; !near(got, c.want) {
			t.Errorf("%s adds %v over %s, want %v: %s", c.tex, got, c.braced, c.want, c.why)
		}
	}
}

// := sets tight: Rel-Rel is 0 in the table of spaces, so a colon that is a
// relation adds nothing between the two characters, and the thick space
// before it.
func TestColonEqualsSetsTight(t *testing.T) {
	coloneq := mustLayout(t, `a:=b`, 10, Text)
	eq := mustLayout(t, `a=b`, 10, Text)
	colon := mustLayout(t, `{:}`, 10, Text)
	if got := coloneq.Width - eq.Width - colon.Width; !near(got, 0) {
		t.Errorf("a:=b is %v wider than a=b and the colon. Rel-Rel is 0", got)
	}
}
