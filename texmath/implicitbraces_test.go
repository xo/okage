// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"strings"
	"testing"
)

func TestImplicitBracesAreTheBraceTokens(t *testing.T) {
	for _, c := range []struct{ braces, implicit string }{
		{`x^{ab}`, `x^\bgroup ab\egroup`},
		{`\frac{a}{b}`, `\frac\bgroup a\egroup\bgroup b\egroup`},
		{`\sqrt{1+x}`, `\sqrt\bgroup 1+x\egroup`},
		{`{\bf ab}c`, `\bgroup\bf ab\egroup c`},
		{`\mathrm{d}x`, `\mathrm\bgroup d\egroup x`},
	} {
		if got, want := tokenKinds(c.implicit), tokenKinds(c.braces); got != want {
			t.Errorf("tokens(%q) = %v, tokens(%q) = %v", c.implicit, got, c.braces, want)
		}
	}
}

// tokenKinds spells the kinds and the texts of the tokens of s, without their
// positions and their spaces.
func tokenKinds(s string) string {
	var out strings.Builder
	for _, tk := range tokenize(s) {
		out.WriteRune(rune('A' + int(tk.kind)))
		out.WriteString(tk.text)
		if tk.r != 0 {
			out.WriteRune(tk.r)
		}
		out.WriteString(";")
	}
	return out.String()
}

func TestAnImplicitGroupScopesLikeABraceGroup(t *testing.T) {
	if !sameTree(t, `\bgroup\bf ab\egroup c`, `{\bf ab}c`) {
		t.Error("an implicit group does not parse as a brace group")
	}
	if sameTree(t, `\bgroup\bf ab\egroup c`, `\bgroup\bf ab c\egroup`) {
		t.Error(`\egroup did not end the group`)
	}
	_, err := Parse(`\bgroup\bf abc`)
	_, err2 := Parse(`{\bf abc`)
	var a, b *SyntaxError
	if !errors.As(err, &a) || !errors.As(err2, &b) || a.Msg != b.Msg {
		t.Errorf(`\bgroup gives %v, { gives %v: they must fail alike`, err, err2)
	}
}

func TestImplicitBracesRenderWhereAPaperWritesThem(t *testing.T) {
	for _, tex := range []string{
		`\bgroup a+b\egroup`,
		`x_\bgroup i,j\egroup`,
		`\left(\bgroup a\egroup\right)`,
		`\bgroup\bgroup a\egroup\egroup`,
	} {
		mustParse(t, tex)
	}
}
