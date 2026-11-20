// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// This file holds \rule in math mode.
//
// \rule is a LaTeX command for any mode: \rule{0pt}{2.6ex} in a cell of an
// array or in a \frac is the usual way to strut a row or to open a fraction,
// and \rule{0pt}{h} gives a formula a minimum height. go-tex/engine#294 is a
// document that lost all its pages to an unknown \rule.
//
// The layout half is layoutRule, the end of parseRule of the source.

// parseRule parses \rule[lift]{width}{height}. The lift is 0 when it is
// absent.
//
// okage: a length past the limit of a length is an error.
func (p *parser) parseRule(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	var lift Length
	if len(toks) > 0 && toks[0].kind == tChar && toks[0].r == '[' {
		s, rest, ok := readToBracket(toks[1:])
		if !ok {
			return nil, nil, p.fail(toks, `\rule: unterminated [lift]`)
		}
		lift, toks = readDimen(s), rest
	}
	ws, rest, err := p.readOpName(toks)
	if err != nil {
		return nil, nil, p.fail(toks, `\rule needs {width}{height}`)
	}
	hs, rest2, err := p.readOpName(rest)
	if err != nil {
		return nil, nil, p.fail(rest, `\rule needs {width}{height}`)
	}
	n := Rule{Width: readDimen(ws), Height: readDimen(hs), Lift: lift}
	for _, l := range []Length{n.Width, n.Height, n.Lift} {
		if err := p.checkLength(toks, `\rule`, l); err != nil {
			return nil, nil, err
		}
	}
	return p.add(n), rest2, nil
}

// readToBracket collects the characters up to the next ']'.
func readToBracket(toks []token) (string, []token, bool) {
	var sb strings.Builder
	for i, t := range toks {
		if t.kind == tChar && t.r == ']' {
			return sb.String(), toks[i+1:], true
		}
		switch t.kind {
		case tChar:
			sb.WriteString(t.text)
		case tCtrl:
			sb.WriteString(`\` + t.text)
		}
	}
	return "", nil, false
}

// texUnits are the units of length of TeX as exact fractions of a point, from
// tex.web:9019-9034 ("Scan for all other units"). sp is a point over 65536.
// em and ex come from the font.
var texUnits = map[string][2]float64{
	"pt": {1, 1},
	"in": {7227, 100},
	"pc": {12, 1},
	"cm": {7227, 254},
	"mm": {7227, 2540},
	"bp": {7227, 7200},
	"dd": {1238, 1157},
	"cc": {14856, 1157},
	"sp": {1, 65536},
}

// readDimen reads a length of TeX: a number and a unit. It is the parse half
// of e.dimen of the source. A bare number is in points. A length that it
// cannot read is 0pt and not an error: \rule is a space, and a formula that
// loses a strut is better than a document that loses the formula.
func readDimen(s string) Length {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '+' || s[i] == '-' || s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 64)
	if err != nil {
		return Length{Unit: Pt}
	}
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	switch unit {
	case "", "pt":
		return Length{Value: v, Unit: Pt}
	case "em":
		return Length{Value: v, Unit: Em}
	case "ex":
		return Length{Value: v, Unit: Ex}
	case "mu":
		// okage: a length in mu, as KaTeX reads it in \rule. The source
		// reads it as 0pt.
		return Length{Value: v, Unit: Mu}
	}
	if c, ok := texUnits[unit]; ok {
		return Length{Value: v * c[0] / c[1], Unit: Pt}
	}
	return Length{Unit: Pt}
}

// checkLength reports a length that is past the limit of a length: maxLength
// of its unit, or maxLength em for a length in mu.
func (p *parser) checkLength(toks []token, cmd string, l Length) error {
	limit := float64(maxLength)
	if l.Unit == Mu {
		limit *= 18
	}
	if !(math.Abs(l.Value) <= limit) {
		return p.limit(toks, fmt.Sprintf(`%s: the length %g is more than %d of its unit`, cmd, l.Value, maxLength))
	}
	return nil
}

// layoutRule lays out a Rule. It is the end of parseRule of the source. The
// box is lift+height high and, for a negative lift, -lift deep, with a
// height that is never negative. A rule with a width and a height above zero
// draws a rectangle with its bottom edge lift above the baseline. A length in
// em is the size of the style, and a length in ex is the height of the glyph
// "x" of the face, 0 when the face has none (e.dimen of the source).
//
//go:noinline
func (l *layout) layoutRule(n Node, st Style) *box {
	r, _ := n.(Rule)
	w, h, lift := l.length(r.Width, st), l.length(r.Height, st), l.length(r.Lift, st)
	out := newBox(Ord)
	out.w = w
	out.h = max(h+lift, 0)
	if lift < 0 {
		out.d = -lift
	}
	if w > 0 && h > 0 {
		l.rule(out, 0, -(h + lift), w, h) // y grows down, and the baseline is 0
	}
	return out
}
