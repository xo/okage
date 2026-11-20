package texmath

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// This file ports no file of go-tex/math. It holds the commands of KaTeX
// that phase 26 of D78 adds from the missing commands of D78 section 10:
// \cancel, \bcancel, \xcancel, \phase, \tag, \cfrac, \kern, \hspace, \llap,
// \rlap, \clap and their math forms, with the parse half and the layout half
// of each. \hline is in parseEnv, \scriptscriptstyle in listCommandOf,
// \dddot in the table of the accents, and \newcommand and the macros of
// KaTeX, such as \R, in macro.go. The source has none of them.

// parseCancel parses \cancel, \bcancel and \xcancel. The layout half is
// layoutCancel.
func (p *parser) parseCancel(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	n := Cancel{Body: b, Up: cmd.text != "bcancel", Down: cmd.text != "cancel"}
	return p.add(n), r, nil
}

// cancelPad and cancelLine are the padding and the width of the line of
// \cancel in KaTeX: the line goes 0.2 em past the top and the bottom of one
// character, and a longer body is padded by 0.2 em on the left and the
// right. The line is 0.046 em wide.
const (
	cancelPad  = 0.2
	cancelLine = 0.046
)

// layoutCancel lays out a Cancel: its body, padded as KaTeX pads it, with
// each line from corner to corner of the padded box. The line over one
// character overhangs the box, which keeps the height and the depth of the
// character, as in KaTeX. The box keeps the class of the body.
//
//go:noinline
func (l *layout) layoutCancel(n Node, st Style) *box {
	c, _ := n.(Cancel)
	b := l.node(c.Body, st)
	px := l.px(st)
	padX, padY := float64(cancelPad*px), 0.0
	if singleChar(c.Body) {
		padX, padY = 0, float64(cancelPad*px)
	}
	out := newBox(b.cls)
	out.w = b.w + float64(2*padX)
	out.h, out.d = b.h, b.d
	place(out, b, padX, 0)
	rt := float64(cancelLine * px)
	top, bot := -(b.h + padY), b.d+padY
	if c.Up {
		l.path(out, linePath(0, bot, out.w, top, rt))
	}
	if c.Down {
		l.path(out, linePath(0, top, out.w, bot, rt))
	}
	return out
}

// singleChar reports whether n is one character, as KaTeX tests the body of
// \cancel: an Atom with the text of one character, alone or in a List.
func singleChar(n Node) bool {
	if l, ok := n.(List); ok && len(l) == 1 {
		n = l[0]
	}
	a, ok := n.(Atom)
	return ok && a.Text != "" && len([]rune(a.Text)) == 1
}

// parsePhase parses \phase, a request of zuzu on 2026-11-20. The layout half
// is layoutPhase.
func (p *parser) parsePhase(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Phase{Body: b}), r, nil
}

// phaseLine and phaseClear are the width of the line of \phase and the space
// between the line and the body, which KaTeX takes from the package
// steinmetz: 0.6 pt, which is 0.06 em of the size of the layout, and 0.35 ex
// of text style.
const (
	phaseLine  = 0.06
	phaseClear = 0.35
)

// layoutPhase lays out a Phase as KaTeX lays out \phase (enclose.ts and
// phasePath of svgGeometry.ts). The angle is as high as the body, its line
// and its space: the box keeps the height of the body, and its depth grows
// by the line and the space. The body moves right by half that height and
// the line, so the slanted line goes from the bottom left corner to the top
// of the body at a slope of 2. KaTeX draws the angle as an SVG path in a
// view box that is 1000 units for each em of the style at the size of the
// layout, so the angle keeps its shape under a size command. In those units,
// the line under the body is 80 high, and the slanted line is 145 wide at
// the bottom and ends in a cut of 65 by 45 at the top. The view box is a
// whole number of units high, which scales the units a little.
//
//go:noinline
func (l *layout) layoutPhase(n Node, st Style) *box {
	ph, _ := n.(Phase)
	b := l.node(ph.Body, st)
	lw := float64(phaseLine * l.base)
	clear := float64(phaseClear * l.exHeight(l.px(Text)))
	ah := b.h + b.d + lw + clear
	pad := ah/2 + lw
	out := newBox(Ord)
	out.w = b.w + pad
	out.h, out.d = b.h, b.d+lw+clear
	place(out, b, pad, 0)
	em := float64(l.px(st)*l.base) / l.size
	vb := math.Floor(float64(1000*ah) / em)
	if !(vb > 0) {
		return out
	}
	u := ah / vb
	top, bot := -b.h, out.d
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(out.w), float32(bot))
	pb.LineTo(0, float32(bot))
	pb.LineTo(float32(float64(vb/2)*u), float32(top))
	pb.LineTo(float32(float64(float64(vb/2)*u)+float64(65*u)), float32(top+float64(45*u)))
	pb.LineTo(float32(float64(145*u)), float32(bot-float64(80*u)))
	pb.LineTo(float32(out.w), float32(bot-float64(80*u)))
	pb.Close()
	p, _ := pb.Finish()
	l.path(out, p)
	return out
}

// linePath returns the outline of a line of the width rt from (x0, y0) to
// (x1, y1), with square ends.
func linePath(x0, y0, x1, y1, rt float64) *okage.Path {
	dx, dy := x1-x0, y1-y0
	n := math.Sqrt(float64(dx*dx) + float64(dy*dy))
	if n == 0 {
		return nil
	}
	// The half width across the line.
	nx, ny := float64(float64(-dy/n*rt)/2), float64(float64(dx/n*rt)/2)
	pb := okage.NewPathBuilder()
	pb.MoveTo(float32(x0+nx), float32(y0+ny))
	pb.LineTo(float32(x1+nx), float32(y1+ny))
	pb.LineTo(float32(x1-nx), float32(y1-ny))
	pb.LineTo(float32(x0-nx), float32(y0-ny))
	pb.Close()
	p, _ := pb.Finish()
	return p
}

// startTag reads the \tag or \tag* at the start of toks, with its label,
// which is text, as KaTeX reads it. A formula has at most one \tag, and only
// in its outer list, or in a part of it that a style, a color or a size
// command starts. Parse puts the formula and the label into a Tag. \tag sets
// the label in parentheses, and \tag* without them.
//
//go:noinline
func (b *listBuilder) startTag(toks []token) error {
	p := b.p
	if b.stop != stopEnd {
		return p.fail(toks, `\tag is not in the outer list of the formula`)
	}
	if p.tag != nil {
		return p.fail(toks, `multiple \tag`)
	}
	rest := toks[1:]
	star := len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '*'
	if star {
		rest = rest[1:]
	}
	text := textAlphabet(alphabet{})
	label, r, err := p.parseGroupArg(rest, text)
	if err != nil {
		return err
	}
	b.flush()
	if star {
		p.tag = label
	} else {
		p.tag = p.list([]Node{p.add(text.run("(")), p.group(label), p.add(text.run(")"))})
	}
	b.toks = r
	return nil
}

// layoutTag lays out a Tag: the body, a quad, and the label in text style,
// as TeX sets an equation number at least a quad from its formula (tex.web
// §1202). The layout has no line width, so the label is at that distance and
// not at a margin. The box has the class of the body.
//
//go:noinline
func (l *layout) layoutTag(n Node, st Style) *box {
	t, _ := n.(Tag)
	body := l.node(t.Body, st)
	label := l.node(t.Label, Text)
	out := newBox(body.cls)
	bw, lw := body.w, label.w
	out.h, out.d = max(body.h, label.h), max(body.d, label.d)
	place(out, body, 0, 0)
	place(out, label, bw+l.size, 0)
	out.w = bw + l.size + lw
	out.ic = label.ic
	return out
}

// parseCfrac parses \cfrac[align]{num}{den} of amsmath: a fraction in
// display style, with its numerator and its denominator in display style
// and a strut in the numerator, \strut of LaTeX, 0.84 em high and 0.36 em
// deep, so that a chain of them keeps its rows apart. The alignment of the
// numerator, l or r, is read and dropped: the numerator is centered.
func (p *parser) parseCfrac(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	rest := toks
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		_, r, ok := readToBracket(rest[1:])
		if !ok {
			return nil, nil, p.fail(rest, `\cfrac: unterminated [align]`)
		}
		rest = r
	}
	num, r1, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	den, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	strut := p.add(Rule{Width: Length{Unit: Pt}, Height: Length{Value: 1.2, Unit: Em}, Lift: Length{Value: -0.36, Unit: Em}})
	numerator := p.add(StyleChange{Style: Display, Body: p.list([]Node{strut, num})})
	denominator := p.add(StyleChange{Style: Display, Body: den})
	return p.add(StyleChange{Style: Display, Body: p.add(Fraction{Num: numerator, Den: denominator})}), r2, nil
}

// parseKern parses \kern and \hspace: a horizontal space of a length, which
// can be negative. \kern takes the length with or without braces, such as
// \kern1em or \kern{3mu}, and \hspace and \hspace* take it in braces. The
// units are those of \rule, and mu. A length that this cannot read is an
// error, as an unknown unit is in KaTeX.
func (p *parser) parseKern(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	rest := toks
	if cmd.text == "hspace" && len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '*' {
		rest = rest[1:]
	}
	var s string
	if len(rest) > 0 && rest[0].kind == tLBrace {
		text, r, err := p.readOpName(rest)
		if err != nil {
			return nil, nil, p.fail(rest, fmt.Sprintf(`\%s needs a length`, cmd.text))
		}
		s, rest = text, r
	} else if cmd.text == "kern" {
		s, rest = readLengthToks(rest)
	}
	l, ok := readLength(s)
	if !ok {
		return nil, nil, p.fail(toks, fmt.Sprintf(`\%s needs a length, not %q`, cmd.text, s))
	}
	if err := p.checkLength(toks, `\`+cmd.text, l); err != nil {
		return nil, nil, err
	}
	return p.add(Space{Width: l}), rest, nil
}

// readLengthToks reads the characters of a length without braces at the
// start of toks: a signed decimal and a unit of two letters.
func readLengthToks(toks []token) (string, []token) {
	var sb strings.Builder
	i := 0
	for i < len(toks) && toks[i].kind == tChar && strings.ContainsRune("+-.0123456789", toks[i].r) {
		sb.WriteRune(toks[i].r)
		i++
	}
	if n := glueUnitLen(toks, i); n == 2 {
		for _, t := range toks[i : i+n] {
			sb.WriteRune(t.r)
		}
		i += n
	}
	return sb.String(), toks[i:]
}

// readLength reads a length of \kern or \hspace: a signed decimal and a unit
// of TeX, em, ex or mu. It reports false for a length without a number or
// with an unknown unit.
func readLength(s string) (Length, bool) {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && strings.IndexByte("+-.0123456789", s[i]) >= 0 {
		i++
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return Length{}, false
	}
	switch unit := strings.TrimSpace(s[i:]); unit {
	case "em":
		return Length{Value: v, Unit: Em}, true
	case "ex":
		return Length{Value: v, Unit: Ex}, true
	case "mu":
		return Length{Value: v, Unit: Mu}, true
	default:
		if c, ok := texUnits[unit]; ok {
			return Length{Value: v * c[0] / c[1], Unit: Pt}, true
		}
	}
	return Length{}, false
}

// parseLap parses \llap, \rlap and \clap, whose argument is text, and
// \mathllap, \mathrlap and \mathclap, whose argument is math, as KaTeX
// defines them. The layout half is layoutLap.
func (p *parser) parseLap(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := strings.TrimPrefix(cmd.text, "math")
	a := st
	if name == cmd.text {
		a = textAlphabet(alphabet{})
	}
	b, r, err := p.parseGroupArg(toks, a)
	if err != nil {
		return nil, nil, err
	}
	side := LapRight
	switch name {
	case "llap":
		side = LapLeft
	case "clap":
		side = LapCenter
	}
	return p.add(Lap{Body: b, Side: side}), r, nil
}
