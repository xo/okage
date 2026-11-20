// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"fmt"
	"slices"
)

// This file gives the parser the "mouth" of TeX: a pass that runs the \def
// definitions and expands their uses before the parser sees the tokens. A
// paper writes shorthand such as \def\R{\mathbb{R}}\R, or
// \def\abs#1{\lvert#1\rvert}\abs{x}, inside a formula. The macros of a
// preamble are the work of the TeX engine.
//
// Only undelimited parameters, #1 to #9, are read. A \def whose parameter text
// has delimiters, or a use with too few arguments, is left as it is for the
// parser, which reports it.
//
// This file has no layout half.

// macroDef is one macro of \def: nargs undelimited parameters, and the body,
// in which a pair of tokens #n marks where argument n goes.
type macroDef struct {
	nargs int
	body  []token
}

// expandMacros runs the mouth pass. It reads the tokens from left to right,
// records each \def, and replaces each use of a macro with its body, with the
// arguments in place of #1 to #n. It reads the result again, so that nested
// and chained macros expand. A budget of expansions stops a macro that calls
// itself, such as \def\x{\x}: the name is then left for the parser.
//
// okage: the tokens still to read are a stack in reverse order, so an
// expansion costs the length of its body. The source copies the rest of the
// input at each expansion, in quadratic time. The input and its expansion
// have at most maxTokens tokens. The source has no such limit, so
// \def\a{\a\a}\a grew to millions of tokens.
func (p *parser) expandMacros(toks []token) ([]token, error) {
	macros := map[string]macroDef{}
	out := make([]token, 0, len(toks))
	budget := p.cfg.maxExpansions
	stack := make([]token, len(toks))
	for i, t := range toks {
		stack[len(toks)-1-i] = t
	}
	for len(stack) > 0 {
		top := len(stack) - 1
		t := stack[top]
		if t.kind == tCtrl && t.text == "def" {
			if def, name, n, ok := parseDef(stack[:top]); ok {
				macros[name] = def
				stack = stack[:top-n]
				continue
			}
			// A \def that the pass cannot read, such as one with delimited
			// parameters or no body, is left for the parser, which reports
			// it.
			out = append(out, t)
			stack = stack[:top]
			continue
		}
		if t.kind == tCtrl && isNewcommand(t.text) {
			// okage: \newcommand, \renewcommand and \providecommand define a
			// macro as \def does. The source has none of them.
			if def, name, n, ok := parseNewcommand(stack[:top]); ok {
				if _, defined := macros[name]; !defined || t.text != "providecommand" {
					macros[name] = def
				}
				stack = stack[:top-n]
				continue
			}
			out = append(out, t)
			stack = stack[:top]
			continue
		}
		if t.kind == tCtrl {
			m, ok := macros[t.text]
			if !ok {
				// okage: the macros of KaTeX, such as \R, unless the formula
				// defines the name.
				m, ok = katexMacro(t)
			}
			if ok && budget > 0 {
				if args, n, ok := readArgs(m, stack[:top]); ok {
					size := expansionSize(m, args)
					if len(out)+top-n+size > maxTokens {
						return nil, p.limit([]token{t}, fmt.Sprintf(`the expansion of \%s has more than %d tokens`, t.text, maxTokens))
					}
					budget--
					stack = pushExpansion(stack[:top-n], m, args)
					continue
				}
				// Too few arguments: the name is left for the parser.
			}
		}
		out = append(out, t)
		stack = stack[:top]
	}
	return out, nil
}

// parseDef reads a \def from the top of the stack st, just after the token
// \def: the name of the macro, a control sequence, its undelimited
// parameters, #1 to #n in order, and its {…} body. It returns the number of
// tokens that it read. ok is false for a name that is not a control
// sequence, a parameter text with anything but #1 to #n in order, or a body
// that is missing or not balanced.
func parseDef(st []token) (def macroDef, name string, n int, ok bool) {
	i := len(st) - 1
	if i < 0 || st[i].kind != tCtrl {
		return macroDef{}, "", 0, false
	}
	name = st[i].text
	i--
	nargs := 0
	for i >= 0 && st[i].kind != tLBrace {
		if st[i].kind == tChar && st[i].r == '#' && i > 0 &&
			st[i-1].kind == tChar && st[i-1].r == rune('1'+nargs) {
			nargs++
			i -= 2
			continue
		}
		return macroDef{}, "", 0, false
	}
	rev, g, gok := readGroupRev(st[:i+1])
	if !gok {
		return macroDef{}, "", 0, false
	}
	body := make([]token, len(rev))
	for j, t := range rev {
		body[len(rev)-1-j] = t
	}
	return macroDef{nargs: nargs, body: body}, name, len(st) - 1 - i + g, true
}

// isNewcommand reports whether name is \newcommand, \renewcommand or
// \providecommand.
func isNewcommand(name string) bool {
	return name == "newcommand" || name == "renewcommand" || name == "providecommand"
}

// parseNewcommand reads a \newcommand from the top of the stack st, just after
// the token of the command, as parseDef reads a \def: an optional *, the name
// of the macro, a control sequence alone or in braces, an optional number of
// parameters in brackets, from 0 to 9, and the {…} body. It returns the
// number of tokens that it read. ok is false for any other form, such as a
// default value of the first parameter, which the parser then reports as an
// unknown command.
func parseNewcommand(st []token) (def macroDef, name string, n int, ok bool) {
	i := len(st) - 1
	if i >= 0 && st[i].kind == tChar && st[i].r == '*' {
		i--
	}
	switch {
	case i >= 0 && st[i].kind == tCtrl:
		name = st[i].text
		i--
	case i >= 2 && st[i].kind == tLBrace && st[i-1].kind == tCtrl && st[i-2].kind == tRBrace:
		name = st[i-1].text
		i -= 3
	default:
		return macroDef{}, "", 0, false
	}
	nargs := 0
	if i >= 2 && st[i].kind == tChar && st[i].r == '[' {
		if st[i-1].kind != tChar || st[i-1].r < '0' || st[i-1].r > '9' || st[i-2].kind != tChar || st[i-2].r != ']' {
			return macroDef{}, "", 0, false
		}
		nargs = int(st[i-1].r - '0')
		i -= 3
	}
	rev, g, gok := readGroupRev(st[:i+1])
	if !gok {
		return macroDef{}, "", 0, false
	}
	body := make([]token, len(rev))
	for j, t := range rev {
		body[len(rev)-1-j] = t
	}
	return macroDef{nargs: nargs, body: body}, name, len(st) - 1 - i + g, true
}

// katexMacro returns the macro of KaTeX of the use t, with the position of t
// on each token of its body, and false for a name that katexMacros does not
// hold.
func katexMacro(t token) (macroDef, bool) {
	s, ok := katexMacros[t.text]
	if !ok {
		return macroDef{}, false
	}
	body := tokenize(s)
	for i := range body {
		body[i].pos, body[i].space = t.pos, false
	}
	return macroDef{body: body}, true
}

// katexMacros are the macros of KaTeX, from src/macros.js at v0.18.11, that
// stand for a symbol, a font or an operator of texmath, and the aliases of
// the commands of this package. They take no parameter of their own: a font,
// such as \bold, takes its argument after the expansion.
var katexMacros = map[string]string{
	// The sets of numbers.
	"N": `\mathbb{N}`, "R": `\mathbb{R}`, "Z": `\mathbb{Z}`,
	"natnums": `\mathbb{N}`, "reals": `\mathbb{R}`, "Reals": `\mathbb{R}`,
	"cnums": `\mathbb{C}`, "Complex": `\mathbb{C}`,
	// The Greek capitals that look like Latin ones.
	"Alpha": `\mathrm{A}`, "Beta": `\mathrm{B}`, "Epsilon": `\mathrm{E}`,
	"Zeta": `\mathrm{Z}`, "Eta": `\mathrm{H}`, "Iota": `\mathrm{I}`,
	"Kappa": `\mathrm{K}`, "Mu": `\mathrm{M}`, "Nu": `\mathrm{N}`,
	"Omicron": `\mathrm{O}`, "Rho": `\mathrm{P}`, "Tau": `\mathrm{T}`,
	"Chi": `\mathrm{X}`,
	// The other symbols.
	"alef": `\aleph`, "alefsym": `\aleph`, "real": `\Re`, "image": `\Im`,
	"weierp": `\wp`, "thetasym": `\vartheta`, "empty": `\emptyset`,
	"exist": `\exists`, "isin": `\in`, "infin": `\infty`, "sdot": `\cdot`,
	"Dagger": `\ddagger`, "clubs": `\clubsuit`, "diamonds": `\diamondsuit`,
	"hearts": `\heartsuit`, "spades": `\spadesuit`,
	"lt": `<`, "gt": `>`,
	// The arrows.
	"harr": `\leftrightarrow`, "hArr": `\Leftrightarrow`, "Harr": `\Leftrightarrow`,
	"larr": `\leftarrow`, "lArr": `\Leftarrow`, "Larr": `\Leftarrow`,
	"lrarr": `\leftrightarrow`, "lrArr": `\Leftrightarrow`, "Lrarr": `\Leftrightarrow`,
	"rarr": `\rightarrow`, "rArr": `\Rightarrow`, "Rarr": `\Rightarrow`,
	"uarr": `\uparrow`, "uArr": `\Uparrow`, "Uarr": `\Uparrow`,
	"darr": `\downarrow`, "dArr": `\Downarrow`, "Darr": `\Downarrow`,
	// The fonts.
	"bold": `\mathbf`, "Bbb": `\mathbb`, "frak": `\mathfrak`, "bm": `\boldsymbol`,
	"mathnormal": `\mathit`,
	// The operators.
	"argmax": `\operatorname*{arg\,max}`, "argmin": `\operatorname*{arg\,min}`,
	"plim": `\operatorname*{plim}`, "operatornamewithlimits": `\operatorname*`,
	// The aliases of this package.
	"underbar": `\underline`,
}

// readArgs reads the m.nargs arguments of a macro from the top of the stack
// st: a {group} or a single token each, as TeX takes them. Each argument is
// a copy, in reverse order. It returns the number of tokens that it read. ok
// is false when the tokens end before the arguments do.
func readArgs(m macroDef, st []token) (args [][]token, n int, ok bool) {
	args = make([][]token, m.nargs)
	top := len(st)
	for i := range m.nargs {
		if top == 0 {
			return nil, 0, false
		}
		if st[top-1].kind == tLBrace {
			rev, g, gok := readGroupRev(st[:top])
			if !gok {
				return nil, 0, false
			}
			args[i] = append([]token(nil), rev...)
			top -= g
		} else {
			args[i] = []token{st[top-1]}
			top--
		}
	}
	return args, len(st) - top, true
}

// isParam reports whether body[i] is the digit of a pair #n of the body of a
// macro, and returns n-1.
func isParam(body []token, i int) (int, bool) {
	b := body[i]
	if i > 0 && b.kind == tChar && b.r >= '1' && b.r <= '9' &&
		body[i-1].kind == tChar && body[i-1].r == '#' {
		return int(b.r - '1'), true
	}
	return 0, false
}

// expansionSize returns the number of tokens of the body of m with args in
// place.
func expansionSize(m macroDef, args [][]token) int {
	size := 0
	for i := len(m.body) - 1; i >= 0; i-- {
		if d, ok := isParam(m.body, i); ok {
			if d < len(args) {
				size += len(args[d])
			}
			i-- // the #
			continue
		}
		size++
	}
	return size
}

// pushExpansion pushes the body of m with args in place onto the stack st,
// so that its first token is on top. A #n with no argument n gives nothing,
// as in the source.
//
// The body is read from its end, so a pair #n is found from its digit. That
// gives the pairs of the source, which reads from the start, because the #
// of a pair is never a digit.
func pushExpansion(st []token, m macroDef, args [][]token) []token {
	for i := len(m.body) - 1; i >= 0; i-- {
		if d, ok := isParam(m.body, i); ok {
			if d < len(args) {
				st = append(st, args[d]...) // already in reverse order
			}
			i-- // the #
			continue
		}
		st = append(st, m.body[i])
	}
	return st
}

// readGroupRev reads the {…} group on top of the stack st. It returns the
// tokens inside the braces in reverse order, a part of st, and the number of
// tokens of the group with its braces. ok is false when st does not start
// with { or the group is not balanced. It is readGroup of the source, for a
// stack.
func readGroupRev(st []token) (inner []token, n int, ok bool) {
	if len(st) == 0 || st[len(st)-1].kind != tLBrace {
		return nil, 0, false
	}
	depth := 0
	for i, t := range slices.Backward(st) {
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth--; depth == 0 {
				return st[i+1 : len(st)-1], len(st) - i, true
			}
		}
	}
	return nil, 0, false
}
