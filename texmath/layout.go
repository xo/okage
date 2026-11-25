package texmath

import (
	"fmt"
	"log/slog"
	"math"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/shaping"
)

// This file ports no file of go-tex/math. It holds Layout and the walk of
// the tree of D78 section 3. The source lays out each command as it parses
// it. Here the walk calls, for each node, the layout half of the command
// that made it, which is in the file of that command: math.go for most of
// them, and rule.go, raisebox.go, scalebox.go, ooalign.go and mathchoice.go
// for the others.

// maxItems is the largest number of items that a layout makes (D78 section
// 5).
const maxItems = 1000000

// WithColor sets the color of each item outside a Color node. The default is
// opaque black. The source has no such setting: its SVG text fills with
// currentColor.
func WithColor(c okage.Color) Option {
	return func(cfg *config) {
		cfg.color, cfg.hasColor = c, true
	}
}

// WithItalicCorrection turns the italic correction of the MATH table on or
// off. It is on by default, as phase 26 of D78 adds it. The layout then sets a
// superscript of a character that the MATH table gives an italic correction,
// such as f or the integral sign, to the right by that correction, sets the
// upper limit of an operator half of it to the right and the lower limit half
// of it to the left, puts the correction after such a character in a list,
// and gives the correction of the last glyph in Box.Italic, as
// TeX Appendix G, rules 13a, 17 and 18, and the OpenType specification say.
// The width of a list includes the italic correction of a character at its
// end, as in TeX and KaTeX, and Box.Italic gives it, so that a caller that
// sets a script or a slanted letter after the box can take it off. Off, the
// layout uses no italic correction, as the source.
func WithItalicCorrection(on bool) Option {
	return func(cfg *config) {
		cfg.noItalic = !on
	}
}

// WithAccentAttachment turns the top accent attachment of the MATH table on
// or off. It is on by default, as phase 26 of D78 adds it. The layout then
// sets an accent with the combining mark of the face, such as U+0302 for
// \hat, and puts the attachment point of the mark over the attachment point
// of a nucleus of one glyph, such as an italic letter. A glyph without an
// attachment point has it in the middle of its advance, and a mark with no
// advance in the middle of its ink. Off, the layout centers the spacing
// accent, such as U+02C6, on the advance of the nucleus, as the source. A
// stretched accent, such as \widehat, always takes the horizontal variants
// of the combining mark.
func WithAccentAttachment(on bool) Option {
	return func(cfg *config) {
		cfg.noAttach = !on
	}
}

// WithMathKern turns the math kerning of the MATH table on or off. It is on
// by default, as phase 26 of D78 adds it. The layout then moves a script of
// a nucleus of one glyph by the kerns of the corners of the two glyphs that
// face each other, at two heights, as the OpenType specification says. The
// smaller sum of the two heights moves the script. Off, the layout uses no
// math kerning, as the source.
func WithMathKern(on bool) Option {
	return func(cfg *config) {
		cfg.noKern = !on
	}
}

// WithLogger sets the logger of the layout. It logs, once for each layout, a
// character that has no glyph in the face, at the level Warn (D20). The
// source has no logger, and sets such a character as an empty box.
func WithLogger(lg *slog.Logger) Option {
	return func(cfg *config) {
		cfg.logger = lg
	}
}

// Layout lays out n with face, which must have a MATH table, at size, in
// style. The size is in any unit, such as pixels or points, and every length
// of the result is in that unit. A length of n in pt counts one point as one
// unit of the size, as the source does. Layout reads face, which is not safe
// for concurrent use, so a caller gives each goroutine its own face.
//
// A face without a MATH table gives okage.ErrMathNoMathTable. A size that is
// not a positive finite number, and a tree past a limit, give an error that
// wraps okage.ErrMathLimit. The limits are those of Parse: WithMaxDepth for
// the depth of the tree, and at most 1,000,000 items in the box. A style past
// ScriptScript is ScriptScript.
func Layout(n Node, face *font.Face, size float64, style Style, opts ...Option) (*Box, error) {
	if face == nil || !face.HasMath() {
		return nil, okage.ErrMathNoMathTable
	}
	if !(size > 0) || math.IsInf(size, 1) {
		return nil, fmt.Errorf("%w: the size %g is not a positive finite number", okage.ErrMathLimit, size)
	}
	style = min(style, ScriptScript)
	l := &layout{
		face:  face,
		size:  size,
		base:  size,
		upem:  float64(face.Upem()),
		cfg:   newConfig(opts),
		color: okage.ColorBlack,
	}
	if l.cfg.hasColor {
		l.color = l.cfg.color
	}
	b := l.node(n, style)
	if l.err != nil {
		return nil, l.err
	}
	return &Box{Width: b.w, Height: b.h, Depth: b.d, Italic: b.ic, Items: b.items}, nil
}

// LayoutTeX parses tex and lays out its tree, for a caller that has TeX
// only. It is RenderSVGMetrics and RenderDisplaySVGMetrics of the source,
// with items in place of SVG text. An error of Parse is a *SyntaxError.
func LayoutTeX(tex string, face *font.Face, size float64, style Style, opts ...Option) (*Box, error) {
	if face == nil || !face.HasMath() {
		return nil, okage.ErrMathNoMathTable
	}
	tree, err := Parse(tex, opts...)
	if err != nil {
		return nil, err
	}
	return Layout(tree, face, size, style, opts...)
}

// layout holds the state of one call of Layout. It is the engine of the
// source.
type layout struct {
	face  *font.Face
	size  float64 // the size of display and text style
	base  float64 // the size that Layout gets, of which a SizeChange is a factor
	upem  float64
	cfg   config
	color okage.Color // the color of the items that are made

	depth   int
	items   int
	err     error
	missed  map[rune]bool // the characters without a glyph that were logged
	xHeight map[float64]float64

	shaper shaping.HarfbuzzShaper // the shaper of the text faces, in text.go

	// middle is the height of a delimiter of \middle in the body of the
	// Delimited that the walk is in, and 0 outside one. measuring reports
	// the pass that measures such a body: its delimiters of \middle take
	// their natural size.
	middle    float64
	measuring bool
}

// fail records the first error of the layout. The walk then stops.
func (l *layout) fail(err error) {
	if l.err == nil {
		l.err = err
	}
}

// count counts one item, and reports false past the limit of the items.
func (l *layout) count() bool {
	l.items++
	if l.items > maxItems {
		l.fail(fmt.Errorf("%w: the box has more than %d items", okage.ErrMathLimit, maxItems))
		return false
	}
	return true
}

// missing logs the character r, which has no glyph in the face.
func (l *layout) missing(r rune) {
	if l.cfg.logger == nil || l.missed[r] {
		return
	}
	if l.missed == nil {
		l.missed = map[rune]bool{}
	}
	l.missed[r] = true
	l.cfg.logger.Warn("texmath: the face has no glyph for a character", "char", string(r), "code", fmt.Sprintf("U+%04X", r))
}

// enter goes one level deeper into the tree, and reports false past the
// limit of the depth. leave goes back.
//
// The parser counts a level for each step of its recursion. A tree that it
// makes is at most twice as deep as its levels: \over makes a fraction and
// its denominator at the level of the list, and a style command makes a
// StyleChange around the rest of the list. So the walk allows twice the
// depth of WithMaxDepth. A StyleChange takes no level, because the walk
// unwraps it in a loop. A level of the walk takes three small frames of
// the stack: node, nested and the function of the node.
func (l *layout) enter() bool {
	if l.err != nil {
		return false
	}
	l.depth++
	if l.depth > 2*l.cfg.maxDepth {
		l.fail(fmt.Errorf("%w: the depth of nesting is more than %d", okage.ErrMathLimit, l.cfg.maxDepth))
		return false
	}
	return true
}

func (l *layout) leave() { l.depth-- }

// node lays out n in the style st. A nil node is an empty Ord box.
//
// The switches of the walk do not bind the node to a variable of its type,
// and each function asserts its own type and is not inlined. The functions
// that make the boxes, in math.go and the other files, are not inlined
// either. TinyGo does not share the slots of the stack of the variables of
// inlined functions, or of the cases of a switch that binds, so a level of
// the walk took more than 500 bytes of the stack of 64 KiB of a goroutine
// on WebAssembly. Now it takes about 200, and the deepest trees that the
// walk allows lay out with TinyGo 0.44 for wasip1.
//
//go:noinline
func (l *layout) node(n Node, st Style) *box {
	n, st = unwrapStyle(n, st)
	switch n.(type) {
	case nil:
		return newBox(Ord)
	case Space:
		return l.space(n, st)
	case Rule:
		return l.layoutRule(n, st)
	case Delimiter:
		return l.delimiter(n, st)
	case TextRun:
		return l.textRun(n, st)
	}
	if !l.enter() {
		return newBox(Ord)
	}
	b := l.nested(n, st)
	l.leave()
	return b
}

// nested lays out a node that holds other nodes. It is not inlined, so that
// its frame is not on the stack of a leaf.
//
//go:noinline
func (l *layout) nested(n Node, st Style) *box {
	switch n.(type) {
	case List:
		return l.list(n, st)
	case Atom:
		return l.atom(n, st)
	case Scripts:
		return l.scripts(n, st)
	case Fraction:
		return l.layoutFraction(n, st)
	case Radical:
		return l.layoutRadical(n, st)
	case Delimited:
		return l.layoutDelimited(n, st)
	case Accent:
		return l.layoutAccent(n, st)
	case OverUnder:
		return l.layoutOverUnder(n, st)
	case Bar:
		return l.layoutBar(n, st)
	case Array:
		return l.layoutArray(n, st)
	case Color:
		return l.layoutColor(n, st)
	case Phantom:
		return l.layoutPhantom(n, st)
	case Raise:
		return l.layoutRaise(n, st)
	case Scale:
		return l.layoutScale(n, st)
	case Boxed:
		return l.layoutBoxed(n, st)
	case Negate:
		return l.layoutNegate(n, st)
	case Brace:
		return l.layoutBrace(n, st)
	case OverArrow:
		return l.layoutOverArrow(n, st)
	case XArrow:
		return l.layoutXArrow(n, st)
	case Lap:
		return l.layoutLap(n, st)
	case Overlay:
		return l.layoutOverlay(n, st)
	case Choice:
		return l.layoutChoice(n, st)
	case Cancel:
		return l.layoutCancel(n, st)
	case Phase:
		return l.layoutPhase(n, st)
	case SizeChange:
		return l.layoutSize(n, st)
	case Tag:
		return l.layoutTag(n, st)
	}
	return l.nestedMore(n, st)
}

// nestedMore lays out the nodes of D95. It is not inlined, and nested does
// not hold its cases, so that the frame of nested does not grow on the
// stack of a nested formula.
//
//go:noinline
func (l *layout) nestedMore(n Node, st Style) *box {
	switch n.(type) {
	case Stretch:
		return l.layoutStretch(n, st)
	case VCenter:
		return l.layoutVCenter(n, st)
	case Angl:
		return l.layoutAngl(n, st)
	case Sideset:
		return l.layoutSideset(n, st)
	}
	return newBox(Ord)
}

// listFrame is a list whose items the walk reads, with its style and its
// color.
type listFrame struct {
	items List
	i     int
	st    Style
	color okage.Color
}

// list lays out the items of a List on one baseline, with the space between
// the atoms. A StyleChange in the list sets its items in its style, and they
// join the list for the space between atoms, as a style command does in TeX.
// The items of a Color join the list in the same way. A Space is not an atom.
// A Choice whose branch is empty makes no atom, so it does not change the
// space between its neighbors.
//
// The class of the list, as a part of a formula, is the class of its only
// item when it has one, and Ord in the other cases.
//
//go:noinline
func (l *layout) list(node Node, st Style) *box {
	n, _ := node.(List)
	var entries []entry
	saved := l.color
	stack := []listFrame{{items: n, st: st, color: l.color}}
	for len(stack) > 0 && l.err == nil {
		f := &stack[len(stack)-1]
		if f.i >= len(f.items) {
			stack = stack[:len(stack)-1]
			continue
		}
		item, ist := unwrapStyle(f.items[f.i], f.st)
		_, styled := f.items[f.i].(StyleChange)
		f.i++
		if items, ok := item.(List); ok && styled {
			// The items of a StyleChange join this list.
			stack = append(stack, listFrame{items: items, st: ist, color: f.color})
			continue
		}
		if c, ok := item.(Color); ok {
			// okage: the items of a Color join this list, in its color.
			items, ok := c.Body.(List)
			if !ok {
				items = List{c.Body}
			}
			stack = append(stack, listFrame{items: items, st: ist, color: c.Color})
			continue
		}
		l.color = f.color
		switch item.(type) {
		case nil:
		case Space:
			entries = append(entries, entry{b: l.space(item, ist), st: ist})
		case Choice:
			if emptyChoice(item, ist) {
				continue
			}
			entries = append(entries, entry{b: l.node(item, ist), st: ist, atom: true})
		default:
			entries = append(entries, entry{b: l.node(item, ist), st: ist, atom: true})
		}
	}
	l.color = saved
	return l.hlist(entries)
}

// unwrapStyle returns the node in the StyleChange nodes around n, and its
// style. A StyleChange takes no level of the walk.
func unwrapStyle(n Node, st Style) (Node, Style) {
	for {
		sc, ok := n.(StyleChange)
		if !ok {
			return n, st
		}
		n, st = sc.Body, min(sc.Style, ScriptScript)
	}
}

// emptyChoice reports whether the branch of the Choice n in the style st is
// empty.
func emptyChoice(n Node, st Style) bool {
	c, _ := n.(Choice)
	return len(c.branch(st)) == 0
}

// layoutBoxed lays out \boxed, \fbox, \colorbox and \fcolorbox.
//
//go:noinline
func (l *layout) layoutBoxed(n Node, st Style) *box {
	b, _ := n.(Boxed)
	return l.boxed(l.node(b.Body, st), st, &b)
}

// layoutNegate lays out \not.
//
//go:noinline
func (l *layout) layoutNegate(n Node, st Style) *box {
	g, _ := n.(Negate)
	return l.negate(l.node(g.Body, st), st)
}

// layoutLap lays out \lefteqn, \rlap, \llap and \clap: the body with a width
// of zero, which overhangs to the right, to the left or to both sides.
//
//go:noinline
func (l *layout) layoutLap(n Node, st Style) *box {
	p, _ := n.(Lap)
	b := l.node(p.Body, st)
	switch p.Side {
	case LapLeft:
		translateItems(b.items, -b.w, 0)
	case LapCenter:
		translateItems(b.items, -b.w/2, 0)
	}
	b.w, b.cls, b.ic = 0, Ord, 0
	return b
}

// atom lays out an Atom: its characters through its Variant, or its body,
// with the class of the atom.
//
//go:noinline
func (l *layout) atom(n Node, st Style) *box {
	a, _ := n.(Atom)
	px := l.px(st)
	var b *box
	switch {
	case a.Text != "":
		r, size := utf8.DecodeRuneInString(a.Text)
		switch {
		case a.Large && size == len(a.Text):
			// okage: a large operator is centered on the math axis, in each
			// style (Appendix G, rule 13). The source sets it on the
			// baseline, so the variant of display style hung from the
			// baseline (parse.go:1007).
			r = mapVariant(r, a.Variant)
			b = l.mustGlyph(r, px, a.Class)
			if st == Display {
				b = l.displayOperator(r, px, a.Class, b)
			}
			b = l.axisCentre(b, px)
		default:
			b = l.opName(a.Text, a.Variant, px)
		}
	case a.Body != nil:
		b = l.node(a.Body, st)
	default:
		b = newBox(a.Class)
	}
	b.cls = a.Class
	return b
}

// scripts lays out the scripts and the primes of a nucleus. A large
// operator takes its scripts as limits in display style, or always after
// \limits. This is the end of the loop of parseList of the source.
//
//go:noinline
func (l *layout) scripts(n Node, st Style) *box {
	s, _ := n.(Scripts)
	nuc := l.node(s.Base, st)
	sst := scriptOf(st)
	var sup, sub *box
	if s.Sup != nil {
		sup = l.node(s.Sup, sst)
	}
	if s.Sub != nil {
		sub = l.node(s.Sub, sst)
	}
	if s.Primes > 0 {
		pb := l.primeBox(s.Primes, sst)
		if sup == nil {
			sup = pb
		} else {
			sup = l.hlist([]entry{{b: pb, st: sst, atom: true}, {b: sup, st: sst, atom: true}})
		}
	}
	if sup == nil && sub == nil {
		return nuc
	}
	if _, ok := unwrapBrace(s.Base); ok {
		return l.attachBraceLabels(nuc, sup, sub, st)
	}
	if takesLimits(s.Base, st) {
		return l.attachLimits(nuc, sup, sub, st)
	}
	return l.attachScripts(nuc, sup, sub, st)
}

// takesLimits reports whether the scripts of the nucleus n go above and
// below it in the style st: n is an Atom of class Op, with \limits, or in
// display style without \nolimits.
func takesLimits(n Node, st Style) bool {
	for {
		sc, ok := n.(StyleChange)
		if !ok {
			break
		}
		n, st = sc.Body, sc.Style
	}
	a, ok := n.(Atom)
	if !ok || a.Class != Op {
		return false
	}
	return a.Limits == LimitsAbove || (a.Limits == LimitsAuto && st == Display)
}

// unwrapBrace returns the Brace in the StyleChange nodes around n, and false
// when n is not a Brace.
func unwrapBrace(n Node) (Brace, bool) {
	n, _ = unwrapStyle(n, Text)
	b, ok := n.(Brace)
	return b, ok
}

// length returns the length v in the unit of the size, in the style st.
// The source reads ex as the height of the glyph "x" of the face, and 0 when
// the face has none (e.dimen).
func (l *layout) length(v Length, st Style) float64 {
	px := l.px(st)
	switch v.Unit {
	case Em:
		return float64(v.Value * px)
	case Ex:
		return float64(v.Value * l.exHeight(px))
	case Mu:
		return float64(v.Value*px) / 18
	}
	return v.Value
}

// exHeight returns the height of the glyph "x" at the size px.
func (l *layout) exHeight(px float64) float64 {
	if h, ok := l.xHeight[px]; ok {
		return h
	}
	h := 0.0
	if gid, ok := l.face.NominalGlyph('x'); ok {
		h, _ = l.gidVExtent(gid, px)
	}
	if l.xHeight == nil {
		l.xHeight = map[float64]float64{}
	}
	l.xHeight[px] = h
	return h
}

// space lays out a Space as an empty box of its width.
//
//go:noinline
func (l *layout) space(node Node, st Style) *box {
	n, _ := node.(Space)
	return l.kern(l.length(n.Width, st))
}

// delimiter lays out a Delimiter: its glyph on the baseline at the natural
// size, or grown to its size and centered on the math axis, as the \big
// family and \middle. It is the layout of the \big family and of \middle in
// parseControl of the source, and of braWrap.
//
// okage: the null delimiter "." is an empty box. The source draws the glyph
// .notdef for \big. (parse.go:983).
//
//go:noinline
func (l *layout) delimiter(n Node, st Style) *box {
	d, _ := n.(Delimiter)
	px := l.px(st)
	if d.Rune == 0 {
		return newBox(d.Class)
	}
	if d.Middle && l.measuring {
		return l.mustGlyph(d.Rune, px, d.Class)
	}
	if d.Middle && l.middle > 0 {
		// okage: \middle grows as its \left and \right do.
		return l.axisCentre(l.stretchVertical(d.Rune, l.middle, px, d.Class), px)
	}
	if !(d.Size > 0) {
		return l.mustGlyph(d.Rune, px, d.Class)
	}
	return l.axisCentre(l.stretchVertical(d.Rune, float64(min(d.Size, maxLength)*px), px, d.Class), px)
}

// layoutFraction lays out a Fraction. Its parts are one style down, by
// fracInner. \atop and \binom have no rule, and \binom and \genfrac can put
// delimiters around it. It is the layout of \frac, \over, \atop, \choose,
// \binom and \genfrac in parseControl and parseList of the source.
//
//go:noinline
func (l *layout) layoutFraction(n Node, st Style) *box {
	f, _ := n.(Fraction)
	inner := fracInner(st)
	num := l.node(f.Num, inner)
	den := l.node(f.Den, inner)
	var b *box
	if f.NoRule {
		b = l.binom(num, den, st)
	} else {
		b = l.fraction(num, den, st, f.NumAlign)
	}
	if f.Open != 0 || f.Close != 0 {
		b = l.delimited(b, f.Open, f.Close, st)
	}
	b.cls = Inner
	return b
}

// alignNumerator returns num in a box as wide as den, at its left for
// AlignLeft and at its right for AlignRight, for \cfrac[l] and \cfrac[r]
// (D95). A numerator that is not narrower than den stays as it is. fraction
// calls it, and not layoutFraction, because the frame of layoutFraction is
// on the stack of each nested fraction, and the deepest chain of \over must
// still lay out in the stack of 64 KiB of a goroutine of TinyGo.
//
//go:noinline
func alignNumerator(num, den *box, align Align) *box {
	if num.w >= den.w {
		return num
	}
	wide := newBox(num.cls)
	wide.w, wide.h, wide.d = den.w, num.h, num.d
	x := 0.0
	if align == AlignRight {
		x = den.w - num.w
	}
	place(wide, num, x, 0)
	return wide
}

// layoutRadical lays out a Radical. The degree is in scriptscript style:
// "\setbox\rootbox\hbox{$\m@th\scriptscriptstyle{#1}$}" (latex.ltx:11187).
//
//go:noinline
func (l *layout) layoutRadical(n Node, st Style) *box {
	r, _ := n.(Radical)
	body := l.node(r.Body, st)
	var degree *box
	if r.Degree != nil {
		degree = l.node(r.Degree, ScriptScript)
	}
	return l.radical(body, degree, st)
}

// layoutDelimited lays out \left and \right.
//
// okage: a body with a delimiter of \middle among its items is laid out
// twice. The first pass measures it, with the delimiters of \middle at their
// natural size, and the second sets them at the size of the delimiters of
// \left and \right, as e-TeX does. A Delimited in the first pass takes one
// pass, so a nest of them takes time in the square of its depth, not in a
// power of two. The source sets \middle at 1.8 em.
//
//go:noinline
func (l *layout) layoutDelimited(n Node, st Style) *box {
	d, _ := n.(Delimited)
	if l.measuring || !hasMiddle(d.Body) {
		saved := l.middle
		l.middle = 0
		inner := l.node(d.Body, st)
		l.middle = saved
		return l.delimited(inner, d.Open, d.Close, st)
	}
	items, saved := l.items, l.middle
	l.measuring = true
	measured := l.node(d.Body, st)
	l.measuring = false
	l.items = items
	l.middle = l.delimTarget(measured, l.px(st))
	inner := l.node(d.Body, st)
	l.middle = saved
	return l.delimited(inner, d.Open, d.Close, st)
}

// hasMiddle reports whether a delimiter of \middle is an item of the List n,
// or of a StyleChange or a Color among its items, which join the list.
func hasMiddle(n Node) bool {
	stack := []Node{n}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch n := n.(type) {
		case List:
			stack = append(stack, n...)
		case StyleChange:
			stack = append(stack, n.Body)
		case Color:
			stack = append(stack, n.Body)
		case Delimiter:
			if n.Middle {
				return true
			}
		}
	}
	return false
}

// layoutSize lays out a SizeChange: its body at its factor of the size of the
// layout.
//
//go:noinline
func (l *layout) layoutSize(n Node, st Style) *box {
	s, _ := n.(SizeChange)
	f := s.Factor
	if !(f >= 0.01) {
		f = 0.01 // also for NaN
	}
	saved := l.size
	l.size = float64(l.base * min(f, maxScale))
	b := l.node(s.Body, st)
	l.size = saved
	return b
}

// layoutAccent lays out an Accent.
//
//go:noinline
func (l *layout) layoutAccent(n Node, st Style) *box {
	a, _ := n.(Accent)
	return l.accent(l.node(a.Base, st), a.Accent, a.Stretch, st)
}

// layoutOverUnder lays out an OverUnder. Over and Under are in script style.
// The source sets one of them only. With both, the base takes Over and then
// Under.
//
//go:noinline
func (l *layout) layoutOverUnder(n Node, st Style) *box {
	o, _ := n.(OverUnder)
	b := l.node(o.Base, st)
	b.cls = o.Class
	sst := scriptOf(st)
	if o.Over != nil {
		b = l.overUnder(b, l.node(o.Over, sst), true, st, o.Class)
	}
	if o.Under != nil {
		b = l.overUnder(b, l.node(o.Under, sst), false, st, o.Class)
	}
	return b
}

// layoutBar lays out \overline and \underline.
//
//go:noinline
func (l *layout) layoutBar(n Node, st Style) *box {
	bar, _ := n.(Bar)
	b := l.node(bar.Base, st)
	if bar.Under {
		return l.underline(b, st)
	}
	return l.overline(b, st)
}

// layoutArray lays out an Array: its cells, then the grid of its kind. The
// cells of smallmatrix and the rows of \substack are in script style, as
// \scriptstyle in their templates of amsmath sets. The cells of the others
// are in text style, or in the style in force when it is smaller.
//
// okage: \substack sets its rows in script style. The source sets them one
// style below the style in force, so a \substack in a subscript was at the
// scriptscript size (parse.go:935).
//
//go:noinline
func (l *layout) layoutArray(n Node, st Style) *box {
	a, _ := n.(Array)
	cst := cellStyle(st)
	if a.Kind == ArraySmallMatrix || a.Kind == ArraySubstack {
		cst = Script
	}
	rows := make([][]*box, len(a.Rows))
	for i, r := range a.Rows {
		rows[i] = make([]*box, len(r))
		for j, c := range r {
			rows[i][j] = l.node(c, cst)
		}
	}
	return l.finishEnv(&a, rows, cst, st)
}

// layoutColor lays out Body in the color of a Color node. The box keeps the
// class of Body.
//
//go:noinline
func (l *layout) layoutColor(n Node, st Style) *box {
	c, _ := n.(Color)
	saved := l.color
	l.color = c.Color
	b := l.node(c.Body, st)
	l.color = saved
	return b
}

// layoutPhantom lays out \phantom, \hphantom, \vphantom and \mathstrut.
//
//go:noinline
func (l *layout) layoutPhantom(n Node, st Style) *box {
	p, _ := n.(Phantom)
	return l.phantom(l.node(p.Body, st), p.Horizontal, p.Vertical)
}

// layoutBrace lays out \overbrace and \underbrace.
//
//go:noinline
func (l *layout) layoutBrace(n Node, st Style) *box {
	b, _ := n.(Brace)
	return l.brace(l.node(b.Base, st), b.Rune, !b.Under, st)
}

// layoutOverArrow lays out \overrightarrow and its kin.
//
//go:noinline
func (l *layout) layoutOverArrow(n Node, st Style) *box {
	a, _ := n.(OverArrow)
	b := l.node(a.Base, st)
	if a.Under || a.Harpoon || a.Segment {
		return l.arrowOverUnder(b, &a, st)
	}
	return l.overArrow(b, a.Left, a.Right, st)
}

// layoutXArrow lays out \xrightarrow and its kin. An arrow that xArrowSpec
// does not know, from a tree that a program makes, is a right arrow.
//
//go:noinline
func (l *layout) layoutXArrow(n Node, st Style) *box {
	a, _ := n.(XArrow)
	sst := scriptOf(st)
	var sup, sub *box
	if a.Over != nil {
		sup = l.node(a.Over, sst)
	}
	if a.Under != nil {
		sub = l.node(a.Under, sst)
	}
	if b := l.xArrowMore(sup, sub, a.Arrow, st); b != nil {
		return b
	}
	left, right, hooked, mapsto, ok := xArrowSpec(a.Arrow)
	if !ok {
		right = true
	}
	return l.xArrow(sup, sub, left, right, hooked, mapsto, st)
}
