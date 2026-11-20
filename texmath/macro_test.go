// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/okage"
)

func TestDefMacros(t *testing.T) {
	for _, c := range [][2]string{
		{`\def\R{\mathbb{R}}\R`, `\mathbb{R}`},
		{`\def\R{\mathbb{R}}x \in \R^n`, `x \in \mathbb{R}^n`},
		{`\def\abs#1{\lvert #1 \rvert}\abs{x}`, `\lvert x \rvert`},
		{`\def\abs#1{\lvert #1 \rvert}\abs x`, `\lvert x \rvert`},
		{`\def\pair#1#2{(#1, #2)}\pair{a}{b}`, `(a, b)`},
		{`\def\a{\b}\def\b{x+y}\a`, `x+y`},
		{`\def\sq#1{#1^2}\frac{\sq{x}}{\sq{y}}`, `\frac{x^2}{y^2}`},
		{`\def\R{\mathbb{R}}\def\C{\mathbb{C}}\R\to\C`, `\mathbb{R}\to\mathbb{C}`},
		{`\def\g#1{#1_{ij}}\g{A} + \g{B}`, `A_{ij} + B_{ij}`},
		{`\def\x#1{#1#2}\x{a}`, `a`}, // a parameter with no argument gives nothing
		{`\def\h#1{##1}\h{a}`, `#a`},
	} {
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q does not parse as %q", c[0], c[1])
		}
	}
}

func TestDefUnresolvedIsLeftToParser(t *testing.T) {
	for name, tex := range map[string]string{
		"name not a control seq": `\def{x}{y} z`,
		"delimited parameter":    `\def\x a#1{#1} \x b`,
		"missing body":           `\def\x`,
		"unbalanced body":        `\def\x{a+b`,
		"too few arguments":      `\def\p#1#2{#1#2}\p{a}`,
		"unbalanced argument":    `\def\x#1{#1}\x{a+b`,
		"self-recursion halts":   `\def\x{\x}\x`,
	} {
		_, err := Parse(tex)
		if err == nil {
			t.Errorf("%q (%s): want an error, got none", tex, name)
			continue
		}
		if !errors.Is(err, okage.ErrTeXMathUnknownCommand) {
			t.Errorf("%q (%s): %v, want an unknown command", tex, name, err)
		}
	}
}

func TestExpandMacrosNoOpWithoutDef(t *testing.T) {
	p := &parser{cfg: newConfig(nil)}
	in := tokenize(`\frac{a}{b} + x^2`)
	out, err := p.expandMacros(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("expandMacros changed a stream without \\def: %d tokens in, %d out", len(in), len(out))
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("token %d changed: %+v → %+v", i, in[i], out[i])
		}
	}
}

// okage: a macro that grows the input stops at the limit of the tokens, with
// ErrMathLimit. \def\a{\a\a}\a spends the budget in time linear in the
// expansions, and is then an unknown command. The source copies the input at
// each expansion, in quadratic time.
func TestMacroGrowthStopsAtTheLimit(t *testing.T) {
	_, err := Parse(`\def\a{\a\a}\a`)
	if !errors.Is(err, okage.ErrTeXMathUnknownCommand) {
		t.Errorf(`\def\a{\a\a}\a: %v, want an unknown command`, err)
	}
	for _, tex := range []string{
		`\def\a#1{#1#1#1#1}\a{\a{\a{\a{\a{\a{\a{\a{\a{\a{\a{xxxxxxxxxxxxxxxx}}}}}}}}}}}`,
		`\def\b{` + strings.Repeat("x", 4000) + `}\def\a{\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b}\def\c{\a\a\a\a\a\a\a\a\a\a\a\a}\c`,
	} {
		_, err := Parse(tex)
		if !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("Parse(%.40q): %v, want ErrMathLimit", tex, err)
		}
	}
	// A budget of no expansions leaves the macro to the parser.
	_, err = Parse(`\def\R{x}\R`, WithMaxExpansions(0))
	if !errors.Is(err, okage.ErrTeXMathUnknownCommand) {
		t.Errorf("with no budget: %v, want an unknown command", err)
	}
}
