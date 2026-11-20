// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

// \ooalign{row\cr row\cr} is the overlay of plain TeX: each row is set on the
// same baseline, over the others. A paper draws a symbol that the fonts do
// not have with it, by stacking two that they have. latex.ltx gives it:
//
//	:492  \newskip\hideskip \hideskip=-1000pt plus 1fill  % negative but can grow
//	:615  \def\hidewidth{\hskip\hideskip}
//	:624  \def\ialign{\everycr{}\tabskip\z@skip\halign}
//	:625  \def\oalign#1{\leavevmode\vtop{\baselineskip\z@skip \lineskip.25ex%
//	        \ialign{##\crcr#1\crcr}}}
//	:628  \def\ooalign{\lineskiplimit-\maxdimen \oalign}
//
// The baseline skip is zero and the line skip limit is -\maxdimen, so every
// row lands on the same baseline. \o@lign and \oalign depend on the
// \lineskiplimit in force, which this package does not hold, so they are not
// here.
//
// A paper writes
//
//	\newcommand*{\@cupdot}[2]{\ooalign{$\m@th#1\cup$\cr \hidewidth$\m@th#1\cdot$\hidewidth}}
//
// which is a \cup with a \cdot centered on it.
//
// The layout half is layoutOverlay, the end of parseOoalign of the source.

// parseOoalign parses \ooalign{…}, after the command cmd.
func (p *parser) parseOoalign(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	o, err := p.ooalignRows(cmd, toks)
	if err != nil {
		return nil, nil, err
	}
	for i := range o.rows {
		b, _, err := p.parseList(o.inner(i), st, stopEnd)
		if err != nil {
			return nil, nil, err
		}
		o.out[i].Body = b
	}
	return o.node(p), o.rest, nil
}

// ooalignParts holds the rows of \ooalign as tokens, and the rows of the
// Overlay.
//
// okage: they are on the heap, so that a level of nesting takes little of
// the stack.
type ooalignParts struct {
	rows [][]token
	out  []OverlayRow
	rest []token // the tokens after the group
}

// ooalignRows reads the group of \ooalign and splits it into rows.
//
//go:noinline
func (p *parser) ooalignRows(cmd *token, toks []token) (*ooalignParts, error) {
	body, rest, ok := readGroupToks(toks)
	if !ok {
		return nil, p.fail([]token{*cmd}, `\ooalign needs a {…} group`)
	}
	rows := splitOoalignRows(body)
	return &ooalignParts{rows: rows, out: make([]OverlayRow, len(rows)), rest: rest}, nil
}

// inner returns the tokens of row i without its \hidewidth, and records the
// \hidewidth in the Overlay row.
//
//go:noinline
func (o *ooalignParts) inner(i int) []token {
	hl, hr, inner := stripHidewidth(o.rows[i])
	o.out[i].HideLeft, o.out[i].HideRight = hl, hr
	return inner
}

// node makes the Overlay of the rows.
//
//go:noinline
func (o *ooalignParts) node(p *parser) Node {
	var rows []OverlayRow
	if len(o.out) > 0 {
		rows = o.out
	}
	return p.add(Overlay{Rows: rows})
}

// splitOoalignRows splits on \cr and \crcr, at a brace depth of zero, and
// drops the empty rows, such as the one after a trailing \cr, which the
// template of \oalign makes.
//
// A \cr inside braces belongs to a nested alignment, which this package does
// not have, so such a row fails as an unknown \cr either way. The depth is
// kept, so that a nested alignment, when it comes, is not torn in half.
func splitOoalignRows(toks []token) [][]token {
	var rows [][]token
	depth, start := 0, 0
	for i, t := range toks {
		switch {
		case t.kind == tLBrace:
			depth++
		case t.kind == tRBrace:
			depth--
		case depth == 0 && t.kind == tCtrl && (t.text == "cr" || t.text == "crcr"):
			rows = append(rows, toks[start:i])
			start = i + 1
		}
	}
	if start < len(toks) {
		rows = append(rows, toks[start:])
	}
	out := rows[:0]
	for _, r := range rows {
		if len(r) > 0 {
			out = append(out, r)
		}
	}
	return out
}

// stripHidewidth removes a leading and a trailing \hidewidth, and says which
// were there.
func stripHidewidth(toks []token) (hideL, hideR bool, inner []token) {
	for len(toks) > 0 && toks[0].kind == tCtrl && toks[0].text == "hidewidth" {
		hideL = true
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].kind == tCtrl && toks[len(toks)-1].text == "hidewidth" {
		hideR = true
		toks = toks[:len(toks)-1]
	}
	return hideL, hideR, toks
}

// layoutOverlay lays out an Overlay. It is the end of parseOoalign of the
// source. The box is as wide as the widest row that is not hidden on both
// sides: \hidewidth is a skip of -1000pt that can grow without end, so a row
// with one on each side adds nothing to the width, and is centered. A row
// with \hidewidth on the left only is set at the right. When every row is
// hidden, the widest row sets the width. Each row is at a vertical offset of
// zero, which is the overlay. An Overlay with no rows is an empty Ord box.
//
//go:noinline
func (l *layout) layoutOverlay(n Node, st Style) *box {
	o, _ := n.(Overlay)
	rows := make([]*box, len(o.Rows))
	for i, r := range o.Rows {
		rows[i] = l.node(r.Body, st)
	}
	w := 0.0
	for i, r := range o.Rows {
		if !(r.HideLeft && r.HideRight) {
			w = max(w, rows[i].w)
		}
	}
	if w == 0 {
		for _, b := range rows {
			w = max(w, b.w)
		}
	}
	out := newBox(Ord)
	for i, r := range o.Rows {
		b := rows[i]
		x := 0.0
		switch {
		case r.HideLeft && r.HideRight:
			x = (w - b.w) / 2
		case r.HideLeft:
			x = w - b.w
		}
		out.h = max(out.h, b.h)
		out.d = max(out.d, b.d)
		place(out, b, x, 0)
	}
	out.w = w
	return out
}
