// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

// \mathchoice{display}{text}{script}{scriptscript} is a primitive of TeX that
// sets one of four bodies, chosen by the style in force, and drops the other
// three. Two uses in the kernel give the order:
//
//	latex.ltx:11179-11184  \def\mathpalette#1#2{\mathchoice
//	                         {#1\displaystyle{#2}}{#1\textstyle{#2}}
//	                         {#1\scriptstyle{#2}}{#1\scriptscriptstyle{#2}}}
//	amsmath.sty:266-270    \mathchoice{…\textfont…}{…\textfont…}
//	                         {…\scriptfont…}{…\scriptscriptfont…}
//
// amsmath's own \intkern@ is \mathchoice{\mkern-3mu}{}{}{}, and a paper writes
// \mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}: the branch that display style
// takes is often empty.
//
// The layout half is layoutChoice and Choice.branch, which are
// mathchoiceBranch and mathchoiceEmptyBranch of the source, with the end of
// parseMathchoice. An empty branch makes no atom at all, because an empty Ord
// atom changes the space between its neighbors: list in layout.go skips it.
// A script after an empty branch then attaches to an empty nucleus, as in the
// source. A branch that is not empty is an Ord atom.

// parseMathchoice parses the four groups of \mathchoice, after the command
// cmd. Fewer than four groups is an error, not a guess of the branches.
//
// okage: each branch is parsed, because the style is not known before the
// layout. The source parses the chosen branch only, so a fault in another
// branch was not reported.
func (p *parser) parseMathchoice(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	c, err := p.mathchoiceGroups(cmd, toks)
	if err != nil {
		return nil, nil, err
	}
	for i := range c.groups {
		b, _, err := p.parseList(c.groups[i], st, stopEnd)
		if err != nil {
			return nil, nil, err
		}
		c.branches[i] = b
	}
	return c.node(p), c.rest, nil
}

// mathchoiceParts holds the four groups of \mathchoice and their branches.
//
// okage: they are on the heap, so that a level of nesting takes little of
// the stack.
type mathchoiceParts struct {
	groups   [4][]token
	branches [4]List
	rest     []token // the tokens after the groups
}

// mathchoiceGroups reads the four groups of \mathchoice as tokens.
//
//go:noinline
func (p *parser) mathchoiceGroups(cmd *token, toks []token) (*mathchoiceParts, error) {
	c := &mathchoiceParts{rest: toks}
	for i := range c.groups {
		g, r, ok := readGroupToks(c.rest)
		if !ok && len(c.rest) > 0 && c.rest[0].kind != tRBrace && c.rest[0].kind != tLBrace {
			// okage: a group of one token, as KaTeX reads \mathchoice{a}{b}{c}x
			// (D95).
			g, r, ok = c.rest[:1], c.rest[1:], true
		}
		if !ok {
			return nil, p.fail([]token{*cmd}, `\mathchoice needs four {…} groups`)
		}
		c.groups[i], c.rest = g, r
	}
	return c, nil
}

// node makes the Choice of the four branches.
//
//go:noinline
func (c *mathchoiceParts) node(p *parser) Node {
	return p.add(Choice{
		Display:      c.branches[0],
		Text:         c.branches[1],
		Script:       c.branches[2],
		ScriptScript: c.branches[3],
	})
}

// readGroupToks reads one {group} as tokens, balanced, and returns what
// follows. It returns the contents without the braces, and false when the
// next token is not { or the group does not close.
func readGroupToks(toks []token) (group, rest []token, ok bool) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, toks, false
	}
	depth := 0
	for i, t := range toks {
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			depth--
			if depth == 0 {
				return toks[1:i], toks[i+1:], true
			}
		}
	}
	return nil, toks, false
}

// branch returns the branch of the style st. It is mathchoiceBranch of the
// source: 0 in display style, 1 in text style, 2 in script style and 3 in
// scriptscript style. The cramped styles of TeX choose as the others do, and
// the source has none.
func (c Choice) branch(st Style) List {
	switch st {
	case Display:
		return c.Display
	case Text:
		return c.Text
	case Script:
		return c.Script
	}
	return c.ScriptScript
}

// layoutChoice lays out the branch of the style st as an Ord atom.
//
//go:noinline
func (l *layout) layoutChoice(node Node, st Style) *box {
	n, _ := node.(Choice)
	b := l.node(n.branch(st), st)
	b.cls = Ord
	return b
}
