package texmath

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// This file ports no file of go-tex/math. It holds the commands of KaTeX
// that phase 26 of D78 adds from the missing commands of D78 section 10:
// \cancel, \bcancel, \xcancel, \phase, \tag, \cfrac, \kern, \hspace, \llap,
// \rlap, \clap and their math forms, with the parse half and the layout half
// of each. \hline is in parseEnv, \scriptscriptstyle in listCommandOf,
// \dddot in the table of the accents, and \newcommand and the macros of
// KaTeX, such as \R, in macro.go. The source has none of them.
//
// On 2026-11-25 the maintainers asked for the rest of the commands of KaTeX
// that texmath did not parse, and for the alignment of \cfrac, the dashed
// rule of \hdashline and the label of \tag at the right margin (D95). The
// commands that make a node of their own are at the end of this file. The
// symbols are in the table of parse.go, and the macros in macro.go.

// parseCancel parses \cancel, \bcancel, \xcancel and \sout. The layout
// half is layoutCancel.
func (p *parser) parseCancel(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	if cmd.text == "sout" {
		return p.add(Cancel{Body: b, Strike: true}), r, nil
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
	if c.Strike && !c.Up && !c.Down {
		return l.strike(b, px)
	}
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

// strike draws the line of \sout across b, as KaTeX draws it (enclose.ts):
// a rule of the default rule thickness of KaTeX, 0.04 em, centered half the
// height of the letter x over the baseline. The box keeps the size and the
// class of b.
func (l *layout) strike(b *box, px float64) *box {
	out := newBox(b.cls)
	out.w, out.h, out.d, out.ic = b.w, b.h, b.d, b.ic
	place(out, b, 0, 0)
	rt := float64(soutLine * px)
	y := -float64(l.exHeight(px)/2) - float64(rt/2)
	l.rule(out, 0, y, out.w, rt)
	return out
}

// soutLine is the width of the line of \sout, the default rule thickness
// of KaTeX, in em.
const soutLine = 0.04

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
// §1202). Without WithLineWidth, the label is at that distance. With it, the
// body is centered in the line and the label is at the right margin, as
// TeX sets a display with \eqno. When the centered body comes closer than a
// quad to the label, the body moves left. A line that is too short for the
// body, a quad and the label sets them as without the option. The box has
// the class of the body.
//
//go:noinline
func (l *layout) layoutTag(n Node, st Style) *box {
	t, _ := n.(Tag)
	body := l.node(t.Body, st)
	label := l.node(t.Label, Text)
	out := newBox(body.cls)
	bw, lw := body.w, label.w
	out.h, out.d = max(body.h, label.h), max(body.d, label.d)
	out.ic = label.ic
	if line := l.cfg.lineWidth; line > bw+l.size+lw {
		labelX := line - lw
		bodyX := max(min(float64((line-bw)/2), labelX-l.size-bw), 0)
		place(out, body, bodyX, 0)
		place(out, label, labelX, 0)
		out.w = line
		return out
	}
	place(out, body, 0, 0)
	place(out, label, bw+l.size, 0)
	out.w = bw + l.size + lw
	return out
}

// WithLineWidth sets the width of the line of a formula, in the unit of the
// size of the layout. A formula with \tag then has the width of the line,
// with the body centered and the label at the right margin. A width of 0 or
// less, the default, sets the label a quad after the body. The other
// formulas do not change (D95).
func WithLineWidth(w float64) Option {
	return func(c *config) {
		c.lineWidth = 0
		if w > 0 && !math.IsInf(w, 1) {
			c.lineWidth = w
		}
	}
}

// parseCfrac parses \cfrac[align]{num}{den} of amsmath: a fraction in
// display style, with its numerator and its denominator in display style
// and a strut in the numerator, \strut of LaTeX, 0.84 em high and 0.36 em
// deep, so that a chain of them keeps its rows apart. The alignment of the
// numerator, l or r, sets a numerator that is narrower than the denominator
// at the left or at the right of it, as the \hfill of amsmath does (D95).
// Another alignment is an error, as in KaTeX.
func (p *parser) parseCfrac(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	rest := toks
	align := AlignCenter
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		a, r, ok := readToBracket(rest[1:])
		if !ok {
			return nil, nil, p.fail(rest, `\cfrac: unterminated [align]`)
		}
		switch strings.TrimSpace(a) {
		case "l":
			align = AlignLeft
		case "r":
			align = AlignRight
		case "c", "":
		default:
			return nil, nil, p.fail(rest, fmt.Sprintf(`\cfrac: unknown alignment %q`, a))
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
	return p.add(StyleChange{Style: Display, Body: p.add(Fraction{Num: numerator, Den: denominator, NumAlign: align})}), r2, nil
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

// ── The commands of D95 ─────────────────────────────────────────────────────

// parseSmallint parses \smallint: the integral sign of text style, which does
// not grow in display style, as KaTeX sets it.
func (p *parser) parseSmallint(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	return p.add(Atom{Class: Op, Text: "∫", Variant: Upright, Limits: LimitsSide}), toks, nil
}

// parseNewline parses \\ and \newline outside an environment, with the
// optional [length] of \\. texmath sets a formula on one line, so they make
// a space of no width.
func (p *parser) parseNewline(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	rest := toks
	if cmd.text == `\` && len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		_, r, ok := readToBracket(rest[1:])
		if !ok {
			return nil, nil, p.fail(rest, `\\: unterminated [length]`)
		}
		rest = r
	}
	return p.add(Space{}), rest, nil
}

// parseStretch parses \overgroup, \undergroup, \utilde and \Overrightarrow.
// The layout half is layoutStretch.
func (p *parser) parseStretch(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	n := Stretch{Base: b}
	switch cmd.text {
	case "overgroup":
		n.Rune = '⏠'
	case "undergroup":
		n.Rune, n.Under = '⏡', true
	case "utilde":
		n.Rune, n.Under = '̰', true
	case "Overrightarrow":
		n.Rune = '⇒'
	}
	return p.add(n), r, nil
}

// layoutStretch lays out a Stretch: its character 0.1 em from the base, and
// the box 0.1 em past the ink of the character. A face without the
// character sets the base alone.
//
//go:noinline
func (l *layout) layoutStretch(n Node, st Style) *box {
	s, _ := n.(Stretch)
	b := l.node(s.Base, st)
	if _, ok := l.face.NominalGlyph(s.Rune); !ok {
		l.missing(s.Rune)
		return b
	}
	px := l.px(st)
	gap := float64(px * 0.1)
	return l.stretchOver(b, s.Rune, !s.Under, gap, gap, Ord, px)
}

// parseVCenter parses \vcenter. The layout half is layoutVCenter.
func (p *parser) parseVCenter(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(VCenter{Body: b}), r, nil
}

// layoutVCenter lays out a VCenter: its body moved so that the middle of its
// height and depth is on the math axis, as KaTeX does (vcenter.ts). The box
// is an Ord.
//
//go:noinline
func (l *layout) layoutVCenter(n Node, st Style) *box {
	v, _ := n.(VCenter)
	out := l.axisCentre(l.node(v.Body, st), l.px(st))
	out.cls = Ord
	return out
}

// parseAngl parses \angl. The layout half is layoutAngl.
func (p *parser) parseAngl(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Angl{Body: b}), r, nil
}

// anglLine and anglPad are the width of the lines of \angl, the default rule
// thickness of KaTeX, and the padding at the left and the right of its body,
// the class anglpad of katex.css, in em.
const (
	anglLine = 0.04
	anglPad  = 0.03889
)

// layoutAngl lays out an Angl as KaTeX lays out \angl (enclose.ts): the body
// padded at its sides, a line over it four line widths above the body, and
// a line at its right from that line down to the depth of the body, or to
// 0.25 em under the baseline when the body is less deep.
//
//go:noinline
func (l *layout) layoutAngl(n Node, st Style) *box {
	a, _ := n.(Angl)
	b := l.node(a.Body, st)
	px := l.px(st)
	rt := float64(anglLine * px)
	pad := float64(anglPad * px)
	out := newBox(Ord)
	out.w = b.w + float64(2*pad) + rt
	out.h = b.h + float64(4*rt)
	out.d = b.d + max(0, float64(0.25*px)-b.d)
	place(out, b, pad, 0)
	l.rule(out, 0, -out.h, out.w, rt)
	l.rule(out, out.w-rt, -out.h, rt, out.h+out.d)
	return out
}

// parseSideset parses \sideset{left}{right}, and the large operator after
// it with its own scripts. left and right hold scripts only, such as _1^2.
// The layout half is layoutSideset.
func (p *parser) parseSideset(_ *token, toks []token, st alphabet) (Node, []token, error) {
	left, r1, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	right, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	var n Sideset
	var ok1, ok2 bool
	n.LeftSup, n.LeftSub, ok1 = sidesetScripts(left)
	n.RightSup, n.RightSub, ok2 = sidesetScripts(right)
	if !ok1 || !ok2 {
		return nil, nil, p.fail(toks, `\sideset needs {scripts}{scripts}`)
	}
	if len(r2) == 0 {
		return nil, nil, p.fail(r2, `\sideset needs an operator`)
	}
	base, r3, err := p.parseAtom(r2, st)
	if err != nil {
		return nil, nil, err
	}
	base, r4, err := p.parseScripts(base, r3, st)
	if err != nil {
		return nil, nil, err
	}
	n.Base = base
	return p.add(n), r4, nil
}

// sidesetScripts returns the superscript and the subscript of an argument
// of \sideset, and false when the argument holds anything other than
// scripts on an empty nucleus. Primes come before the superscript, as the
// prime characters of the math face.
func sidesetScripts(l List) (sup, sub Node, ok bool) {
	for _, it := range l {
		s, isScripts := it.(Scripts)
		if !isScripts {
			return nil, nil, false
		}
		if b, isList := s.Base.(List); !isList || len(b) != 0 {
			return nil, nil, false
		}
		if s.Sup != nil {
			sup = s.Sup
		}
		if s.Primes > 0 {
			primes := Atom{Class: Ord, Text: strings.Repeat("′", min(s.Primes, maxSidesetPrimes)), Variant: Upright}
			if sup != nil {
				sup = List{primes, sup}
			} else {
				sup = primes
			}
		}
		if s.Sub != nil {
			sub = s.Sub
		}
	}
	return sup, sub, true
}

// maxSidesetPrimes is the most primes of an argument of \sideset that the
// superscript holds.
const maxSidesetPrimes = 16

// layoutSideset lays out a Sideset as amsmath sets \sideset: the operator
// with its limits, the left scripts before the operator and the right
// scripts after it, each set as the scripts of the operator alone. The box is
// an Op.
//
//go:noinline
func (l *layout) layoutSideset(n Node, st Style) *box {
	s, _ := n.(Sideset)
	nucNode := s.Base
	if sc, ok := nucNode.(Scripts); ok {
		nucNode = sc.Base
	}
	core := l.node(s.Base, st)
	nuc := l.node(nucNode, st)
	sst := scriptOf(st)
	side := func(sup, sub Node) *box {
		if sup == nil && sub == nil {
			return newBox(Ord)
		}
		var bs, bb *box
		if sup != nil {
			bs = l.node(sup, sst)
		}
		if sub != nil {
			bb = l.node(sub, sst)
		}
		return l.attachScripts(l.phantom(nuc, false, true), bs, bb, st)
	}
	left := side(s.LeftSup, s.LeftSub)
	right := side(s.RightSup, s.RightSub)
	// The nucleus is centered in core, which holds the limits.
	margin := float64((core.w - nuc.w) / 2)
	nucX := max(left.w, margin)
	coreX := nucX - margin
	rightX := nucX + nuc.w
	out := newBox(Op)
	out.w = max(coreX+core.w, rightX+right.w)
	out.h = max(core.h, left.h, right.h)
	out.d = max(core.d, left.d, right.d)
	leftW := left.w
	place(out, left, nucX-leftW, 0)
	place(out, core, coreX, 0)
	place(out, right, rightX, 0)
	return out
}

// verbStart returns the offset in p.tex just after the control word of cmd,
// and false when cmd does not come from the source, as a token of a macro
// body does not.
func (p *parser) verbStart(cmd *token) (int, bool) {
	name := `\` + cmd.text
	if cmd.pos < 0 || cmd.pos+len(name) > len(p.tex) || p.tex[cmd.pos:cmd.pos+len(name)] != name {
		return 0, false
	}
	return cmd.pos + len(name), true
}

// skipTo returns the tokens of toks from the first one at or after the
// offset end of the source.
func skipTo(toks []token, end int) []token {
	for i, t := range toks {
		if t.pos >= end {
			return toks[i:]
		}
	}
	return nil
}

// parseVerb parses \verb and \verb*: the characters of the source between
// two copies of the character after the command, as text in the monospace
// family, with their spaces. \verb* shows each space as U+2423.
func (p *parser) parseVerb(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	start, ok := p.verbStart(cmd)
	if !ok {
		return nil, nil, p.fail(toks, `\verb is not in the source`)
	}
	star := start < len(p.tex) && p.tex[start] == '*'
	if star {
		start++
	}
	delim, size := utf8.DecodeRuneInString(p.tex[start:])
	if size == 0 || delim == ' ' || delim == utf8.RuneError {
		return nil, nil, p.fail(toks, `\verb needs a delimiter`)
	}
	body := p.tex[start+size:]
	end := strings.IndexRune(body, delim)
	if end < 0 {
		return nil, nil, p.fail(toks, `\verb: unterminated text`)
	}
	text := body[:end]
	if star {
		text = strings.ReplaceAll(text, " ", "␣")
	}
	rest := skipTo(toks, start+size+end+size)
	return p.add(TextRun{Text: text, Family: FamilyMono}), rest, nil
}

// rawGroup returns the characters of the source in the {group} at the start
// of toks, and the tokens after it. It reports false when the group does not
// close, or when it does not come from the source.
func (p *parser) rawGroup(toks []token) (string, []token, bool) {
	g, rest, ok := readGroupToks(toks)
	if !ok {
		return "", nil, false
	}
	open, closing := toks[0].pos, toks[len(g)+1].pos
	if open < 0 || closing >= len(p.tex) || open >= closing || p.tex[open] != '{' || p.tex[closing] != '}' {
		return "", nil, false
	}
	return p.tex[open+1 : closing], rest, true
}

// parseURL parses \url{url}: the characters of the url, as text in the
// monospace family. A backslash before one of #$%&~_^{}\ is dropped, as
// KaTeX reads an argument of the type url. The link is not kept, because a
// box cannot hold one.
func (p *parser) parseURL(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	raw, rest, ok := p.rawGroup(toks)
	if !ok {
		return nil, nil, p.fail(toks, `\url needs {url}`)
	}
	var sb strings.Builder
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' && i+1 < len(raw) && strings.IndexByte(`#$%&~_^{}\`, raw[i+1]) >= 0 {
			i++
		}
		sb.WriteByte(raw[i])
	}
	return p.add(TextRun{Text: sb.String(), Family: FamilyMono}), rest, nil
}

// parseHref parses \href{url}{body}, and \htmlId, \htmlClass, \htmlStyle and
// \htmlData, which take a value of HTML and a body. A box cannot hold a link
// or an attribute of HTML, so each sets its body alone, as an Ord.
func (p *parser) parseHref(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	_, rest, ok := readGroupToks(toks)
	if !ok {
		return nil, nil, p.fail(toks, fmt.Sprintf(`\%s needs {value}{body}`, cmd.text))
	}
	b, r, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	return p.group(b), r, nil
}

// parseIncludegraphics parses \includegraphics[options]{url}. A box cannot
// hold an image, so it is an empty space of the size of the image, as
// KaTeX gives it: the height of the option height, 0.9 em by default, the
// depth that the option totalheight adds to it, and the width of the option
// width. Without a width, the space is as wide as it is high.
func (p *parser) parseIncludegraphics(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	rest := toks
	height := Length{Value: 0.9, Unit: Em}
	var width, total Length
	hasTotal := false
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		opts, r, ok := readToBracket(rest[1:])
		if !ok {
			return nil, nil, p.fail(rest, `\includegraphics: unterminated [options]`)
		}
		rest = r
		for opt := range strings.SplitSeq(opts, ",") {
			key, value, _ := strings.Cut(opt, "=")
			key = strings.TrimSpace(key)
			if key != "height" && key != "width" && key != "totalheight" {
				continue
			}
			l, ok := readLength(value)
			if !ok {
				return nil, nil, p.fail(toks, fmt.Sprintf(`\includegraphics: %s needs a length, not %q`, key, value))
			}
			if err := p.checkLength(toks, `\includegraphics`, l); err != nil {
				return nil, nil, err
			}
			switch key {
			case "height":
				height = l
			case "width":
				width = l
			case "totalheight":
				total, hasTotal = l, true
			}
		}
	}
	if _, r, ok := readGroupToks(rest); ok {
		rest = r
	} else {
		return nil, nil, p.fail(rest, `\includegraphics needs {url}`)
	}
	rule := Rule{Width: width, Height: height}
	if width.Value == 0 {
		rule.Width = height
	}
	if hasTotal && total.Unit == height.Unit && total.Value > height.Value {
		rule.Height = total
		rule.Lift = Length{Value: height.Value - total.Value, Unit: height.Unit}
	}
	return p.add(Phantom{Body: p.add(rule), Horizontal: true, Vertical: true}), rest, nil
}

// arrowOverUnder lays out an OverArrow under its base, with harpoons or
// with the bars of a line segment: the forms of KaTeX of D95. The arrow has
// the size of the arrow of overArrow. Under the base, it takes the gap and
// the extra descender of a bar under the base from the MATH table.
//
//go:noinline
func (l *layout) arrowOverUnder(b *box, a *OverArrow, st Style) *box {
	px := l.px(st)
	rt := max(l.mc(font.OverbarRuleThickness, px), float64(px*0.045))
	headLen := float64(px * 0.42)
	headHW := float64(px * 0.24)
	out := newBox(Ord)
	out.w, out.h, out.d = b.w, b.h, b.d
	place(out, b, 0, 0)
	var y float64 // the center line of the arrow
	if a.Under {
		bot := b.d + l.mc(font.UnderbarVerticalGap, px)
		y = bot + headHW
		out.d = bot + float64(2*headHW) + l.mc(font.UnderbarExtraDescender, px)
	} else {
		top := b.h + l.mc(font.OverbarVerticalGap, px)
		y = -(top + headHW)
		out.h = top + float64(2*headHW) + l.mc(font.OverbarExtraAscender, px)
	}
	l.rule(out, 0, y-float64(rt/2), out.w, rt)
	if a.Segment {
		if a.Left {
			l.rule(out, 0, y-headHW, rt, float64(2*headHW))
		}
		if a.Right {
			l.rule(out, out.w-rt, y-headHW, rt, float64(2*headHW))
		}
		return out
	}
	if a.Right {
		if a.Harpoon {
			l.path(out, harpoonHead(out.w, out.w-headLen, y, headHW, rt, true))
		} else {
			l.path(out, arrowHead(out.w, out.w-headLen, y, headHW))
		}
	}
	if a.Left {
		if a.Harpoon {
			l.path(out, harpoonHead(0, headLen, y, headHW, rt, true))
		} else {
			l.path(out, arrowHead(0, headLen, y, headHW))
		}
	}
	return out
}

// harpoonHead returns the half of a head of an arrow on one side of a shaft
// of the width rt, with its tip at (tipX, y), its back at backX and its barb
// hw from the line y, above the shaft when up and under it otherwise. The
// points go clockwise on the screen, as those of arrowHead do.
func harpoonHead(tipX, backX, y, hw, rt float64, up bool) *okage.Path {
	half := float64(rt / 2)
	pb := okage.NewPathBuilder()
	switch {
	case up && tipX > backX:
		pb.MoveTo(float32(tipX), float32(y+half))
		pb.LineTo(float32(backX), float32(y+half))
		pb.LineTo(float32(backX), float32(y-hw))
	case up:
		pb.MoveTo(float32(tipX), float32(y+half))
		pb.LineTo(float32(backX), float32(y-hw))
		pb.LineTo(float32(backX), float32(y+half))
	case tipX > backX:
		pb.MoveTo(float32(tipX), float32(y-half))
		pb.LineTo(float32(backX), float32(y+hw))
		pb.LineTo(float32(backX), float32(y-half))
	default:
		pb.MoveTo(float32(tipX), float32(y-half))
		pb.LineTo(float32(backX), float32(y-half))
		pb.LineTo(float32(backX), float32(y+hw))
	}
	pb.Close()
	p, _ := pb.Finish()
	return p
}

// xArrowMore lays out the extensible arrows of D95 that xArrow does not
// draw alone, and returns nil for another arrow. It is not inlined, so that
// its frame is not on the stack of a nested arrow.
//
//go:noinline
func (l *layout) xArrowMore(sup, sub *box, arrow rune, st Style) *box {
	switch arrow {
	case '⇌', '⇋', '⇄', '=':
		return l.xArrowPair(sup, sub, arrow, st)
	case '↠', '↞':
		left := arrow == '↞'
		b := l.xArrow(sup, sub, left, !left, false, false, st)
		l.xArrowTwoHead(b, left, st)
		return b
	}
	return nil
}

// xArrowTwoHead adds the second head of \xtwoheadrightarrow or
// \xtwoheadleftarrow to the arrow b that xArrow made, half a head behind the
// first one, as the glyphs U+21A0 and U+219E draw it.
func (l *layout) xArrowTwoHead(b *box, left bool, st Style) {
	px := l.px(st)
	axisY := -l.axis(px)
	headLen := float64(px * 0.42)
	headHW := float64(px * 0.24)
	tip := b.w - float64(headLen/2)
	back := tip - headLen
	if left {
		tip = float64(headLen / 2)
		back = tip + headLen
	}
	l.path(b, arrowHead(tip, back, axisY, headHW))
}

// xArrowPairGap is the distance of each of the two shafts of an arrow of
// xArrowPair from the math axis, in em.
const xArrowPairGap = 0.15

// xArrowPair builds an arrow of two shafts that grows to the width of its
// labels, as xArrow builds an arrow of one: \xrightleftharpoons (U+21CC),
// \xleftrightharpoons (U+21CB), \xtofrom (U+21C4) and \xlongequal (=). The
// shafts are xArrowPairGap above and under the math axis. The upper shaft
// of the harpoons carries its barb above it and the lower one under it. The
// heads of \xtofrom are 0.6 times as wide as those of xArrow, so that they
// stay apart.
//
//go:noinline
func (l *layout) xArrowPair(sup, sub *box, arrow rune, st Style) *box {
	px := l.px(st)
	axisY := -l.axis(px)
	rt := max(l.mc(font.FractionRuleThickness, px), float64(px*0.045))
	headLen := float64(px * 0.42)
	headHW := float64(px * 0.24)
	pad := float64(px * 0.30)
	minLen := float64(px * 1.4)
	sep := float64(px * xArrowPairGap)
	labelW := 0.0
	if sup != nil {
		labelW = max(labelW, sup.w)
	}
	if sub != nil {
		labelW = max(labelW, sub.w)
	}
	w := max(labelW+float64(2*pad), minLen)
	out := newBox(Rel)
	out.w = w
	upper, lower := axisY-sep, axisY+sep
	half := float64(rt / 2)
	topY, botY := upper-half, lower+half
	switch arrow {
	case '=':
		l.rule(out, 0, upper-half, w, rt)
		l.rule(out, 0, lower-half, w, rt)
	case '⇄':
		hw := float64(headHW * 0.6)
		l.rule(out, 0, upper-half, w-headLen, rt)
		l.path(out, arrowHead(w, w-headLen, upper, hw))
		l.rule(out, headLen, lower-half, w-headLen, rt)
		l.path(out, arrowHead(0, headLen, lower, hw))
		topY, botY = upper-hw, lower+hw
	case '⇌':
		l.rule(out, 0, upper-half, w-headLen, rt)
		l.path(out, harpoonHead(w, w-headLen, upper, headHW, rt, true))
		l.rule(out, headLen, lower-half, w-headLen, rt)
		l.path(out, harpoonHead(0, headLen, lower, headHW, rt, false))
		topY, botY = upper-headHW, lower+headHW
	default: // '⇋'
		l.rule(out, headLen, upper-half, w-headLen, rt)
		l.path(out, harpoonHead(0, headLen, upper, headHW, rt, true))
		l.rule(out, 0, lower-half, w-headLen, rt)
		l.path(out, harpoonHead(w, w-headLen, lower, headHW, rt, false))
		topY, botY = upper-headHW, lower+headHW
	}
	out.h = -topY
	out.d = botY
	gap := float64(px * 0.12)
	if sup != nil {
		by := topY - gap - sup.d
		out.h = max(out.h, -by+sup.h)
		place(out, sup, (w-sup.w)/2, by)
	}
	if sub != nil {
		by := botY + gap + sub.h
		out.d = max(out.d, by+sub.d)
		place(out, sub, (w-sub.w)/2, by)
	}
	out.d = max(out.d, 0)
	return out
}
