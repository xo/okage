// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// \mkern<mudimen> and \mskip<muglue> are primitives of TeX that add a
// horizontal space in math units, where 18mu is one em of the current size.
// The space table uses the same unit: \, is 3mu, \: 4mu, \; 5mu, \quad 18mu.
//
// amsmath defines
//
//	\def\intkern@{\mkern-6mu\mathchoice{\mkern-3mu}{}{}{}}      amsmath.sty:654
//
// so each multiple integral of a document with amsmath has two of them, and a
// paper writes \mathchoice{}{}{\mskip-0.5mu}{\mskip-1mu}.
//
// \mskip takes glue, a width with an optional stretch and shrink. This parser
// has no glue, so only the width is kept, and the stretch is read and
// dropped.

// parseMuKern parses \mkern or \mskip, after the command cmd. The layout half
// of the source is e.kern(v * px / 18): a Space of v mu.
//
// okage: a width past maxLength em is an error. A width in another unit than
// mu, such as \mkern3pt, is read as KaTeX reads it, in that unit, and the
// parts of the glue of \mskip after it are dropped (D95).
func (p *parser) parseMuKern(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	v, rest, err := readMuDimen(toks, true)
	if err != nil {
		s, r := readLengthToks(toks)
		l, ok := readLength(s)
		if !ok || l.Unit == Mu {
			return nil, nil, p.fail(toks, fmt.Sprintf(`\%s: %s`, cmd.text, err))
		}
		if err := p.checkLength(toks, `\`+cmd.text, l); err != nil {
			return nil, nil, err
		}
		if cmd.text == "mskip" {
			r = r[skipGlue(r, 0):]
		}
		return p.add(Space{Width: l}), r, nil
	}
	if !(math.Abs(v) <= maxLength*18) {
		return nil, nil, p.limit(toks, fmt.Sprintf(`\%s: the width %gmu is more than %d em`, cmd.text, v, maxLength))
	}
	return p.add(Space{Width: Length{Value: v, Unit: Mu}}), rest, nil
}

// readMuDimen reads a signed decimal and the unit "mu", and for \mskip the
// "plus" and "minus" parts of the glue, which it reads and drops.
//
// A missing or unreadable width is an error and not a zero: \mkern with no
// argument would leave the number in the formula, to be set as digits.
//
// The tokens hold no white space, so "\mskip3mu plus 1mu" reads as
// "3muplus1mu".
//
// okage: glue is false for the parts of the glue, which have no glue of their
// own. The source reads them with glue, so a run such as "plus1muplus1mu…"
// recursed once for each part, without a bound.
func readMuDimen(toks []token, glue bool) (float64, []token, error) {
	var sb strings.Builder
	i := 0
	for i < len(toks) && toks[i].kind == tChar {
		r := toks[i].r
		if r == '+' || r == '-' || r == '.' || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			i++
			continue
		}
		break
	}
	if sb.Len() == 0 {
		return 0, nil, errors.New("no dimension")
	}
	v, err := strconv.ParseFloat(sb.String(), 64)
	if err != nil {
		return 0, nil, fmt.Errorf("unreadable dimension %q", sb.String())
	}
	// The unit must be "mu": these two primitives take math units only.
	if i+1 >= len(toks) || toks[i].kind != tChar || toks[i+1].kind != tChar ||
		toks[i].r != 'm' || toks[i+1].r != 'u' {
		return 0, nil, errors.New("expected mu")
	}
	i += 2
	if !glue {
		return v, toks[i:], nil
	}
	return v, toks[skipGlue(toks, i):], nil
}

// skipGlue returns the index after the stretch and the shrink of glue that
// start at i in toks, or i when there are none.
func skipGlue(toks []token, i int) int {
	// The stretch and the shrink of \mskip are read and dropped. "plus 1mu
	// minus 2mu" and "plus1fil" both come here.
	for _, kw := range []string{"plus", "minus"} {
		j := i
		if !hasWord(toks, j, kw) {
			continue
		}
		j += len(kw)
		if _, rest, err := readMuDimen(toks[j:], false); err == nil {
			i = len(toks) - len(rest)
		} else {
			// A part of the glue that this cannot read, such as fil, fill,
			// filll, or a length in another unit than mu, is skipped by its
			// shape: a number, then one known unit. Skipping every letter
			// would also skip the material after the glue, such as the b of
			// "\mskip3mu plus1fil b".
			for j < len(toks) && toks[j].kind == tChar &&
				(toks[j].r == '.' || toks[j].r == '-' || toks[j].r == '+' ||
					(toks[j].r >= '0' && toks[j].r <= '9')) {
				j++
			}
			j += glueUnitLen(toks, j)
			i = j
		}
	}
	return i
}

// hasWord reports whether the tokens at i spell w as characters.
func hasWord(toks []token, i int, w string) bool {
	if i+len(w) > len(toks) {
		return false
	}
	for k, c := range w {
		if toks[i+k].kind != tChar || toks[i+k].r != c {
			return false
		}
	}
	return true
}

// glueUnitLen is the length of the unit at i, or 0 when none is there. The
// infinite units come first, because fil is a prefix of fill and of filll.
func glueUnitLen(toks []token, i int) int {
	for _, u := range []string{"filll", "fill", "fil", "mu", "pt", "em", "ex",
		"in", "cm", "mm", "bp", "dd", "cc", "sp", "pc"} {
		if hasWord(toks, i, u) {
			return len(u)
		}
	}
	return 0
}
