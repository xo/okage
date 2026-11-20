// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

// \raisebox{lift}[height][depth]{content}, from latex.ltx:11913-11935:
//
//	11913  \DeclareRobustCommand\raisebox[1]{\leavevmode
//	11915    \@ifnextchar[{\@rsbox{#1}}{\@irsbox{#1}[]}}
//	11918  \long\def\@irsbox#1[#2]#3{\@begin@tempboxa\hbox{#3}%
//	11922    \setbox\@tempboxa\hbox{\raise\@tempdima\box\@tempboxa}%   RAISE first
//	11923    \ifx\\#2\\\else\ht\@tempboxa\@tempdimb\fi                 then OVERRIDE
//	11926  \long\def\@iirsbox#1[#2][#3]#4{…
//	11932    \ht\@tempboxa\@tempdimb
//	11933    \dp\@tempboxa\dimen@                                      both, two-option
//
// The content moves up by lift, and down for a negative lift. The width does
// not change. A height or a depth replaces the extent of the moved box, and
// is not added to it. With one optional argument only the height is
// replaced, and the depth keeps what the raise left.
//
// The layout half is raiseBox.

// parseRaisebox parses \raisebox{lift}[height][depth]{content}.
//
// The lift is required: latex.ltx declares \raisebox[1], so a missing group is
// an error, not a shift of zero.
//
// okage: a length past the limit of a length is an error.
func (p *parser) parseRaisebox(_ *token, toks []token, st alphabet) (Node, []token, error) {
	n, rest, err := p.raiseboxHead(toks)
	if err != nil {
		return nil, nil, err
	}
	b, r3, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	n.Body = b
	return p.add(*n), r3, nil
}

// raiseboxHead reads the lift, the height and the depth of \raisebox, and
// returns the Raise without its body and the tokens of the body.
//
// okage: it is not inlined, so that its frame is not on the stack while
// parseRaisebox parses the body.
//
//go:noinline
func (p *parser) raiseboxHead(toks []token) (*Raise, []token, error) {
	ls, rest, err := p.readOpName(toks)
	if err != nil {
		return nil, nil, p.fail(toks, `\raisebox needs {lift}`)
	}
	n := &Raise{Lift: readDimen(ls)}
	for i := range 2 {
		if len(rest) == 0 || rest[0].kind != tChar || rest[0].r != '[' {
			break
		}
		s, r2, ok := readToBracket(rest[1:])
		if !ok {
			// An unclosed [ is not an optional argument: it is a character,
			// and the content follows it.
			break
		}
		// LaTeX tells an empty bracket from an absent one: \@irsbox gets []
		// when no bracket was written, and \ifx\\#2\\ then keeps the extent.
		// So [] does not set the extent to zero.
		if s != "" {
			if i == 0 {
				n.Height, n.HasHeight = readDimen(s), true
			} else {
				n.Depth, n.HasDepth = readDimen(s), true
			}
		}
		rest = r2
	}
	for _, l := range []Length{n.Lift, n.Height, n.Depth} {
		if err := p.checkLength(toks, `\raisebox`, l); err != nil {
			return nil, nil, err
		}
	}
	return n, rest, nil
}

// layoutRaise lays out a Raise, with the lengths in the style st.
//
//go:noinline
func (l *layout) layoutRaise(n Node, st Style) *box {
	r, _ := n.(Raise)
	return l.raiseBox(l.node(r.Body, st), l.length(r.Lift, st),
		l.length(r.Height, st), l.length(r.Depth, st), r.HasHeight, r.HasDepth)
}

// raiseBox places b lift above the baseline, for \raisebox. The box is an
// \hbox, so it is an Ord atom. Its height is h+lift and its depth d-lift,
// which can be negative: the bottom of the box is then above the baseline,
// as TeX allows. A declared height or depth then replaces it. The width does
// not change.
//
//go:noinline
func (l *layout) raiseBox(b *box, lift float64, h, d float64, hasH, hasD bool) *box {
	out := newBox(Ord)
	out.w = b.w
	out.h, out.d = b.h+lift, b.d-lift
	if hasH {
		out.h = h
	}
	if hasD {
		out.d = d
	}
	// y grows down, so a raise by lift is a negative dy.
	place(out, b, 0, -lift)
	return out
}
