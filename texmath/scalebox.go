// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// \scalebox{h}[v]{content} scales a box, and \reflectbox mirrors it.
// graphics.sty gives both:
//
//	:519  \protected\def\scalebox#1{\@ifnextchar[{\Gscale@box{#1}}{\Gscale@box{#1}[#1]}}
//	:521  \long\def\Gscale@box#1[#2]#3{…}
//	:538  \protected\def\reflectbox{\Gscale@box-1[1]}
//
// So the vertical factor is the horizontal one when it is absent, and
// \reflectbox is \scalebox{-1}[1], a mirror. parseScalebox in parse.go makes
// the Scale node.
//
// The layout half is scaleBox, with a transform on the items in place of the
// SVG text of the source.

// readScaleFactor reads one factor of \scalebox: a {group}, or a bare number,
// as the definition of \reflectbox writes -1 without braces.
//
// A factor that does not parse is an error, not a silent 1.
//
// okage: a factor that is not finite, or past maxScale, is an error. The
// source takes NaN and Inf from strconv.ParseFloat into the SVG text.
func (p *parser) readScaleFactor(toks []token) (float64, []token, error) {
	txt, rest := readArgText(toks)
	v, err := strconv.ParseFloat(strings.TrimSpace(txt), 64)
	if err != nil {
		return 0, nil, p.fail(toks, fmt.Sprintf("\\scalebox factor %q is not a number", txt))
	}
	if err := p.checkScale(toks, v); err != nil {
		return 0, nil, err
	}
	return v, rest, nil
}

// readOptScaleFactor reads the optional [v] of \scalebox, and reports whether
// it is there. An unclosed [, or a factor that does not parse, is not an
// optional argument.
//
// okage: a factor that is not finite, or past maxScale, is an error.
func (p *parser) readOptScaleFactor(toks []token) (float64, []token, bool, error) {
	if len(toks) == 0 || toks[0].kind != tChar || toks[0].r != '[' {
		return 0, toks, false, nil
	}
	var b strings.Builder
	i := 1
	for ; i < len(toks) && (toks[i].kind != tChar || toks[i].r != ']'); i++ {
		if toks[i].kind == tChar {
			b.WriteRune(toks[i].r)
		}
	}
	if i >= len(toks) {
		return 0, toks, false, nil
	}
	v, perr := strconv.ParseFloat(strings.TrimSpace(b.String()), 64)
	if perr == nil {
		if err := p.checkScale(toks, v); err != nil {
			return 0, nil, false, err
		}
		return v, toks[i+1:], true, nil
	}
	return 0, toks, false, nil
}

// checkScale reports a factor that is not finite, or past maxScale.
func (p *parser) checkScale(toks []token, v float64) error {
	if !(math.Abs(v) <= maxScale) {
		return p.limit(toks, fmt.Sprintf(`\scalebox factor %g is not a finite number of at most %d`, v, maxScale))
	}
	return nil
}

// layoutScale lays out a Scale.
//
//go:noinline
func (l *layout) layoutScale(n Node, st Style) *box {
	s, _ := n.(Scale)
	return l.scaleBox(l.node(s.Body, st), s.X, s.Y)
}

// scaleBox scales b by h and v around the left end of its baseline. The box
// keeps the class of b: a scale changes the size of a box, not the kind of
// its atom. A negative h mirrors b about x = 0, so the content goes left of
// the origin. graphics.sty:534-536 puts a kern in an \hbox of the new width,
// and here a translation by that width does the same.
//
//go:noinline
func (l *layout) scaleBox(b *box, h, v float64) *box {
	out := newBox(b.cls)
	// graphics.sty:526-532: a negative vertical factor swaps the height and
	// the depth, because the scale around the baseline turns what was above
	// it to below it.
	if v < 0 {
		out.h, out.d = float64(-v*b.d), float64(-v*b.h)
	} else {
		out.h, out.d = float64(v*b.h), float64(v*b.d)
	}
	// :533-536: the width is |h| times the width of b.
	out.w = float64(math.Abs(h) * b.w)
	t := okage.TransformFromScale(float32(h), float32(v))
	if h < 0 {
		t = t.PostTranslate(float32(out.w), 0)
	}
	for i := range b.items {
		b.items[i].Transform = b.items[i].Transform.PostConcat(t)
	}
	out.items, b.items = b.items, nil
	return out
}
