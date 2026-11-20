// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// The source lays out each command as it parses it. This file holds the parse
// half of each command of parse.go of the source: it makes the nodes of
// node.go. The layout half of each command, such as e.fraction and e.radical,
// is in math.go, and layout.go walks the tree. A comment at each command
// names the function of the source that holds its layout half. The layout
// halves of the helpers of parse.go of the source, e.kern, e.primeBox,
// e.mustGlyph and e.opName, are at the end of this file.

// The limits of D78 section 5. They are additions of okage.
const (
	// defaultMaxDepth is the depth of nesting of groups, arguments and
	// environments. Each step of the recursion of the parser is one level: a
	// list, an atom, a command, an argument, and the scripts of a nucleus.
	// A goroutine of TinyGo for wasip1 has a stack of 64 KiB that does not
	// grow. There, the deepest input of 100 levels takes at most 27 KiB of
	// it, as TestParseStackUse measures, which leaves room for the caller
	// and the garbage collector.
	defaultMaxDepth = 100

	// defaultMaxNodes is the number of nodes that the parser makes.
	defaultMaxNodes = 100000

	// defaultMaxExpansions is the budget of macro expansions of the source.
	defaultMaxExpansions = 100000

	// maxTokens is the number of tokens of the input, before and after the
	// expansion of the macros.
	maxTokens = 1 << 20

	// maxLength is the largest absolute value of a length in pt, em or ex,
	// about the \maxdimen of TeX. A length in mu is at most maxLength em.
	maxLength = 16384

	// maxScale is the largest absolute value of a factor of \scalebox.
	maxScale = 100
)

// Option changes a limit of Parse, or a setting of the layout.
type Option func(*config)

// config holds the settings that the options change.
type config struct {
	maxDepth      int
	maxNodes      int
	maxExpansions int

	// The settings of the layout, in layout.go and text.go.
	color     okage.Color
	hasColor  bool
	logger    *slog.Logger
	textFaces [3][4]*font.Face // the faces of WithTextFamily: regular, bold, italic and bold italic

	// The additions of phase 26 of D78 that an option turns off, for the
	// comparison with the source.
	noItalic bool // WithItalicCorrection(false)
	noAttach bool // WithAccentAttachment(false)
	noKern   bool // WithMathKern(false)
}

// newConfig returns the defaults, changed by opts.
func newConfig(opts []Option) config {
	c := config{
		maxDepth:      defaultMaxDepth,
		maxNodes:      defaultMaxNodes,
		maxExpansions: defaultMaxExpansions,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// WithMaxDepth sets the largest depth of nesting of groups, arguments and
// environments. Each step of the recursion of the parser is one level: a
// list, an atom, a command, an argument, and the scripts of a nucleus. So x
// in {{x}} is six levels deep: the outer list, two groups with their lists,
// and x. The default is 100. A value below 1 keeps the default.
func WithMaxDepth(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxDepth = n
		}
	}
}

// WithMaxNodes sets the largest number of nodes of a tree that Parse makes.
// The default is 100,000. A value below 1 keeps the default.
func WithMaxNodes(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxNodes = n
		}
	}
}

// WithMaxExpansions sets the budget of macro expansions. The default is
// 100,000, the budget of the source. When the budget is spent, a macro is
// left as it is, and the parser reports it as an unknown command. A value
// below 0 keeps the default.
func WithMaxExpansions(n int) Option {
	return func(c *config) {
		if n >= 0 {
			c.maxExpansions = n
		}
	}
}

// SyntaxError is the error of Parse. It wraps okage.ErrTeXMathSyntax for TeX
// math that the parser cannot read. For an unknown command or environment, it
// also wraps okage.ErrTeXMathUnknownCommand, and Command names it. For a
// formula past a limit, it wraps okage.ErrMathLimit only.
type SyntaxError struct {
	// Pos is the byte offset of the fault in the TeX source. A fault at the
	// end of the source has the length of the source.
	Pos int

	// Msg says what is wrong, with the words of go-tex/math.
	Msg string

	// Command is the unknown command, such as `\foo`, or the unknown
	// environment, such as `\begin{foo}`. It is empty for the other faults.
	Command string

	// err is the sentinel error.
	err okage.Error
}

// Error returns the message, such as `parsing tex math: unknown command
// \foo`.
func (e *SyntaxError) Error() string {
	if e.err == okage.ErrMathLimit {
		return string(okage.ErrMathLimit) + ": " + e.Msg
	}
	return string(okage.ErrTeXMathSyntax) + ": " + e.Msg
}

// Unwrap returns the sentinel errors of the fault.
func (e *SyntaxError) Unwrap() []error {
	if e.err == okage.ErrTeXMathUnknownCommand {
		return []error{okage.ErrTeXMathSyntax, okage.ErrTeXMathUnknownCommand}
	}
	return []error{e.err}
}

// Parse reads TeX math, such as `\frac{a}{b}`, and returns its tree. An
// unknown command is an error that names it. Parse lays nothing out, so it
// needs no font. An error of Parse is a *SyntaxError.
func Parse(tex string, opts ...Option) (List, error) {
	p := &parser{tex: tex, cfg: newConfig(opts)}
	toks := tokenize(tex)
	if len(toks) > maxTokens {
		return nil, p.limit(nil, fmt.Sprintf("the source has more than %d tokens", maxTokens))
	}
	toks, err := p.expandMacros(resolveConditionals(toks))
	if err != nil {
		return nil, err
	}
	list, _, err := p.parseList(toks, alphabet{}, stopEnd)
	if err != nil {
		return nil, err
	}
	if p.tag != nil {
		// okage: \tag labels the whole formula.
		list = p.list([]Node{p.add(Tag{Body: list, Label: p.tag})})
	}
	if p.nodes > p.cfg.maxNodes {
		return nil, p.limit(nil, fmt.Sprintf("the formula has more than %d nodes", p.cfg.maxNodes))
	}
	return list, nil
}

// parser holds the state of one call of Parse.
type parser struct {
	tex   string
	cfg   config
	depth int  // the depth of nesting
	nodes int  // the nodes made
	tag   Node // the label of \tag, in katex.go
}

// pos returns the byte offset of the first token of toks, or the end of the
// source.
func (p *parser) pos(toks []token) int {
	if len(toks) == 0 {
		return len(p.tex)
	}
	return toks[0].pos
}

// fail returns a syntax error at the first token of toks.
func (p *parser) fail(toks []token, msg string) error {
	return &SyntaxError{Pos: p.pos(toks), Msg: msg, err: okage.ErrTeXMathSyntax}
}

// limit returns the error of a limit at the first token of toks.
func (p *parser) limit(toks []token, msg string) error {
	return &SyntaxError{Pos: p.pos(toks), Msg: msg, err: okage.ErrMathLimit}
}

// unknown returns the error of an unknown command or environment.
func (p *parser) unknown(pos int, msg, command string) error {
	return &SyntaxError{Pos: pos, Msg: msg, Command: command, err: okage.ErrTeXMathUnknownCommand}
}

// enter goes one level deeper, and checks the limits of the depth and of the
// nodes. leave goes back.
func (p *parser) enter(toks []token) error {
	p.depth++
	if p.depth > p.cfg.maxDepth {
		return p.limit(toks, fmt.Sprintf("the depth of nesting is more than %d", p.cfg.maxDepth))
	}
	if p.nodes > p.cfg.maxNodes {
		return p.limit(toks, fmt.Sprintf("the formula has more than %d nodes", p.cfg.maxNodes))
	}
	return nil
}

func (p *parser) leave() { p.depth-- }

// add counts the node n and returns it.
func (p *parser) add(n Node) Node {
	p.nodes++
	return n
}

// list counts a List of items and returns it.
func (p *parser) list(items []Node) List {
	p.nodes++
	return List(items)
}

// ── tokens ──────────────────────────────────────────────────────────────────

type tokenKind uint8

const (
	tChar   tokenKind = iota // a single rune
	tCtrl                    // a control sequence \name (or control symbol)
	tSup                     // ^
	tSub                     // _
	tLBrace                  // {
	tRBrace                  // }
	tAmp                     // & (matrix column separator)
	tPrime                   // ' (prime)
)

type token struct {
	kind tokenKind
	text string
	r    rune

	// okage: pos is the byte offset of the token in the source, for the
	// position of an error. A token of the body of a macro has the offset of
	// the body.
	pos int

	// okage: space reports white space before the token that TeX does not
	// skip. TeX skips the white space after a control word, after a control
	// space and after a comment. The source drops every space, and \text
	// keeps them with this flag.
	space bool
}

// tokenize splits s into tokens.
//
// okage: a carriage return is white space, as TeX reads an end of line. The
// source sets it as a character. The tokens record their position and the
// white space before them.
func tokenize(s string) []token {
	var out []token
	space, skip := false, false
	emit := func(t token, pos int) {
		t.pos, t.space = pos, space
		out = append(out, t)
		space, skip = false, false
	}
	for i := 0; i < len(s); {
		c, size := utf8.DecodeRuneInString(s[i:])
		start := i
		switch c {
		case ' ', '\t', '\n', '\r':
			if !skip {
				space = true
			}
			i += size
		case '%':
			// latex.ltx:153 is \catcode`\%=14: % is a comment, to the end of the
			// line. TeX also drops the end of the line and the white space at
			// the start of the next line. A literal percent is \%, which the
			// symbol table serves.
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				i++
			}
			skip = true
		case '~':
			// latex.ltx:306 makes ~ active, and the active ~ is \nobreakspace
			// (latex.ltx:6621): a space between words, not a tilde. The control
			// space gives it, 6mu in math.
			emit(token{kind: tCtrl, text: " "}, start)
			i += size
		case '^':
			emit(token{kind: tSup}, start)
			i += size
		case '_':
			emit(token{kind: tSub}, start)
			i += size
		case '{':
			emit(token{kind: tLBrace}, start)
			i += size
		case '}':
			emit(token{kind: tRBrace}, start)
			i += size
		case '&':
			emit(token{kind: tAmp}, start)
			i += size
		case '\'':
			emit(token{kind: tPrime}, start)
			i += size
		case '\\':
			i += size
			if i < len(s) {
				if d, dsize := utf8.DecodeRuneInString(s[i:]); !isCtrlLetter(d) {
					// A control symbol, such as \{ \\ \, or \|.
					i += dsize
					emit(token{kind: tCtrl, text: string(d)}, start)
					// TeX skips the white space after a control space.
					skip = d == ' '
					continue
				}
			}
			name := i
			for i < len(s) && isCtrlLetter(rune(s[i])) {
				i++
			}
			// latex.ltx:115 is \let\bgroup={ \let\egroup=}: they are the brace
			// tokens, so the scanner gives them.
			switch s[name:i] {
			case "bgroup":
				emit(token{kind: tLBrace}, start)
			case "egroup":
				emit(token{kind: tRBrace}, start)
			default:
				emit(token{kind: tCtrl, text: s[name:i]}, start)
			}
			skip = true
		default:
			emit(token{kind: tChar, text: string(c), r: c}, start)
			i += size
		}
	}
	return out
}

// isCtrlLetter reports whether r can be in a control word. TeX reads a control
// word as a run of letters, and LaTeX makes @ a letter in package code, so
// \the@inst and \@ifundefined are one control sequence each.
func isCtrlLetter(r rune) bool { return isLetter(r) || r == '@' }

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// resolveConditionals evaluates \ifmmode at the token level. This parser is
// always in math mode, so \ifmmode is always true: it keeps the true branch,
// drops the \else branch, and removes the \ifmmode, \else and \fi markers,
// nested or not. A stray \else or \fi is dropped. The engine reaches this
// with macros such as \ensuremath, which is \ifmmode#1\else$#1$\fi.
//
// okage: one pass with a stack, in time linear in the tokens. The source
// scans for the \fi of each \ifmmode and recurses into the true branch, which
// takes quadratic time for nested conditionals.
func resolveConditionals(toks []token) []token {
	out := make([]token, 0, len(toks))
	var inElse []bool // for each open \ifmmode, whether its \else was met
	skipping := 0     // the open conditionals that are in their \else branch
	for _, t := range toks {
		if t.kind == tCtrl {
			switch t.text {
			case "ifmmode":
				inElse = append(inElse, false)
				continue
			case "else":
				if n := len(inElse); n > 0 && !inElse[n-1] {
					inElse[n-1] = true
					skipping++
				}
				continue
			case "fi":
				if n := len(inElse); n > 0 {
					if inElse[n-1] {
						skipping--
					}
					inElse = inElse[:n-1]
				}
				continue
			}
		}
		if skipping == 0 {
			out = append(out, t)
		}
	}
	return out
}

func tokenText(t token) string {
	switch t.kind {
	case tCtrl:
		return "\\" + t.text
	case tLBrace:
		return "{"
	case tRBrace:
		return "}"
	case tSup:
		return "^"
	case tSub:
		return "_"
	case tAmp:
		return "&"
	case tPrime:
		return "'"
	default:
		return t.text
	}
}

// ── parsing ─────────────────────────────────────────────────────────────────

type stopMode uint8

const (
	stopEnd      stopMode = iota // only end of input
	stopBrace                    // stop at }
	stopRight                    // stop at \right
	stopCell                     // stop at & \\ or \end (matrix cell)
	stopStackRow                 // stop at \\ or } (a \substack line)
	stopOver                     // stop at \over (\buildrel's delimited argument)
)

// atStop reports whether toks begins with a terminator for the given mode.
func atStop(toks []token, stop stopMode) bool {
	if len(toks) == 0 {
		return true
	}
	t := toks[0]
	switch stop {
	case stopBrace:
		return t.kind == tRBrace
	case stopRight:
		return t.kind == tCtrl && t.text == "right"
	case stopOver:
		return t.kind == tCtrl && t.text == "over"
	case stopCell:
		return t.kind == tAmp || (t.kind == tCtrl && (t.text == `\` || t.text == "end"))
	case stopStackRow:
		return t.kind == tRBrace || (t.kind == tCtrl && t.text == `\`)
	default:
		return false
	}
}

// alphabet is the alphabet in force, the alpha field of the style of the
// source. The rest of that style, the size and the display flag, is the work
// of the layout.
type alphabet struct {
	set     bool // a command chose an alphabet: the alpha of the source is not nil
	text    bool // \text and its kin: characters make a TextRun
	variant Variant

	// okage: the style of the text of a TextRun, which the text commands of
	// text.go set.
	bold, italic bool
	family       Family
}

// variantOf returns the variant of a letter in the alphabet a.
func (a alphabet) variantOf() Variant {
	if !a.set {
		return Italic
	}
	return a.variant
}

// isMathDelim reports whether t is $, \(, \), \[ or \]. latex.ltx says that
// \( is \relax\ifmmode\@badmath\else$\fi (11298) and \[ the same with $$
// (11302). Here the source is already in math, so the delimiter is
// redundant. It reaches the parser when the engine lifts text such as
// \scalebox{0.5}{$a+b$} into math.
func isMathDelim(t token) bool {
	if t.kind == tChar && t.r == '$' {
		return true
	}
	if t.kind == tCtrl {
		switch t.text {
		case "(", ")", "[", "]":
			return true
		}
	}
	return false
}

// textSymbols are the control sequences that \text sets as a character.
var textSymbols = map[string]string{
	" ": " ", "%": "%", "#": "#", "&": "&", "_": "_", "$": "$", "{": "{", "}": "}",
}

// textOf returns the characters of t in a TextRun, and false when t is not
// text. okage: in \text, an apostrophe is a character and not a prime.
func textOf(t token) (string, bool) {
	switch t.kind {
	case tChar:
		return t.text, true
	case tPrime:
		return "'", true
	case tCtrl:
		s, ok := textSymbols[t.text]
		return s, ok
	}
	return "", false
}

// parseList parses a run of atoms, each with its scripts, limits and primes,
// into a List. It stops at the terminator of stop, which it does not consume.
func (p *parser) parseList(toks []token, st alphabet, stop stopMode) (List, []token, error) {
	if err := p.enter(toks); err != nil {
		return nil, nil, err
	}
	l, rest, err := p.parseListIn(toks, st, stop)
	p.leave()
	return l, rest, err
}

// listBuilder holds the state of a list that parseListIn reads.
type listBuilder struct {
	p     *parser
	items []Node

	toks []token  // the tokens still to read
	st   alphabet // the alphabet in force
	stop stopMode

	// saved is the alphabet that a math delimiter in \text put aside.
	saved    alphabet
	hasSaved bool

	nuc    Node  // the next nucleus, for listNucleus
	result List  // the list, for listDone
	err    error // the error, for listDone

	// okage: the text of \text, with its spaces, becomes a TextRun.
	run   strings.Builder
	inRun bool

	// A style command applies to the rest of the list. styled is the index of
	// the first item after the last style command, or -1.
	styled int
	style  Style

	// overs holds the numerators of the \over commands of the list.
	overs []overPart
}

func (b *listBuilder) text(s string) {
	b.inRun = true
	b.run.WriteString(s)
}

func (b *listBuilder) flush() {
	if b.inRun {
		b.items = append(b.items, b.p.add(b.st.run(b.run.String())))
		b.run.Reset()
		b.inRun = false
	}
}

func (b *listBuilder) closeStyle() {
	b.flush()
	if b.styled >= 0 && len(b.items) > b.styled {
		body := b.p.list(append([]Node(nil), b.items[b.styled:]...))
		b.items = append(b.items[:b.styled], b.p.add(StyleChange{Style: b.style, Body: body}))
	}
	b.styled = -1
}

func (b *listBuilder) setStyle(s Style) {
	b.closeStyle()
	b.styled, b.style = len(b.items), s
}

func (b *listBuilder) finish() List {
	b.closeStyle()
	return b.p.list(b.items)
}

// done returns the list, with the fractions of the \over commands that
// startOver read, the last one innermost.
//
//go:noinline
func (b *listBuilder) done() List {
	l := b.finish()
	p := b.p
	for _, o := range slices.Backward(b.overs) {
		f := Fraction{Num: o.num, Den: l}
		switch o.cmd {
		case "atop":
			f.NoRule = true
		case "choose":
			f.NoRule, f.Open, f.Close = true, '(', ')'
		}
		l = p.list([]Node{p.add(f)})
		p.leave()
	}
	return l
}

// parseListIn is the body of parseList, after the count of the level.
//
// okage: the state of the list is on the heap, in a listBuilder, so that a
// level of nesting takes little of the stack. A goroutine of TinyGo has a
// stack of 64 KiB.
func (p *parser) parseListIn(toks []token, st alphabet, stop stopMode) (List, []token, error) {
	b := &listBuilder{p: p, styled: -1, toks: toks, st: st, stop: stop}
	for {
		switch b.next() {
		case listDone:
			return b.result, b.toks, b.err
		case listAtom:
			nuc, rest, err := p.parseAtom(b.toks, b.st)
			if err != nil {
				return nil, nil, err
			}
			b.nuc, b.toks = nuc, rest
		}
		nuc, rest, err := p.parseScripts(b.nuc, b.toks, b.st)
		if err != nil {
			return nil, nil, err
		}
		b.items = append(b.items, nuc)
		b.toks = rest
	}
}

// The steps of parseListIn that next returns.
const (
	listDone    = iota // the list is done: result, toks and err hold the result
	listAtom           // parseAtom reads the next nucleus
	listNucleus        // nuc holds the next nucleus
)

// next reads the tokens of the list up to the next nucleus. It is not
// inlined, so that its frame is not on the stack of a nested group.
//
//go:noinline
func (b *listBuilder) next() int {
	p := b.p
	for {
		toks := b.toks
		// The terminator of stopOver comes before the switch below, because
		// \over is also the infix fraction there.
		if b.stop == stopOver && atStop(toks, b.stop) {
			b.result = b.done()
			return listDone
		}
		if b.st.text && len(toks) > 0 && toks[0].space {
			b.text(" ")
		}
		if len(toks) > 0 && isMathDelim(toks[0]) {
			// The source's textAlpha carries the alphabet across a pair of
			// math delimiters: \text is an alphabet here, so a $ in it
			// returns to math up to the next $. \text{a$b$c} sets a and c as
			// text and b in math italic.
			if b.st.set {
				b.saved, b.hasSaved, b.st = b.st, true, alphabet{}
			} else if b.hasSaved {
				b.st, b.hasSaved = b.saved, false
			}
			b.toks = toks[1:]
			continue
		}
		if len(toks) > 0 && toks[0].kind == tCtrl {
			if f, ok := sizeFactor(toks[0].text); ok {
				// okage: a size command sets the size of the rest of the
				// group. The source ignores it.
				if err := b.startSize(toks, f); err != nil {
					b.result, b.toks, b.err = nil, nil, err
					return listDone
				}
				continue
			}
			switch kind, style := listCommandOf(toks[0].text); kind {
			case listStyle:
				b.setStyle(style)
				b.toks = toks[1:]
				continue
			case listSkip:
				b.toks = toks[1:]
				continue
			case listFont:
				// A font switch changes the alphabet for the rest of the group.
				// \sl has no math alphabet, so it is italic.
				b.flush()
				b.st = fontSwitch[toks[0].text]
				b.toks = toks[1:]
				continue
			case listTag:
				// okage: \tag labels the formula. The source has no \tag.
				if err := b.startTag(toks); err != nil {
					b.result, b.toks, b.err = nil, nil, err
					return listDone
				}
				continue
			case listColor:
				// okage: \color sets the color of the rest of the group.
				// The source has no \color.
				if err := b.startColor(toks); err != nil {
					b.result, b.toks, b.err = nil, nil, err
					return listDone
				}
				continue
			case listOver:
				// The infix fractions of plain TeX: the list so far is the
				// numerator, and the rest of the group is the denominator.
				// \over has a rule, \atop none, and \choose is \atop in
				// parentheses. Layout half: e.fraction and e.binom.
				//
				// okage: a style command before \over sets the numerator part
				// after it, as in TeX. The source also sets the denominator
				// and the fraction in that style.
				if err := b.startOver(toks); err != nil {
					b.result, b.toks, b.err = nil, nil, err
					return listDone
				}
				continue
			}
		}
		if atStop(toks, b.stop) {
			b.result = b.done()
			return listDone
		}
		if s, ok := textOf(toks[0]); ok && b.st.text {
			// A character of \text. It joins the run, unless a script follows
			// it, as in the source, where the script attaches to it.
			if len(toks) > 1 && (toks[1].kind == tSup || toks[1].kind == tSub) {
				b.flush()
				b.nuc, b.toks = p.add(b.st.run(s)), toks[1:]
				return listNucleus
			}
			b.text(s)
			b.toks = toks[1:]
			continue
		}
		b.flush()
		if k := toks[0].kind; k == tSup || k == tSub || k == tPrime {
			// A script or a prime with no nucleus, such as ^1, a footnote
			// mark. TeX attaches it to an empty nucleus.
			b.nuc = p.list(nil)
			return listNucleus
		}
		return listAtom
	}
}

// listCommand is the kind of a command that parseListIn reads itself.
type listCommand uint8

const (
	listNone  listCommand = iota // a command that parseAtom reads
	listStyle                    // a style command
	listSkip                     // a command that makes no node
	listFont                     // a font switch, such as \rm
	listOver                     // \over, \atop or \choose
	listColor                    // \color
	listTag                      // \tag
)

// listCommandOf returns the kind of the command name, and the style of a
// style command. It is not inlined, so that its frame is not on the stack of
// a nested group.
//
//go:noinline
func listCommandOf(name string) (listCommand, Style) {
	switch name {
	case "displaystyle":
		return listStyle, Display
	case "textstyle":
		return listStyle, Text
	case "scriptstyle":
		return listStyle, Script
	case "scriptscriptstyle":
		// okage: the source has no \scriptscriptstyle.
		return listStyle, ScriptScript
	case "limits", "nolimits", "displaylimits",
		"normalfont", "rmfamily", "sffamily", "ttfamily", "bfseries", "mdseries",
		"itshape", "slshape", "scshape", "upshape", "em", "normalcolor",
		"par", "relax", "protect", "leavevmode", "noindent", "ignorespaces",
		"allowbreak", "nobreak", "break", "notag", "nonumber":
		// Text font and mode switches, paragraph commands and penalties
		// that reach math through macros. They carry no glyph and take no
		// argument, so they make no node. An empty Ord atom would change the
		// space between its neighbors. okage: the size commands are
		// sizeFactor, and \notag and \nonumber belong to \tag.
		//
		// latex.ltx:598-600 makes \break, \nobreak and \allowbreak
		// penalties. This parser does not break a formula, so a penalty makes
		// nothing. \limits after an operator is read by parseScripts.
		return listSkip, 0
	case "rm", "bf", "it", "sf", "tt", "cal", "sl":
		return listFont, 0
	case "over", "atop", "choose":
		return listOver, 0
	case "color":
		return listColor, 0
	case "tag":
		return listTag, 0
	}
	return listNone, 0
}

// parseScripts reads the \limits, the scripts and the primes after the
// nucleus nuc. It is not inlined, so that its frame is not on the stack of a
// nested group.
//
//go:noinline
func (p *parser) parseScripts(nuc Node, toks []token, st alphabet) (Node, []token, error) {
	if err := p.enter(toks); err != nil {
		return nil, nil, err
	}
	n, rest, err := p.parseScriptsIn(nuc, toks, st)
	p.leave()
	return n, rest, err
}

// parseScriptsIn is the body of parseScripts, after the count of the level.
func (p *parser) parseScriptsIn(nuc Node, toks []token, st alphabet) (Node, []token, error) {
	// okage: \limits, \nolimits and \displaylimits after an operator set
	// where its scripts go. The source drops them, and the scripts that
	// follow then attach to an empty atom after the operator.
	for len(toks) > 0 {
		l, ok := limitsCommand(toks[0])
		if !ok {
			break
		}
		if a, ok := nuc.(Atom); ok && a.Class == Op {
			a.Limits = l
			nuc = a
		}
		toks = toks[1:]
	}
	var sup, sub Node
	primes := 0
	for len(toks) > 0 && (toks[0].kind == tSup || toks[0].kind == tSub || (toks[0].kind == tPrime && !st.text)) {
		switch toks[0].kind {
		case tPrime:
			primes++
			toks = toks[1:]
		case tSup:
			g, r, err := p.parseGroupArg(toks[1:], st)
			if err != nil {
				return nil, nil, err
			}
			sup, toks = g, r
		case tSub:
			g, r, err := p.parseGroupArg(toks[1:], st)
			if err != nil {
				return nil, nil, err
			}
			sub, toks = g, r
		}
	}
	if sup != nil || sub != nil || primes > 0 {
		// Layout half: e.primeBox, e.attachScripts and e.attachLimits.
		nuc = p.add(Scripts{Base: nuc, Sup: sup, Sub: sub, Primes: primes})
	}
	return nuc, toks, nil
}

// overPart is the numerator of one \over, \atop or \choose of a list, and
// its command.
type overPart struct {
	num List
	cmd string
}

// startOver ends the numerator of the \over, \atop or \choose at the start
// of toks, and starts its denominator, the rest of the list. done makes the
// fractions.
//
// okage: the source parses the denominator with a recursive call. Here the
// list goes on in the same loop, so that a chain such as a \over b \over c
// takes no stack for each \over. Each denominator is still a level of the
// depth, as a list.
//
//go:noinline
func (b *listBuilder) startOver(toks []token) error {
	p := b.p
	b.overs = append(b.overs, overPart{num: b.finish(), cmd: toks[0].text})
	if err := p.enter(toks[1:]); err != nil {
		return err
	}
	b.items = nil
	b.saved, b.hasSaved = alphabet{}, false
	b.toks = toks[1:]
	return nil
}

// limitsCommand returns the place of the scripts that \limits, \nolimits or
// \displaylimits sets, and false for another token.
func limitsCommand(t token) (Limits, bool) {
	if t.kind != tCtrl {
		return 0, false
	}
	switch t.text {
	case "limits":
		return LimitsAbove, true
	case "nolimits":
		return LimitsSide, true
	case "displaylimits":
		return LimitsAuto, true
	}
	return 0, false
}

// parseGroupArg parses one atom or {group} as an argument, for a script,
// \frac, an accent and the others. An argument is a List.
func (p *parser) parseGroupArg(toks []token, st alphabet) (List, []token, error) {
	if err := p.enter(toks); err != nil {
		return nil, nil, err
	}
	var l List
	var rest []token
	var err error
	if len(toks) > 0 && toks[0].kind == tLBrace {
		// The list of the group and its closing brace.
		l, rest, err = p.parseList(toks[1:], st, stopBrace)
		switch {
		case err != nil:
		case len(rest) == 0 || rest[0].kind != tRBrace:
			err = p.fail(rest, "missing }")
		default:
			rest = rest[1:]
		}
	} else {
		var n Node
		n, rest, err = p.parseAtom(toks, st)
		if err == nil {
			l = p.list([]Node{n})
		}
	}
	p.leave()
	if err != nil {
		return nil, nil, err
	}
	return l, rest, nil
}

// parseAtom parses one nucleus. A {group} is an Ord atom.
//
// okage: each kind of nucleus has a function of its own that is not inlined,
// so that the frame of parseAtom, which is on the stack at each level of
// nesting, stays small.
func (p *parser) parseAtom(toks []token, st alphabet) (Node, []token, error) {
	if len(toks) == 0 {
		return nil, nil, p.fail(toks, "expected an atom")
	}
	if err := p.enter(toks); err != nil {
		return nil, nil, err
	}
	// One call through a function value keeps one place for the results on
	// the stack.
	f := (*parser).parseChar
	switch toks[0].kind {
	case tLBrace:
		f = (*parser).parseGroup
	case tCtrl:
		// A command is a level of its own, for its frame.
		f = (*parser).parseCommand
	}
	n, rest, err := f(p, toks, st)
	p.leave()
	return n, rest, err
}

// parseGroup parses a {group} at the start of toks as an Ord atom.
func (p *parser) parseGroup(toks []token, st alphabet) (Node, []token, error) {
	body, rest, err := p.parseList(toks[1:], st, stopBrace)
	if err != nil {
		return nil, nil, err
	}
	if len(rest) == 0 || rest[0].kind != tRBrace {
		return nil, nil, p.fail(rest, "missing }")
	}
	return p.group(body), rest[1:], nil
}

// parseCommand parses the command at the start of toks. A command is a level
// of its own, for its frame.
//
// The commands are a table, so that the frame of each command is its own,
// and not on the stack of every command.
func (p *parser) parseCommand(toks []token, st alphabet) (Node, []token, error) {
	if err := p.enter(toks); err != nil {
		return nil, nil, err
	}
	n, rest, err := commandFor(toks[0].text)(p, &toks[0], toks[1:], st)
	p.leave()
	return n, rest, err
}

// commandFor returns the parser of the command name: one of the table
// commands, parseAccent for an accent, or parseTableCommand.
//
//go:noinline
func commandFor(name string) command {
	if f, ok := commands[name]; ok {
		return f
	}
	if _, ok := accents[name]; ok {
		return (*parser).parseAccent
	}
	return (*parser).parseTableCommand
}

// parseChar parses a token that is not a group or a command.
func (p *parser) parseChar(toks []token, st alphabet) (Node, []token, error) {
	t := toks[0]
	switch t.kind {
	case tRBrace, tSup, tSub, tAmp, tPrime:
		return nil, nil, p.fail(toks, fmt.Sprintf("unexpected %q", tokenText(t)))
	}
	if st.text {
		return p.add(st.run(t.text)), toks[1:], nil
	}
	// Layout half: mapAlpha and e.mustGlyph.
	return p.add(Atom{Class: charClass(t.r), Text: string(mathChar(t.r)), Variant: st.variantOf()}), toks[1:], nil
}

// group wraps an argument in an Ord atom, the class that the source gives to
// \mathrm{...}, \text{...} and the other wrappers.
func (p *parser) group(body List) Node {
	return p.add(Atom{Class: Ord, Body: body})
}

// command parses a command, after its control sequence cmd.
type command func(p *parser, cmd *token, toks []token, st alphabet) (Node, []token, error)

// commands maps the name of a command to its parser. It is parseControl of
// the source, as a table and not a switch, so that the frame of each command
// is its own, and not on the stack of every command. A goroutine of TinyGo
// has a stack of 64 KiB. init fills it, because the parsers refer to it.
var commands map[string]command

func init() {
	commands = map[string]command{
		"frac":               (*parser).parseFrac,
		"tfrac":              (*parser).parseFrac,
		"dfrac":              (*parser).parseFrac,
		"sqrt":               (*parser).parseSqrt,
		"xrightarrow":        (*parser).parseXArrow,
		"xleftarrow":         (*parser).parseXArrow,
		"xleftrightarrow":    (*parser).parseXArrow,
		"xRightarrow":        (*parser).parseXArrow,
		"xLeftarrow":         (*parser).parseXArrow,
		"xLeftrightarrow":    (*parser).parseXArrow,
		"xhookrightarrow":    (*parser).parseXArrow,
		"xhookleftarrow":     (*parser).parseXArrow,
		"xmapsto":            (*parser).parseXArrow,
		"xrightharpoonup":    (*parser).parseXArrow,
		"xrightharpoondown":  (*parser).parseXArrow,
		"xleftharpoonup":     (*parser).parseXArrow,
		"xleftharpoondown":   (*parser).parseXArrow,
		"left":               (*parser).parseLeft,
		"middle":             (*parser).parseMiddle,
		"overline":           (*parser).parseBar,
		"underline":          (*parser).parseBar,
		"overrightarrow":     (*parser).parseOverArrow,
		"overleftarrow":      (*parser).parseOverArrow,
		"overleftrightarrow": (*parser).parseOverArrow,
		"vv":                 (*parser).parseOverArrow,
		"text":               (*parser).parseTextFont,
		"textrm":             (*parser).parseTextFont,
		"textnormal":         (*parser).parseTextFont,
		"mbox":               (*parser).parseTextFont,
		"hbox":               (*parser).parseTextFont,
		"textbf":             (*parser).parseTextFont,
		"textit":             (*parser).parseTextFont,
		"textsf":             (*parser).parseTextFont,
		"texttt":             (*parser).parseTextFont,
		"textup":             (*parser).parseTextFont,
		"textmd":             (*parser).parseTextFont,
		"emph":               (*parser).parseTextFont,
		"textcolor":          (*parser).parseTextcolor,
		"colorbox":           (*parser).parseColorbox,
		"fcolorbox":          (*parser).parseColorbox,
		"fbox":               (*parser).parseColorbox,
		"mathrm":             (*parser).parseAlphabet,
		"mathbf":             (*parser).parseAlphabet,
		"mathbb":             (*parser).parseAlphabet,
		"mathds":             (*parser).parseAlphabet,
		"mathbbm":            (*parser).parseAlphabet,
		"mathbbmss":          (*parser).parseAlphabet,
		"mathbbb":            (*parser).parseAlphabet,
		"amsmathbb":          (*parser).parseAlphabet,
		"mathcal":            (*parser).parseAlphabet,
		"mathscr":            (*parser).parseAlphabet,
		"EuScript":           (*parser).parseAlphabet,
		"mathfrak":           (*parser).parseAlphabet,
		"mathsf":             (*parser).parseAlphabet,
		"mathtt":             (*parser).parseAlphabet,
		"mathit":             (*parser).parseAlphabet,
		"boldsymbol":         (*parser).parseAlphabet,
		"mathbfit":           (*parser).parseAlphabet,
		"mathbold":           (*parser).parseAlphabet,
		"mathsfit":           (*parser).parseAlphabet,
		"mathsans":           (*parser).parseAlphabet,
		"mathsfbfit":         (*parser).parseAlphabet,
		"mathboldsans":       (*parser).parseAlphabet,
		"ensuremath":         (*parser).parseEnsuremath,
		"mathbin":            (*parser).parseClassWrapper,
		"mathrel":            (*parser).parseClassWrapper,
		"mathord":            (*parser).parseClassWrapper,
		"mathop":             (*parser).parseClassWrapper,
		"mathopen":           (*parser).parseClassWrapper,
		"mathclose":          (*parser).parseClassWrapper,
		"mathpunct":          (*parser).parseClassWrapper,
		"mathinner":          (*parser).parseClassWrapper,
		"boxed":              (*parser).parseBoxed,
		"underbrace":         (*parser).parseBrace,
		"overbrace":          (*parser).parseBrace,
		"mathchoice":         (*parser).parseMathchoice,
		"ooalign":            (*parser).parseOoalign,
		"hidewidth":          (*parser).parseHidewidth,
		"mkern":              (*parser).parseMuKern,
		"mskip":              (*parser).parseMuKern,
		"raisebox":           (*parser).parseRaisebox,
		"rule":               (*parser).parseRule,
		"phantom":            (*parser).parsePhantom,
		"hphantom":           (*parser).parsePhantom,
		"vphantom":           (*parser).parsePhantom,
		"mathstrut":          (*parser).parseMathstrut,
		"smash":              (*parser).parseSmash,
		"smashoperator":      (*parser).parseSmash,
		"ket":                (*parser).parseBraket,
		"bra":                (*parser).parseBraket,
		"braket":             (*parser).parseBraket,
		"norm":               (*parser).parseBraket,
		"abs":                (*parser).parseBraket,
		"Set":                (*parser).parseBraket,
		"set":                (*parser).parseBraket,
		"abs*":               (*parser).parseBraket,
		"norm*":              (*parser).parseBraket,
		"ketbra":             (*parser).parseKetbra,
		"begin":              (*parser).parseEnv,
		"operatorname":       (*parser).parseOperatorname,
		"bmod":               (*parser).parseBmod,
		"pmod":               (*parser).parseMod,
		"mod":                (*parser).parseMod,
		"pod":                (*parser).parseMod,
		"genfrac":            (*parser).parseGenfrac,
		"binom":              (*parser).parseBinom,
		"dbinom":             (*parser).parseBinom,
		"tbinom":             (*parser).parseBinom,
		"choose":             (*parser).parseBinom,
		"overset":            (*parser).parseOverset,
		"underset":           (*parser).parseOverset,
		"stackrel":           (*parser).parseOverset,
		"buildrel":           (*parser).parseBuildrel,
		"lefteqn":            (*parser).parseLefteqn,
		"scalebox":           (*parser).parseScalebox,
		"reflectbox":         (*parser).parseScalebox,
		"substack":           (*parser).parseSubstack,
		"not":                (*parser).parseNot,
		"char":               (*parser).parseCharCommand,
		"cancel":             (*parser).parseCancel,
		"bcancel":            (*parser).parseCancel,
		"xcancel":            (*parser).parseCancel,
		"phase":              (*parser).parsePhase,
		"cfrac":              (*parser).parseCfrac,
		"kern":               (*parser).parseKern,
		"hspace":             (*parser).parseKern,
		"llap":               (*parser).parseLap,
		"rlap":               (*parser).parseLap,
		"clap":               (*parser).parseLap,
		"mathllap":           (*parser).parseLap,
		"mathrlap":           (*parser).parseLap,
		"mathclap":           (*parser).parseLap,
	}
}

// parseAccent parses an accent of the table accents, such as \hat. Layout
// half: e.accent.
func (p *parser) parseAccent(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Accent{Base: b, Accent: accents[cmd.text], Stretch: wideAccents[cmd.text]}), r, nil
}

// parseTableCommand parses a command of the tables that takes no argument
// to parse: the \big family, a named operator, a space or a symbol. The
// accents are in parseAccent.
func (p *parser) parseTableCommand(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	name := cmd.text
	// \big \Big \bigg \Bigg, and their l, r and m forms, give the next
	// delimiter a fixed height. Layout half: e.stretchVertical and
	// e.axisCentre.
	if f, ok := bigDelimFactor(name); ok {
		d, rest, err := p.readDelim(toks)
		if err != nil {
			return nil, nil, err
		}
		return p.add(Delimiter{Rune: d, Class: bigDelimClass(name), Size: f}), rest, nil
	}
	// The named operators, such as \log, \sin and \lim: an upright name, of
	// class Op. The members that take limits set their scripts above and
	// below in display style. Layout half: e.opName.
	if op, ok := namedOps[name]; ok {
		limits := LimitsSide
		if op.limits {
			limits = LimitsAuto
		}
		return p.opAtom(op.text, Op, limits), toks, nil
	}
	// The spaces, in mu.
	if mu, ok := spaces[name]; ok {
		return p.add(Space{Width: Length{Value: mu, Unit: Mu}}), toks, nil
	}
	// The symbols. An operator symbol grows in display style. Layout half:
	// e.mustGlyph and e.displayOperator.
	if s, ok := symbols[name]; ok {
		a := Atom{Class: s.cls, Text: string(s.r), Variant: Upright}
		if s.cls == Op {
			a.Large = true
			a.Limits = LimitsSide
			if opLimits[name] {
				a.Limits = LimitsAuto
			}
		}
		return p.add(a), toks, nil
	}
	return nil, nil, p.unknown(cmd.pos, `unknown command \`+name, `\`+name)
}

// parseFrac parses \frac, \tfrac and \dfrac.
//
// Layout half: e.fraction, with the style of fracInner for the parts.
func (p *parser) parseFrac(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	num, r1, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	den, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	return p.styled(name, p.add(Fraction{Num: num, Den: den})), r2, nil
}

// parseSqrt parses \sqrt.
//
// Layout half: e.radical. The degree is set in scriptscript style:
// "\setbox\rootbox\hbox{$\m@th\scriptscriptstyle{#1}$}"
// (latex.ltx:11187).
func (p *parser) parseSqrt(_ *token, toks []token, st alphabet) (Node, []token, error) {
	var degree Node
	rest := toks
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		d, r, err := p.parseUntilBracket(rest[1:], st)
		if err != nil {
			return nil, nil, err
		}
		degree, rest = d, r
	}
	body, r, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Radical{Body: body, Degree: degree}), r, nil
}

// parseXArrow parses an arrow that grows, such as \xrightarrow, \xmapsto or
// \xleftharpoonup.
//
// An arrow \xxx[under]{over} that grows to its labels, which are set in
// script style. Layout half: xArrowSpec and e.xArrow.
func (p *parser) parseXArrow(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	var under Node
	rest := toks
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		u, r, err := p.parseUntilBracket(rest[1:], st)
		if err != nil {
			return nil, nil, err
		}
		under, rest = u, r
	}
	over, r, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(XArrow{Arrow: xArrows[name], Over: over, Under: under}), r, nil
}

// parseLeft parses \left.
//
// Layout half: e.delimited.
func (p *parser) parseLeft(_ *token, toks []token, st alphabet) (Node, []token, error) {
	open, rest, err := p.readDelim(toks)
	if err != nil {
		return nil, nil, err
	}
	inner, r1, err := p.parseList(rest, st, stopRight)
	if err != nil {
		return nil, nil, err
	}
	if len(r1) == 0 {
		return nil, nil, p.fail(r1, `\left without \right`)
	}
	close, r2, err := p.readDelim(r1[1:]) // skip \right
	if err != nil {
		return nil, nil, err
	}
	return p.add(Delimited{Open: open, Close: close, Body: inner}), r2, nil
}

// parseMiddle parses \middle.
//
// \middle sets a delimiter that grows as the delimiters of its \left and
// \right do, of the class Rel. Outside them, it has a fixed size, 1.8 em, as
// \Bigm. Layout half: e.stretchVertical and e.axisCentre, and
// layoutDelimited.
//
// okage: the source sets \middle at 1.8 em always, and does not measure the
// fence.
func (p *parser) parseMiddle(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	d, rest, err := p.readDelim(toks)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Delimiter{Rune: d, Class: Rel, Size: 1.8, Middle: true}), rest, nil
}

// parseBar parses \overline and \underline.
//
// Layout half: e.overline and e.underline.
func (p *parser) parseBar(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Bar{Base: b, Under: name == "underline"}), r, nil
}

// parseOverArrow parses \overrightarrow, \overleftarrow, \overleftrightarrow
// and \vv.
//
// An arrow over its base, for a vector. \vv is \overrightarrow of
// esvect. Layout half: e.overArrow.
func (p *parser) parseOverArrow(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	left := name == "overleftarrow" || name == "overleftrightarrow"
	right := name != "overleftarrow"
	return p.add(OverArrow{Base: b, Left: left, Right: right}), r, nil
}

// parseAlphabet parses \text and its kin, such as \mbox, and the math
// alphabets, such as \mathbf and \mathbb.
func (p *parser) parseAlphabet(cmd *token, toks []token, _ alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, alphabetFor(name))
	if err != nil {
		return nil, nil, err
	}
	return p.group(b), r, nil
}

// parseEnsuremath parses \ensuremath.
//
// This parser is always in math, so \ensuremath{x} is x.
func (p *parser) parseEnsuremath(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.group(b), r, nil
}

// parseClassWrapper parses \mathbin, \mathrel, \mathord, \mathop, \mathopen,
// \mathclose, \mathpunct and \mathinner.
//
// The class wrappers set the argument with the class they name. An
// operator of \mathop takes limits in display style.
func (p *parser) parseClassWrapper(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Atom{Class: mathClassFor(name), Body: b}), r, nil
}

// parseBoxed parses \boxed.
//
// Layout half: e.boxed.
func (p *parser) parseBoxed(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Boxed{Body: b}), r, nil
}

// parseBrace parses \underbrace and \overbrace.
//
// Layout half: e.brace, a rule in place of the brace.
func (p *parser) parseBrace(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Brace{Base: b, Under: name == "underbrace"}), r, nil
}

// parseHidewidth parses \hidewidth.
//
// \hskip\hideskip is -1000pt plus 1fill (latex.ltx:492,615). Outside
// an alignment it has no meaning here. The source gives an empty Ord
// atom, not a kern of -1000pt. In \ooalign, stripHidewidth reads it.
func (p *parser) parseHidewidth(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	return p.add(Atom{Class: Ord}), toks, nil
}

// parsePhantom parses \phantom, \hphantom and \vphantom.
//
// Layout half: e.phantom.
func (p *parser) parsePhantom(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Phantom{Body: b, Horizontal: name != "vphantom", Vertical: name != "hphantom"}), r, nil
}

// parseMathstrut parses \mathstrut.
//
// latex.ltx:11200 is \DeclareRobustCommand\mathstrut{\vphantom(}: the
// height and the depth of a parenthesis, with no width.
func (p *parser) parseMathstrut(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	paren := p.add(Atom{Class: Open, Text: "(", Variant: Italic})
	return p.add(Phantom{Body: p.list([]Node{paren}), Vertical: true}), toks, nil
}

// parseSmash parses \smash and \smashoperator.
//
// \smash[t|b]{x} sets x with its height, its depth or both removed: [t]
// removes the height, [b] the depth, and no option both, as amsmath does.
// The layout half is raiseBox: a Raise of zero with a height, a depth or both
// of zero. \smashoperator of mathtools, which removes the width of the limits
// of an operator, keeps the body as it is, as the source does.
//
// okage: the source keeps the extent of \smash, and drops the option.
func (p *parser) parseSmash(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	rest := toks
	top, bottom := true, true
	if len(rest) > 0 && rest[0].kind == tChar && rest[0].r == '[' {
		opt, r, err := p.parseUntilBracket(rest[1:], st)
		if err != nil {
			return nil, nil, err
		}
		if len(opt) == 1 {
			if a, ok := opt[0].(Atom); ok {
				top, bottom = a.Text == "t", a.Text == "b"
			}
		}
		rest = r
	}
	b, r, err := p.parseGroupArg(rest, st)
	if err != nil {
		return nil, nil, err
	}
	if cmd.text == "smashoperator" || (!top && !bottom) {
		return p.group(b), r, nil
	}
	return p.add(Raise{Body: b, HasHeight: top, HasDepth: bottom}), r, nil
}

// sizeFactor returns the factor of the size command name, from \tiny to
// \Huge, of the size of LaTeX at 10pt, as KaTeX takes them, and false for
// any other command.
func sizeFactor(name string) (float64, bool) {
	switch name {
	case "tiny":
		return 0.5, true
	case "scriptsize":
		return 0.7, true
	case "footnotesize":
		return 0.8, true
	case "small":
		return 0.9, true
	case "normalsize":
		return 1, true
	case "large":
		return 1.2, true
	case "Large":
		return 1.44, true
	case "LARGE":
		return 1.728, true
	case "huge":
		return 2.074, true
	case "Huge":
		return 2.488, true
	}
	return 0, false
}

// startSize reads the size command at the start of toks, and the rest of the
// list as its body, as startColor does for \color.
//
//go:noinline
func (b *listBuilder) startSize(toks []token, f float64) error {
	p := b.p
	b.flush()
	body, rest, err := p.parseList(toks[1:], b.st, b.stop)
	if err != nil {
		return err
	}
	b.items = append(b.items, p.add(SizeChange{Factor: f, Body: body}))
	b.toks = rest
	return nil
}

// parseBraket parses \ket, \bra, \braket, \norm, \abs, \Set, \set, \abs* and
// \norm*.
//
// The notation of braket and physics: the argument between two
// delimiters of the natural size. Layout half: e.braWrap.
func (p *parser) parseBraket(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	a, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	open, close := braketDelims(name)
	return p.add(Atom{Class: Inner, Body: p.braWrap(open, close, a)}), r, nil
}

// parseKetbra parses \ketbra.
func (p *parser) parseKetbra(_ *token, toks []token, st alphabet) (Node, []token, error) {
	x, r1, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	y, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	body := p.list([]Node{p.braWrap('|', '⟩', x), p.braWrap('⟨', '|', y)})
	return p.add(Atom{Class: Inner, Body: body}), r2, nil
}

// parseOperatorname parses \operatorname.
//
// \operatorname{name} sets name upright as an operator. The starred
// form takes limits in display style.
func (p *parser) parseOperatorname(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	limits := LimitsSide
	if len(toks) > 0 && toks[0].kind == tChar && toks[0].r == '*' {
		limits, toks = LimitsAuto, toks[1:]
	}
	opname, rest, err := p.readOpName(toks)
	if err != nil {
		return nil, nil, err
	}
	return p.opAtom(opname, Op, limits), rest, nil
}

// parseBmod parses \bmod.
//
// The binary operator "mod", set upright.
func (p *parser) parseBmod(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	return p.opAtom("mod", Bin, LimitsAuto), toks, nil
}

// parseMod parses \pmod, \mod and \pod.
//
// Layout half: e.modBox.
func (p *parser) parseMod(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	n, rest, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	switch name {
	case "pmod":
		return p.modNode('(', ')', "mod", n), rest, nil
	case "mod":
		return p.modNode(0, 0, "mod", n), rest, nil
	default:
		return p.modNode('(', ')', "", n), rest, nil
	}
}

// parseBinom parses \binom, \dbinom, \tbinom and \choose.
//
// Layout half: e.binom inside e.delimited.
func (p *parser) parseBinom(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	num, r1, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	den, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	f := p.add(Fraction{Num: num, Den: den, NoRule: true, Open: '(', Close: ')'})
	return p.styled(name, f), r2, nil
}

// parseOverset parses \overset, \underset and \stackrel.
//
// \overset{top}{base} and \stackrel put the first argument, in script
// style, over the base, and \underset under it. Layout half:
// e.overUnder.
func (p *parser) parseOverset(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	name := cmd.text
	extra, r1, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	base, r2, err := p.parseGroupArg(r1, st)
	if err != nil {
		return nil, nil, err
	}
	n := OverUnder{Base: base, Class: Ord}
	if name == "underset" {
		n.Under = extra
	} else {
		n.Over = extra
	}
	if name == "stackrel" {
		n.Class = Rel
	}
	return p.add(n), r2, nil
}

// parseBuildrel parses \buildrel.
//
// \buildrel <top> \over <base> is the plain TeX \stackrel, with the
// top argument delimited by \over (latex.ltx:11228,
// \def\buildrel#1\over#2{\mathrel{\mathop{\kern\z@#2}\limits^{#1}}}).
// The top argument is a list, so that a font switch such as \rm in
// \buildrel \rm def \over = works. It is a relation.
func (p *parser) parseBuildrel(_ *token, toks []token, st alphabet) (Node, []token, error) {
	extra, r1, err := p.parseList(toks, st, stopOver)
	if err != nil {
		return nil, nil, err
	}
	if len(r1) == 0 || r1[0].kind != tCtrl || r1[0].text != "over" {
		return nil, nil, p.fail(r1, `\buildrel without \over`)
	}
	base, r2, err := p.parseGroupArg(r1[1:], st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(OverUnder{Base: base, Over: extra, Class: Rel}), r2, nil
}

// parseLefteqn parses \lefteqn.
//
// \lefteqn{X} sets X in display style with no width, so that it
// overhangs to the right (latex.ltx:11392,
// \def\lefteqn#1{\rlap{$\displaystyle #1$}}).
func (p *parser) parseLefteqn(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, r, err := p.parseGroupArg(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Lap{Body: p.add(StyleChange{Style: Display, Body: b})}), r, nil
}

// parseScalebox parses \scalebox and \reflectbox.
//
// graphics.sty:519-538. \reflectbox is \Gscale@box-1[1], a mirror,
// so both make a Scale. Layout half: e.scaleBox in scalebox.go.
func (p *parser) parseScalebox(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	n, rest, err := p.scaleboxHead(cmd, toks)
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

// scaleboxHead reads the factors of \scalebox, and returns the Scale without
// its body and the tokens of the body. \reflectbox has no factor to read.
//
// okage: it is not inlined, so that its frame is not on the stack while
// parseScalebox parses the body.
//
//go:noinline
func (p *parser) scaleboxHead(cmd *token, toks []token) (*Scale, []token, error) {
	n := &Scale{X: -1, Y: 1}
	rest := toks
	if cmd.text == "scalebox" {
		f, r, err := p.readScaleFactor(toks)
		if err != nil {
			return nil, nil, err
		}
		n.X, n.Y, rest = f, f, r
		// Without the optional [v], the vertical factor is the horizontal
		// one, as the \@ifnextchar branch of \scalebox gives.
		g, r2, ok, err := p.readOptScaleFactor(rest)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			n.Y, rest = g, r2
		}
	}
	return n, rest, nil
}

// parseSubstack parses \substack.
//
// \substack{a \\ b \\ c}: lines in script style, stacked and centered,
// for the scripts of a large operator. Layout half: e.gridLayout with
// flatLeading.
func (p *parser) parseSubstack(_ *token, toks []token, st alphabet) (Node, []token, error) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, nil, p.fail(toks, `\substack needs {…}`)
	}
	rest := toks[1:]
	var rows [][]Node
	for {
		cell, r, err := p.parseList(rest, st, stopStackRow)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, []Node{cell})
		rest = r
		if len(rest) == 0 {
			return nil, nil, p.fail(rest, `\substack without closing }`)
		}
		if rest[0].kind == tRBrace {
			rest = rest[1:]
			break
		}
		rest = rest[1:] // consume \\
	}
	return p.add(Array{Kind: ArraySubstack, Rows: rows}), rest, nil
}

// parseNot parses \not.
//
// \not X sets a slash over the next atom, such as \not= for ≠, and
// keeps the class of that atom. Layout half: e.negate.
func (p *parser) parseNot(_ *token, toks []token, st alphabet) (Node, []token, error) {
	b, rest, err := p.parseAtom(toks, st)
	if err != nil {
		return nil, nil, err
	}
	return p.add(Negate{Body: b}), rest, nil
}

// parseCharCommand parses \char.
//
// \char<code> sets the character of a code point: `<c> or `\<sym>,
// "<hex>, or a decimal number. It reaches math through macros, such as
// \char`\^ for a literal ^.
func (p *parser) parseCharCommand(_ *token, toks []token, _ alphabet) (Node, []token, error) {
	code, rest, ok := parseCharCode(toks)
	if !ok {
		return nil, nil, p.fail(toks, `\char needs a character code`)
	}
	return p.add(Atom{Class: Ord, Text: string(rune(code)), Variant: Upright}), rest, nil
}

// styled puts n in the style that a form of a command names: display for
// \dfrac and \dbinom, text for \tfrac and \tbinom.
func (p *parser) styled(name string, n Node) Node {
	switch name {
	case "dfrac", "dbinom":
		return p.add(StyleChange{Style: Display, Body: n})
	case "tfrac", "tbinom":
		return p.add(StyleChange{Style: Text, Body: n})
	}
	return n
}

// parseGenfrac parses \genfrac{left}{right}{thickness}{style}{num}{den}
// (amsmath.sty:245-250):
//
//	\edef\@tempb{\@nx\@genfrac\@mathstyle{#4}%
//	  \csname @@\ifx @#3@over\else above\fi
//	  \ifx\@tempa\@empty \else withdelims\fi\endcsname}
//
// An empty thickness is \over, with the rule of the font. Any other thickness
// is \above that thickness, and 0pt is no rule. \binom is \genfrac()\z@{}
// (amsmath.sty:240). The style is the digit of TeX: 0 display, 1 text, 2
// script, 3 scriptscript. An empty style keeps the style in force. A
// thickness that is not zero draws the rule of the font. Layout half:
// e.fraction or e.binom, in e.delimited when a delimiter is given.
//
// okage: style 3 is scriptscript. The source sets it in script style. A
// thickness past the limit of a length is an error.
func (p *parser) parseGenfrac(_ *token, toks []token, st alphabet) (Node, []token, error) {
	g, err := p.genfracHead(toks)
	if err != nil {
		return nil, nil, err
	}
	num, r5, err := p.parseGroupArg(g.rest, st)
	if err != nil {
		return nil, nil, err
	}
	den, r6, err := p.parseGroupArg(r5, st)
	if err != nil {
		return nil, nil, err
	}
	return g.node(num, den), r6, nil
}

// genfrac holds the first four arguments of \genfrac.
type genfrac struct {
	p            *parser
	left, right  rune
	thick, style string
	rest         []token // the tokens after the four arguments
}

// genfracHead reads the first four arguments of \genfrac: the delimiters,
// the thickness and the style.
//
// okage: it is not inlined, so that its frame is not on the stack while
// parseGenfrac parses the numerator and the denominator.
//
//go:noinline
func (p *parser) genfracHead(toks []token) (*genfrac, error) {
	left, r1, err := p.readOptionalDelim(toks)
	if err != nil {
		return nil, err
	}
	right, r2, err := p.readOptionalDelim(r1)
	if err != nil {
		return nil, err
	}
	thick, r3 := readArgText(r2)
	if err := p.checkThickness(r2, thick); err != nil {
		return nil, err
	}
	style, r4 := readArgText(r3)
	return &genfrac{p: p, left: left, right: right, thick: thick, style: style, rest: r4}, nil
}

// node makes the fraction of \genfrac with the numerator num and the
// denominator den, in its style.
//
//go:noinline
func (g *genfrac) node(num, den List) Node {
	p := g.p
	f := p.add(Fraction{Num: num, Den: den, NoRule: isZeroThickness(g.thick), Open: g.left, Close: g.right})
	switch g.style {
	case "0":
		f = p.add(StyleChange{Style: Display, Body: f})
	case "1":
		f = p.add(StyleChange{Style: Text, Body: f})
	case "2":
		f = p.add(StyleChange{Style: Script, Body: f})
	case "3":
		f = p.add(StyleChange{Style: ScriptScript, Body: f})
	}
	return f
}

// checkThickness reports a thickness of \genfrac whose number is larger
// than maxLength, or out of the range of a float64.
func (p *parser) checkThickness(toks []token, s string) error {
	s = strings.TrimSpace(s)
	num := strings.TrimRight(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ")
	v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if (err == nil && !(math.Abs(v) <= maxLength)) || errors.Is(err, strconv.ErrRange) {
		return p.limit(toks, fmt.Sprintf(`the thickness %q of \genfrac is more than %d of its unit`, s, maxLength))
	}
	return nil
}

// envName reads the name of an environment, {name}, after \begin or \end.
// It returns false when no } ends it.
func envName(toks []token) (string, []token, bool) {
	var sb strings.Builder
	i := 1
	for i < len(toks) && toks[i].kind != tRBrace {
		sb.WriteString(tokenText(toks[i]))
		i++
	}
	if i >= len(toks) {
		return "", nil, false
	}
	return sb.String(), toks[i+1:], true
}

// parseEnv parses an environment \begin{env}…\end{env}. Layout half:
// e.finishEnv.
//
// okage: the state of the environment is on the heap, in an envBuilder, so
// that a level of nesting takes little of the stack.
func (p *parser) parseEnv(cmd *token, toks []token, st alphabet) (Node, []token, error) {
	e, err := p.beginEnv(cmd, toks)
	if err != nil {
		return nil, nil, err
	}
	for {
		cell, rest, err := p.parseList(e.toks, st, stopCell)
		if err != nil {
			return nil, nil, err
		}
		if done, err := e.cell(cell, rest); err != nil {
			return nil, nil, err
		} else if done {
			return e.node, e.toks, nil
		}
	}
}

// envBuilder holds the state of an environment that parseEnv reads.
type envBuilder struct {
	p      *parser
	name   string
	info   envInfo
	aligns []Align
	vrules []int
	hrules []int
	rows   [][]Node
	row    []Node
	toks   []token // the tokens still to read
	node   Node    // the Array, when the environment is done
}

// beginEnv reads the name of the environment after \begin, and the column
// specification of array.
//
//go:noinline
func (p *parser) beginEnv(cmd *token, toks []token) (*envBuilder, error) {
	if len(toks) < 3 || toks[0].kind != tLBrace {
		return nil, p.fail(toks, `\begin needs {env}`)
	}
	env, rest, ok := envName(toks)
	if !ok {
		return nil, p.fail(nil, `\begin{ unterminated`)
	}
	info, ok := envTable[env]
	if !ok {
		return nil, p.unknown(cmd.pos, fmt.Sprintf("unknown environment %q", env), `\begin{`+env+`}`)
	}
	e := &envBuilder{p: p, name: env, info: info, toks: rest}
	// array reads a column specification, such as {lcr|…}, before its body. A
	// missing or unterminated specification gives centered columns.
	if info.kind == ArrayArray {
		e.aligns, e.vrules, e.toks = readColSpec(rest)
	}
	e.hlines()
	return e, nil
}

// hlines reads the \hline and \hdashline commands at the start of a row, and
// records a horizontal rule over the row for each. \hdashline is a solid
// rule.
//
// okage: the source has no \hline.
func (e *envBuilder) hlines() {
	for len(e.toks) > 0 && e.toks[0].kind == tCtrl && (e.toks[0].text == "hline" || e.toks[0].text == "hdashline") {
		e.hrules = append(e.hrules, len(e.rows))
		e.toks = e.toks[1:]
	}
}

// cell adds the cell that ends at rest, and reads the &, \\ or \end after
// it. It returns true when \end closes the environment, with the Array in
// node.
//
//go:noinline
func (e *envBuilder) cell(cell List, rest []token) (bool, error) {
	p := e.p
	e.row = append(e.row, cell)
	e.toks = rest
	if len(rest) == 0 {
		return false, p.fail(rest, fmt.Sprintf(`\begin{%s} without \end`, e.name))
	}
	switch {
	case rest[0].kind == tAmp:
		e.toks = rest[1:]
	case rest[0].kind == tCtrl && rest[0].text == `\`:
		e.rows = append(e.rows, e.row)
		e.row = nil
		e.toks = rest[1:]
		e.hlines()
	case rest[0].kind == tCtrl && rest[0].text == "end":
		// okage: a \\ before \end starts no row, as in LaTeX and KaTeX. The
		// source adds an empty row (parse.go:1066).
		if len(cell) > 0 || len(e.row) > 1 || len(e.rows) == 0 {
			e.rows = append(e.rows, e.row)
		}
		r, err := p.consumeEnd(rest[1:], e.name)
		if err != nil {
			return false, err
		}
		e.toks = r
		e.node = p.add(Array{
			Kind:    e.info.kind,
			Rows:    e.rows,
			Align:   e.aligns,
			VRules:  e.vrules,
			HRules:  e.hrules,
			Open:    e.info.open,
			Close:   e.info.close,
			Stretch: e.info.stretch,
		})
		return true, nil
	}
	return false, nil
}

// readColSpec parses the column specification of array, such as {|c|c|} or
// {lcr}. It returns the alignment of each column, the column gaps, 0 to the
// column count, that carry a vertical rule, and the rest of the tokens.
// Without a brace group, every column is centered. The columns p{…}, m{…}
// and b{…} have no width here, and they are flush left. An unknown letter is
// a centered column.
func readColSpec(toks []token) (aligns []Align, vrules []int, rest []token) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return nil, nil, toks
	}
	// Collect the tokens between the outer braces, with the nested braces.
	depth := 1
	i := 1
	var spec []token
collect:
	for ; i < len(toks); i++ {
		t := toks[i]
		switch t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			depth--
			if depth == 0 {
				i++ // consume the closing brace
				break collect
			}
		}
		spec = append(spec, t)
	}
	col := 0
	for j := 0; j < len(spec); j++ {
		t := spec[j]
		if t.kind != tChar {
			continue // a control sequence or a brace
		}
		switch t.r {
		case '|':
			vrules = append(vrules, col)
		case 'l':
			aligns = append(aligns, AlignLeft)
			col++
		case 'c':
			aligns = append(aligns, AlignCenter)
			col++
		case 'r':
			aligns = append(aligns, AlignRight)
			col++
		case 'p', 'm', 'b':
			aligns = append(aligns, AlignLeft)
			col++
			if j+1 < len(spec) && spec[j+1].kind == tLBrace { // skip the {width} group
				d := 0
			width:
				for j++; j < len(spec); j++ {
					switch spec[j].kind {
					case tLBrace:
						d++
					case tRBrace:
						if d--; d == 0 {
							break width
						}
					}
				}
			}
		default:
			if isLetter(t.r) {
				aligns = append(aligns, AlignCenter)
				col++
			}
			// The other characters, such as @ and *, are ignored.
		}
	}
	return aligns, vrules, toks[i:]
}

func (p *parser) consumeEnd(toks []token, env string) ([]token, error) {
	if len(toks) < 2 || toks[0].kind != tLBrace {
		return nil, p.fail(toks, `\end needs {env}`)
	}
	name, rest, ok := envName(toks)
	if !ok {
		return nil, p.fail(nil, `\end{ unterminated`)
	}
	if name != env {
		return nil, p.fail(toks, fmt.Sprintf(`\begin{%s} closed by \end{%s}`, env, name))
	}
	return rest, nil
}

// parseUntilBracket parses atoms up to a literal ']', for the optional
// argument of \sqrt and the others.
func (p *parser) parseUntilBracket(toks []token, st alphabet) (List, []token, error) {
	var items []Node
	for len(toks) > 0 && (toks[0].kind != tChar || toks[0].r != ']') {
		b, rest, err := p.parseAtom(toks, st)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, b)
		toks = rest
	}
	if len(toks) == 0 {
		return nil, nil, p.fail(toks, "missing ] after [")
	}
	return p.list(items), toks[1:], nil
}

// bigDelimFactor reports the height in em of the \big family, \big, \bigl,
// \bigr, \bigm and the others, and false for any other command.
func bigDelimFactor(name string) (float64, bool) {
	base := strings.TrimRight(name, "lrm")
	switch base {
	case "big":
		return 1.2, true
	case "Big":
		return 1.8, true
	case "bigg":
		return 2.4, true
	case "Bigg":
		return 3.0, true
	}
	return 0, false
}

// braketDelims returns the delimiters of a command of braket or physics.
func braketDelims(name string) (rune, rune) {
	switch name {
	case "ket":
		return '|', '⟩'
	case "bra":
		return '⟨', '|'
	case "braket":
		return '⟨', '⟩'
	case "norm", "norm*":
		return '‖', '‖'
	case "Set", "set":
		return '{', '}'
	}
	return '|', '|' // abs / abs*
}

// braWrap puts arg between two delimiters of the natural size.
func (p *parser) braWrap(open, close rune, arg List) List {
	return p.list([]Node{
		p.add(Delimiter{Rune: open, Class: Open}),
		arg,
		p.add(Delimiter{Rune: close, Class: Close}),
	})
}

// mathClassFor maps a class wrapper to the class it sets.
func mathClassFor(name string) Class {
	switch name {
	case "mathbin":
		return Bin
	case "mathrel":
		return Rel
	case "mathop":
		return Op
	case "mathopen":
		return Open
	case "mathclose":
		return Close
	case "mathpunct":
		return Punct
	case "mathinner":
		return Inner
	}
	return Ord // mathord
}

// bigDelimClass maps the suffix of a command of the \big family to its class:
// l to Open, r to Close, m to Rel, and none to Ord.
func bigDelimClass(name string) Class {
	switch name[len(name)-1] {
	case 'l':
		return Open
	case 'r':
		return Close
	case 'm':
		return Rel
	}
	return Ord
}

// readDelim reads a delimiter for \left and \right: a character such as (, [,
// | or ., or a control sequence of the symbol table such as \{ or \langle. It
// returns 0 for '.'.
func (p *parser) readDelim(toks []token) (rune, []token, error) {
	if len(toks) == 0 {
		return 0, nil, p.fail(toks, `\left/\right needs a delimiter`)
	}
	t := toks[0]
	switch t.kind {
	case tChar:
		if t.r == '.' {
			return 0, toks[1:], nil
		}
		return t.r, toks[1:], nil
	case tCtrl:
		if s, ok := symbols[t.text]; ok {
			return s.r, toks[1:], nil
		}
	}
	return 0, nil, p.fail(toks, fmt.Sprintf("bad delimiter %q", tokenText(t)))
}

// readOptionalDelim reads a delimiter of \genfrac: a group, where {} is no
// delimiter, or a single token, as TeX takes it without braces.
// \genfrac[]{0pt}{}{n}{k} gives [ and ] that way.
func (p *parser) readOptionalDelim(toks []token) (rune, []token, error) {
	if len(toks) == 0 {
		return 0, nil, p.fail(toks, `\genfrac needs its six arguments`)
	}
	if toks[0].kind != tLBrace {
		return p.readDelim(toks)
	}
	if len(toks) > 1 && toks[1].kind == tRBrace {
		return 0, toks[2:], nil
	}
	r, rest, err := p.readDelim(toks[1:])
	if err != nil {
		return 0, nil, err
	}
	if len(rest) == 0 || rest[0].kind != tRBrace {
		return 0, nil, p.fail(rest, `\genfrac delimiter is not one token`)
	}
	return r, rest[1:], nil
}

// readArgText reads one argument as text, the characters of a group or a
// single token, for the arguments that are a length or a digit: the
// thickness and the style of \genfrac.
func readArgText(toks []token) (string, []token) {
	if len(toks) == 0 {
		return "", nil
	}
	if toks[0].kind != tLBrace {
		// \z@ is one control word here, which is how amsmath spells zero in
		// \binom (\genfrac()\z@{}, amsmath.sty:240).
		return tokenText(toks[0]), toks[1:]
	}
	var sb strings.Builder
	depth := 1
	for i := 1; i < len(toks); i++ {
		switch t := toks[i]; t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth--; depth == 0 {
				return sb.String(), toks[i+1:]
			}
		default:
			sb.WriteString(tokenText(t))
		}
	}
	return sb.String(), nil
}

// isZeroThickness reports whether a thickness of \genfrac asks for no rule.
// An empty argument is \over, with the rule of the font, and every other
// spelling of zero, such as 0pt, 0, \z@ or 0.0pt, is \above 0pt, which draws
// nothing.
func isZeroThickness(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if s == `\z@` {
		return true
	}
	num := strings.TrimRight(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ ")
	v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	return err == nil && v == 0
}

// parseCharCode reads the number after \char and returns it with the rest of
// the tokens. It reads `<char> or `\<sym>, the code point of that character,
// "<hex>, and a decimal number. ok is false when no number follows.
func parseCharCode(toks []token) (code int, rest []token, ok bool) {
	if len(toks) == 0 {
		return 0, toks, false
	}
	if toks[0].kind == tChar && toks[0].r == '`' {
		toks = toks[1:]
		if len(toks) == 0 {
			return 0, toks, false
		}
		switch toks[0].kind {
		case tCtrl:
			if r, size := utf8.DecodeRuneInString(toks[0].text); size > 0 {
				return int(r), toks[1:], true
			}
			return 0, toks, false
		case tChar:
			return int(toks[0].r), toks[1:], true
		default:
			return 0, toks, false
		}
	}
	base := 10
	if toks[0].kind == tChar && toks[0].r == '"' {
		base, toks = 16, toks[1:]
	}
	var digits []rune
	for len(toks) > 0 && toks[0].kind == tChar && isBaseDigit(toks[0].r, base) {
		digits = append(digits, toks[0].r)
		toks = toks[1:]
	}
	if len(digits) == 0 {
		return 0, toks, false
	}
	v, err := strconv.ParseInt(string(digits), base, 32)
	if err != nil || v < 0 || v > 0x10FFFF {
		return 0, toks, false
	}
	return int(v), toks, true
}

// isBaseDigit reports whether r is a digit in base 10 or 16.
func isBaseDigit(r rune, base int) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case base == 16 && (r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F'):
		return true
	}
	return false
}

// opAtom makes an upright operator name of class cls. A space in s is a thin
// space of 3mu, for "lim sup" and "lim inf".
func (p *parser) opAtom(s string, cls Class, limits Limits) Node {
	if !strings.Contains(s, " ") {
		return p.add(Atom{Class: cls, Text: s, Variant: Upright, Limits: limits})
	}
	var items []Node
	for i, w := range strings.Split(s, " ") {
		if i > 0 {
			items = append(items, p.add(Space{Width: Length{Value: 3, Unit: Mu}}))
		}
		if w != "" {
			items = append(items, p.add(Atom{Class: Ord, Text: w, Variant: Upright}))
		}
	}
	return p.add(Atom{Class: cls, Variant: Upright, Body: p.list(items), Limits: limits})
}

// modNode makes \pmod{n}, "(mod n)", \mod{n}, "mod n", and \pod{n}, "(n)".
// A quad, 18mu, comes first, as in plain TeX. open and close are 0 for \mod,
// and label is "" for \pod.
func (p *parser) modNode(open, close rune, label string, n List) Node {
	items := []Node{p.add(Space{Width: Length{Value: 18, Unit: Mu}})}
	if open != 0 {
		items = append(items, p.add(Atom{Class: Ord, Text: string(open), Variant: Upright}))
	}
	if label != "" {
		items = append(items, p.opAtom(label, Ord, LimitsAuto), p.add(Space{Width: Length{Value: 3, Unit: Mu}}))
	}
	items = append(items, n)
	if close != 0 {
		items = append(items, p.add(Atom{Class: Ord, Text: string(close), Variant: Upright}))
	}
	return p.add(Atom{Class: Ord, Body: p.list(items)})
}

// readOpName reads the {name} argument of \operatorname, \rule and
// \raisebox as text. A character gives its rune, the spaces \, \: \; and the
// control space give a space, and the other control sequences are ignored.
func (p *parser) readOpName(toks []token) (string, []token, error) {
	if len(toks) == 0 || toks[0].kind != tLBrace {
		return "", nil, p.fail(toks, `\operatorname needs {name}`)
	}
	var sb strings.Builder
	depth := 1
	for i := 1; i < len(toks); i++ {
		switch t := toks[i]; t.kind {
		case tLBrace:
			depth++
		case tRBrace:
			if depth--; depth == 0 {
				return sb.String(), toks[i+1:], nil
			}
		case tChar:
			sb.WriteString(t.text)
		case tCtrl:
			switch t.text {
			case ",", ":", ";", " ":
				sb.WriteByte(' ')
			}
		}
	}
	return "", nil, p.fail(nil, `\operatorname unterminated {name}`)
}

// ── character classes & alphabets ───────────────────────────────────────────

// charClass gives a character its class. fontmath.ltx declares each ASCII
// character with \DeclareMathSymbol (lines 149-158, 168-169, 171):
//
//	{*}{\mathbin}   {+}{\mathbin}   {-}{\mathbin}
//	{=}{\mathrel}   {<}{\mathrel}   {>}{\mathrel}   {:}{\mathrel}
//	{,}{\mathpunct} {;}{\mathpunct}
//	{!}{\mathclose} {?}{\mathclose}
//	{.}{\mathord}   {/}{\mathord}
func charClass(r rune) Class {
	switch r {
	case '+', '-', '*':
		return Bin
	case '=', '<', '>', ':':
		return Rel
	case '(', '[':
		return Open
	case ')', ']', '!', '?':
		return Close
	case ',', ';':
		return Punct
	default: // '.', '/', letters, digits
		return Ord
	}
}

// mathChar maps a character to the glyph that math sets for it. fontmath.ltx
// sets - as the minus sign U+2212 and * as the asterisk operator U+2217
// (fontmath.ltx:150 and :153).
func mathChar(r rune) rune {
	switch r {
	case '-':
		return '−' // MINUS SIGN
	case '*':
		return '∗' // ASTERISK OPERATOR
	default:
		return r
	}
}

// mathItalic maps the ASCII letters to the code points of math italic.
func mathItalic(r rune) rune {
	switch {
	case r == 'h':
		return 0x210E
	case r >= 'a' && r <= 'z':
		return 0x1D44E + (r - 'a')
	case r >= 'A' && r <= 'Z':
		return 0x1D434 + (r - 'A')
	default:
		return r
	}
}

// alphabetFor returns the alphabet of a command such as \mathbb or \text.
func alphabetFor(name string) alphabet {
	switch name {
	case "text", "hbox", "mbox", "textrm", "textnormal":
		return alphabet{set: true, text: true, variant: Upright}
	case "mathrm":
		return alphabet{set: true, variant: Upright}
	case "mathscr", "EuScript", "mathcal":
		// The script block, shared with \mathcal. The font has no roundhand
		// face of its own.
		return alphabet{set: true, variant: Calligraphic}
	case "mathbf":
		return alphabet{set: true, variant: Bold}
	case "mathit":
		return alphabet{set: true, variant: Italic}
	case "boldsymbol", "mathbfit", "mathbold":
		// \mathbfit is bold italic in unicode-math and in isomath
		// (isomath.sty:184), and \mathbold is the alias of fixmath that isomath
		// keeps (:189).
		return alphabet{set: true, variant: BoldItalic}
	case "mathsfit", "mathsans":
		// isomath.sty:200.
		return alphabet{set: true, variant: SansSerifItalic}
	case "mathsfbfit", "mathboldsans":
		// The face of ISO 80000-2 for tensors (isomath.sty:218 and :284).
		return alphabet{set: true, variant: SansSerifBoldItalic}
	case "mathsf":
		return alphabet{set: true, variant: SansSerif}
	case "mathtt":
		return alphabet{set: true, variant: Monospace}
	case "mathbb", "mathds", "mathbbm", "mathbbmss", "mathbbb", "amsmathbb":
		// \mathds (dsfont), \mathbbm (bbm), \mathbbmss and \mathbbb (bbold)
		// are double-struck fonts, and the blackboard block stands for each.
		// \amsmathbb is the AMSb font, the font of \mathbb.
		return alphabet{set: true, variant: DoubleStruck}
	case "mathfrak":
		return alphabet{set: true, variant: Fraktur}
	default:
		return alphabet{}
	}
}

// mapVariant maps a character to its code point in the variant v. Only the
// ASCII letters, and the digits of the blocks that have digits, change. The
// layout calls it for the Text of an Atom, in opName. It is mapAlpha of the
// source.
func mapVariant(r rune, v Variant) rune {
	switch v {
	case Italic:
		return mathItalic(r)
	case Bold:
		return blockMap(r, 0x1D400, 0x1D41A, 0x1D7CE, nil)
	case BoldItalic:
		// Bold italic has no digits in Unicode, so they are bold.
		return blockMap(r, 0x1D468, 0x1D482, 0x1D7CE, nil)
	case DoubleStruck:
		return blockMap(r, 0x1D538, 0x1D552, 0x1D7D8, bbHoles)
	case Calligraphic:
		return blockMap(r, 0x1D49C, 0x1D4B6, 0, calHoles)
	case Fraktur:
		return blockMap(r, 0x1D504, 0x1D51E, 0, frakHoles)
	case SansSerif:
		return blockMap(r, 0x1D5A0, 0x1D5BA, 0x1D7E2, nil)
	case SansSerifItalic:
		return blockMap(r, 0x1D608, 0x1D622, 0x1D7E2, nil)
	case SansSerifBoldItalic:
		return blockMap(r, 0x1D63C, 0x1D656, 0x1D7EC, nil)
	case SansSerifBold:
		return blockMap(r, 0x1D5D4, 0x1D5EE, 0x1D7EC, nil)
	case Monospace:
		return blockMap(r, 0x1D670, 0x1D68A, 0x1D7F6, nil)
	default: // Upright
		return r
	}
}

// blockMap maps a letter or a digit into a block of code points, with a table
// of holes for the code points that are elsewhere. It is blockMapper of the
// source, without the closure.
func blockMap(r, ucBase, lcBase, digBase rune, holes map[rune]rune) rune {
	if h, ok := holes[r]; ok {
		return h
	}
	switch {
	case r >= 'A' && r <= 'Z':
		return ucBase + (r - 'A')
	case r >= 'a' && r <= 'z':
		return lcBase + (r - 'a')
	case r >= '0' && r <= '9' && digBase != 0:
		return digBase + (r - '0')
	default:
		return r
	}
}

var bbHoles = map[rune]rune{'C': 0x2102, 'H': 0x210D, 'N': 0x2115, 'P': 0x2119, 'Q': 0x211A, 'R': 0x211D, 'Z': 0x2124}
var calHoles = map[rune]rune{'B': 0x212C, 'E': 0x2130, 'F': 0x2131, 'H': 0x210B, 'I': 0x2110, 'L': 0x2112, 'M': 0x2133, 'R': 0x211B, 'e': 0x212F, 'g': 0x210A, 'o': 0x2134}
var frakHoles = map[rune]rune{'C': 0x212D, 'H': 0x210C, 'I': 0x2111, 'R': 0x211C, 'Z': 0x2128}

// ── tables ──────────────────────────────────────────────────────────────────

type sym struct {
	r   rune
	cls Class
}

// accents maps an accent command to the spacing modifier letter that draws
// it, such as U+02C6 and not the combining U+0302, because a combining
// character has no width. The layout places the glyph itself.
var accents = map[string]rune{
	"hat": 'ˆ', "widehat": 'ˆ', "bar": '¯', "vec": '⃗',
	"tilde": '˜', "widetilde": '˜', "dot": '˙', "ddot": '¨',
	"check": 'ˇ', "widecheck": 'ˇ', "breve": '˘', "acute": '´', "grave": '`',
	"mathring": '˚',
	// okage: \dddot and \ddddot of amsmath, which have no spacing form.
	"dddot": '\u20DB', "ddddot": '\u20DC',
}

// wideAccents are the accents that stretch over their base. The source sets
// them with the glyph of the narrow accent.
var wideAccents = map[string]bool{"widehat": true, "widetilde": true, "widecheck": true}

// spaces maps a command of space to its width in mu (em/18).
var spaces = map[string]float64{
	",": 3, ":": 4, ";": 5, "!": -3, "quad": 18, "qquad": 36, " ": 6,
}

// opLimits marks the large operator symbols, such as \sum and \prod, that set
// their scripts as limits in display style. The named operators, such as
// \lim and \max, carry their own flag in namedOps.
var opLimits = map[string]bool{
	"sum": true, "prod": true, "coprod": true, "bigcup": true, "bigcap": true,
	"bigvee": true, "bigwedge": true, "bigoplus": true, "bigotimes": true,
	"bigodot": true, "biguplus": true, "bigsqcup": true,
}

// fontSwitch maps a font switch, such as \rm, to the alphabet it sets for the
// rest of the group.
var fontSwitch = map[string]alphabet{
	"rm":  alphabetFor("mathrm"),
	"bf":  alphabetFor("mathbf"),
	"it":  alphabetFor("mathit"),
	"sf":  alphabetFor("mathsf"),
	"tt":  alphabetFor("mathtt"),
	"cal": alphabetFor("mathcal"),
	"sl":  alphabetFor("mathit"), // no slanted math alphabet: italic
}

// xArrows maps an extensible arrow to the arrow that it draws.
var xArrows = map[string]rune{
	"xrightarrow": '→', "xleftarrow": '←', "xleftrightarrow": '↔',
	"xRightarrow": '⇒', "xLeftarrow": '⇐', "xLeftrightarrow": '⇔',
	"xhookrightarrow": '↪', "xhookleftarrow": '↩', "xmapsto": '↦',
	"xrightharpoonup": '⇀', "xrightharpoondown": '⇁',
	"xleftharpoonup": '↼', "xleftharpoondown": '↽',
}

// opInfo describes a named operator: the upright text, and whether it takes
// limits in display style.
type opInfo struct {
	text   string
	limits bool
}

// namedOps are the operators like \log. As in plain TeX, \lim, \liminf,
// \limsup, \max, \min, \sup, \inf, \det, \gcd and \Pr set limits in display
// style, and the others set their scripts at the side.
var namedOps = map[string]opInfo{
	"log": {"log", false}, "ln": {"ln", false}, "lg": {"lg", false}, "exp": {"exp", false},
	"sin": {"sin", false}, "cos": {"cos", false}, "tan": {"tan", false}, "cot": {"cot", false},
	"sec": {"sec", false}, "csc": {"csc", false},
	"sinh": {"sinh", false}, "cosh": {"cosh", false}, "tanh": {"tanh", false}, "coth": {"coth", false},
	"arcsin": {"arcsin", false}, "arccos": {"arccos", false}, "arctan": {"arctan", false},
	"min": {"min", true}, "max": {"max", true}, "inf": {"inf", true}, "sup": {"sup", true},
	"lim": {"lim", true}, "limsup": {"lim sup", true}, "liminf": {"lim inf", true},
	"det": {"det", true}, "gcd": {"gcd", true}, "Pr": {"Pr", true},
	"dim": {"dim", false}, "ker": {"ker", false}, "deg": {"deg", false},
	"hom": {"hom", false}, "arg": {"arg", false},
}

// envInfo describes an environment of \begin…\end.
type envInfo struct {
	kind        ArrayKind
	open, close rune // the outer delimiters, for ArrayMatrix
	// stretch is \arraystretch, which scales the strut of a row. amsmath sets
	// it for cases only: amsmath.sty:1106 is
	// \left\lbrace \def\arraystretch{1.2} \array{@{}l@{\quad}l@{}}. Zero
	// means 1.
	stretch float64
}

var envTable = map[string]envInfo{
	"matrix":      {kind: ArrayMatrix},
	"pmatrix":     {kind: ArrayMatrix, open: '(', close: ')'},
	"bmatrix":     {kind: ArrayMatrix, open: '[', close: ']'},
	"Bmatrix":     {kind: ArrayMatrix, open: '{', close: '}'},
	"vmatrix":     {kind: ArrayMatrix, open: '|', close: '|'},
	"Vmatrix":     {kind: ArrayMatrix, open: '‖', close: '‖'},
	"cases":       {kind: ArrayMatrix, open: '{', stretch: 1.2},
	"array":       {kind: ArrayArray},
	"aligned":     {kind: ArrayAligned},
	"split":       {kind: ArrayAligned},
	"gathered":    {kind: ArrayGathered},
	"smallmatrix": {kind: ArraySmallMatrix},
}

// symbols maps a control-sequence name to its glyph and atom class.
var symbols = map[string]sym{
	// greek lower
	"alpha": {'α', Ord}, "beta": {'β', Ord}, "gamma": {'γ', Ord}, "delta": {'δ', Ord},
	// \epsilon is the LUNATE one and \varepsilon the rounded one, not the other way
	// round: fontmath.ltx:179 puts \epsilon at cmmi slot "0F and :198 \varepsilon at
	// "22, and unicode-math-table.tex reads U+03F5 "greek lunate varepsilon symbol"
	// for \epsilon against U+03B5 "rounded small varepsilon" for \varepsilon. This
	// table had the pair the wrong way up, and so had \phi/\varphi below.
	"epsilon": {'ϵ', Ord}, "varepsilon": {'ε', Ord}, "zeta": {'ζ', Ord}, "eta": {'η', Ord},
	"theta": {'θ', Ord}, "vartheta": {'ϑ', Ord}, "iota": {'ι', Ord}, "kappa": {'κ', Ord},
	"varkappa": {'ϰ', Ord}, "digamma": {'ϝ', Ord}, // AMS (amssymb) lowercase-greek variants
	"lambda": {'λ', Ord}, "mu": {'μ', Ord}, "nu": {'ν', Ord}, "xi": {'ξ', Ord},
	"pi": {'π', Ord}, "varpi": {'ϖ', Ord}, "rho": {'ρ', Ord}, "varrho": {'ϱ', Ord},
	"sigma": {'σ', Ord}, "varsigma": {'ς', Ord}, "tau": {'τ', Ord}, "upsilon": {'υ', Ord},
	// unicode-math-table.tex: U+03D5 "/straightphi - small phi" is \phi, U+03C6
	// "curly or open small phi" is \varphi (fontmath.ltx:194 and :203).
	"phi": {'ϕ', Ord}, "varphi": {'φ', Ord}, "chi": {'χ', Ord}, "psi": {'ψ', Ord}, "omega": {'ω', Ord},
	"omicron": {'ο', Ord}, // KaTeX: U+03BF, a request of zuzu on 2026-11-20
	// greek upper
	"Gamma": {'Γ', Ord}, "Delta": {'Δ', Ord}, "Theta": {'Θ', Ord}, "Lambda": {'Λ', Ord},
	"Xi": {'Ξ', Ord}, "Pi": {'Π', Ord}, "Sigma": {'Σ', Ord}, "Upsilon": {'Υ', Ord},
	"Phi": {'Φ', Ord}, "Psi": {'Ψ', Ord}, "Omega": {'Ω', Ord},
	// big operators
	"sum": {'∑', Op}, "prod": {'∏', Op}, "coprod": {'∐', Op}, "int": {'∫', Op},
	"oint": {'∮', Op}, "iint": {'∬', Op}, "iiint": {'∭', Op},
	"bigcup": {'⋃', Op}, "bigcap": {'⋂', Op}, "bigvee": {'⋁', Op}, "bigwedge": {'⋀', Op},
	"bigoplus": {'⨁', Op}, "bigotimes": {'⨂', Op}, "bigodot": {'⨀', Op}, "bigsqcup": {'⨆', Op},
	"biguplus": {'⨄', Op}, // named operators (\lim, \max, …) live in namedOps, not here.
	// binary operators
	"times": {'×', Bin}, "div": {'÷', Bin}, "pm": {'±', Bin}, "mp": {'∓', Bin},
	"cdot": {'⋅', Bin}, "ast": {'∗', Bin}, "star": {'⋆', Bin}, "circ": {'∘', Bin},
	"bullet": {'∙', Bin}, "oplus": {'⊕', Bin}, "ominus": {'⊖', Bin}, "otimes": {'⊗', Bin},
	"oslash": {'⊘', Bin}, "odot": {'⊙', Bin}, "cup": {'∪', Bin}, "cap": {'∩', Bin},
	"uplus": {'⊎', Bin}, "sqcup": {'⊔', Bin}, "sqcap": {'⊓', Bin}, "vee": {'∨', Bin},
	"wedge": {'∧', Bin}, "setminus": {'∖', Bin}, "wr": {'≀', Bin}, "diamond": {'⋄', Bin},
	"bigtriangleup": {'△', Bin}, "bigtriangledown": {'▽', Bin},
	// unicode-math-table.tex:694/704 gives both as \mathbin at U+25B7/U+25C1,
	// as does fontmath.ltx:264-265 (\DeclareMathSymbol …{\mathbin}{letters}).
	// The ⊲ ⊳ of \vartriangleleft/right below are amssymb's RELATIONS, not these.
	"triangleleft": {'◁', Bin}, "triangleright": {'▷', Bin},
	"intercal": {'⊺', Bin}, "dotplus": {'∔', Bin},
	// relations
	"leq": {'≤', Rel}, "le": {'≤', Rel}, "geq": {'≥', Rel}, "ge": {'≥', Rel},
	"neq": {'≠', Rel}, "ne": {'≠', Rel}, "equiv": {'≡', Rel}, "approx": {'≈', Rel},
	"cong": {'≅', Rel}, "simeq": {'≃', Rel}, "sim": {'∼', Rel}, "propto": {'∝', Rel},
	"ll": {'≪', Rel}, "gg": {'≫', Rel}, "doteq": {'≐', Rel}, "asymp": {'≍', Rel},
	"in": {'∈', Rel}, "notin": {'∉', Rel}, "ni": {'∋', Rel}, "notni": {'∌', Rel}, "subset": {'⊂', Rel},
	"supset": {'⊃', Rel}, "subseteq": {'⊆', Rel}, "supseteq": {'⊇', Rel},
	"sqsubseteq": {'⊑', Rel}, "sqsupseteq": {'⊒', Rel}, "prec": {'≺', Rel}, "succ": {'≻', Rel},
	"preceq": {'⪯', Rel}, "succeq": {'⪰', Rel}, "models": {'⊨', Rel}, "vdash": {'⊢', Rel},
	"dashv": {'⊣', Rel}, "perp": {'⊥', Rel}, "mid": {'∣', Rel}, "parallel": {'∥', Rel},
	"doteqdot": {'≑', Rel}, "bowtie": {'⋈', Rel},
	// more relations (amssymb / stmaryrd / mathtools), from the corpus census
	"Coloneqq": {'⩴', Rel}, "Coloneq": {'⩴', Rel},
	"vDash": {'⊨', Rel}, "Vdash": {'⊩', Rel}, "Vvdash": {'⊪', Rel}, "nsim": {'≁', Rel},
	"sqsubset": {'⊏', Rel}, "sqsupset": {'⊐', Rel},
	"vartriangleleft": {'⊲', Rel}, "vartriangleright": {'⊳', Rel},
	"preccurlyeq": {'≼', Rel}, "succcurlyeq": {'≽', Rel},
	"between": {'≬', Rel}, "pitchfork": {'⋔', Rel}, "therefore": {'∴', Rel}, "because": {'∵', Rel},
	"rightsquigarrow": {'⇝', Rel}, "leadsto": {'⇝', Rel},
	"twoheadrightarrow": {'↠', Rel}, "twoheadleftarrow": {'↞', Rel},
	"rightrightarrows": {'⇉', Rel}, "leftleftarrows": {'⇇', Rel},
	"curvearrowright": {'↷', Rel}, "curvearrowleft": {'↶', Rel},
	"circlearrowright": {'↻', Rel}, "circlearrowleft": {'↺', Rel},
	// binary-operator and bracket aliases
	"land": {'∧', Bin}, "lor": {'∨', Bin}, "rtimes": {'⋊', Bin}, "ltimes": {'⋉', Bin},
	"amalg":   {'⨿', Bin},
	"boxplus": {'⊞', Bin}, "boxtimes": {'⊠', Bin}, "boxminus": {'⊟', Bin}, "boxdot": {'⊡', Bin},
	"llbracket": {'⟦', Open}, "rrbracket": {'⟧', Close},
	// dots and misc ordinary
	"hdots": {'⋯', Inner}, "dotsc": {'…', Inner}, "dotsb": {'⋯', Inner}, "dotsm": {'⋯', Inner}, "dotsi": {'⋯', Inner},
	"hslash": {'ℏ', Ord}, "bigstar": {'★', Ord},
	// arrows
	"leftarrow": {'←', Rel}, "gets": {'←', Rel}, "rightarrow": {'→', Rel}, "to": {'→', Rel},
	"leftrightarrow": {'↔', Rel}, "Leftarrow": {'⇐', Rel}, "Rightarrow": {'⇒', Rel},
	"Leftrightarrow": {'⇔', Rel}, "iff": {'⇔', Rel}, "mapsto": {'↦', Rel},
	"longrightarrow": {'⟶', Rel}, "longleftarrow": {'⟵', Rel}, "implies": {'⟹', Rel}, "impliedby": {'⟸', Rel},
	"Longrightarrow": {'⟹', Rel}, "Longleftarrow": {'⟸', Rel}, "Longleftrightarrow": {'⟺', Rel},
	"longmapsto": {'⟼', Rel},
	"uparrow":    {'↑', Rel}, "downarrow": {'↓', Rel}, "updownarrow": {'↕', Rel},
	"nearrow": {'↗', Rel}, "searrow": {'↘', Rel}, "swarrow": {'↙', Rel}, "nwarrow": {'↖', Rel},
	"hookrightarrow": {'↪', Rel}, "hookleftarrow": {'↩', Rel},
	// misc ordinary / symbols
	"infty": {'∞', Ord}, "partial": {'∂', Ord}, "nabla": {'∇', Ord}, "forall": {'∀', Ord},
	"exists": {'∃', Ord}, "nexists": {'∄', Ord}, "emptyset": {'∅', Ord}, "varnothing": {'∅', Ord},
	"neg": {'¬', Ord}, "lnot": {'¬', Ord}, "top": {'⊤', Ord}, "bot": {'⊥', Ord},
	"aleph": {'ℵ', Ord}, "beth": {'ℶ', Ord}, "gimel": {'ℷ', Ord}, "daleth": {'ℸ', Ord}, // AMS Hebrew
	"hbar": {'ℏ', Ord}, "ell": {'ℓ', Ord}, "wp": {'℘', Ord},
	"Re": {'ℜ', Ord}, "Im": {'ℑ', Ord}, "angle": {'∠', Ord}, "triangle": {'△', Ord},
	"backslash": {'\\', Ord}, "prime": {'′', Ord}, "surd": {'√', Ord},
	"clubsuit": {'♣', Ord}, "diamondsuit": {'♢', Ord}, "heartsuit": {'♡', Ord}, "spadesuit": {'♠', Ord},
	"flat": {'♭', Ord}, "natural": {'♮', Ord}, "sharp": {'♯', Ord},
	"dagger": {'†', Ord}, "ddagger": {'‡', Ord}, "S": {'§', Ord}, "P": {'¶', Ord},
	"cdots": {'⋯', Ord}, "ldots": {'…', Ord}, "vdots": {'⋮', Ord}, "ddots": {'⋱', Ord},
	"dots": {'…', Ord}, "sqrt": {'√', Ord},
	// delimiters (also used by \left \right)
	"langle": {'⟨', Open}, "rangle": {'⟩', Close}, "lceil": {'⌈', Open}, "rceil": {'⌉', Close},
	"lfloor": {'⌊', Open}, "rfloor": {'⌋', Close}, "vert": {'|', Ord}, "Vert": {'‖', Ord},
	"lbrace": {'{', Open}, "rbrace": {'}', Close}, "|": {'‖', Ord},
	// amsmath paired delimiters (\lvert…\rvert = |x|, \lVert…\rVert = ‖x‖): the
	// bodies \abs and \norm expand to, and used directly.
	"lvert": {'|', Open}, "rvert": {'|', Close},
	"lVert": {'‖', Open}, "rVert": {'‖', Close},
	"lgroup": {'⟮', Open}, "rgroup": {'⟯', Close},
	// amsmath colon-relations and negated / slanted relations
	"colon": {':', Punct}, "coloneqq": {'≔', Rel}, "coloneq": {'≔', Rel},
	"eqqcolon": {'≕', Rel}, "eqcolon": {'≕', Rel},
	"nmid": {'∤', Rel}, "nparallel": {'∦', Rel},
	"subsetneq": {'⊊', Rel}, "supsetneq": {'⊋', Rel},
	"subsetneqq": {'⫋', Rel}, "supsetneqq": {'⫌', Rel},
	"leqslant": {'⩽', Rel}, "geqslant": {'⩾', Rel},
	"lesssim": {'≲', Rel}, "gtrsim": {'≳', Rel},
	"lessgtr": {'≶', Rel}, "gtrless": {'≷', Rel},
	"trianglelefteq": {'⊴', Rel}, "trianglerighteq": {'⊵', Rel},
	"vartriangle": {'△', Rel}, "triangleq": {'≜', Rel},
	// amssymb ordinaries seen in the corpus
	"Diamond": {'◇', Ord}, "Box": {'□', Ord}, "square": {'□', Ord},
	"blacksquare": {'■', Ord}, "lozenge": {'◊', Ord}, "complement": {'∁', Ord},
	"circledast": {'⊛', Bin}, "circledcirc": {'⊚', Bin},
	// control symbols
	"{": {'{', Open}, "}": {'}', Close},
	// escaped specials: \% \# \& \_ \$ render the literal glyph (common in real
	// papers — a percent, a literal underscore, an ampersand, a dollar in math).
	"%": {'%', Ord}, "#": {'#', Ord}, "&": {'&', Ord},
	"_": {'_', Ord}, "$": {'$', Ord},
	// mathtools relation-punctuation seen in the corpus
	"vcentcolon": {'∶', Rel}, "dblcolon": {'∷', Rel},
	// upgreek: the UPRIGHT Greek alphabet, one \DeclareMathSymbol per name in
	// upgreek.sty. Each takes the same character as its italic counterpart above —
	// this table already keys \mu on U+03BC, the upright codepoint, so \upmu is the
	// same rune and the face decides. Added as a family: a paper that writes \upmu
	// writes \upsigma on the next line, and an unknown one drops the whole formula.
	"upalpha": {'α', Ord}, "upbeta": {'β', Ord}, "upchi": {'χ', Ord}, "updelta": {'δ', Ord},
	"upepsilon": {'ϵ', Ord}, "upeta": {'η', Ord}, "upgamma": {'γ', Ord}, "upiota": {'ι', Ord},
	"upkappa": {'κ', Ord}, "uplambda": {'λ', Ord}, "upmu": {'μ', Ord}, "upnu": {'ν', Ord},
	"upomega": {'ω', Ord}, "upphi": {'ϕ', Ord}, "uppi": {'π', Ord}, "uppsi": {'ψ', Ord},
	"uprho": {'ρ', Ord}, "upsigma": {'σ', Ord}, "uptau": {'τ', Ord}, "uptheta": {'θ', Ord},
	"upupsilon": {'υ', Ord}, "upvarepsilon": {'ε', Ord}, "upvarphi": {'φ', Ord}, "upvarpi": {'ϖ', Ord},
	"upvartheta": {'ϑ', Ord}, "upxi": {'ξ', Ord}, "upzeta": {'ζ', Ord},
	"Updelta": {'Δ', Ord}, "Upgamma": {'Γ', Ord}, "Uplambda": {'Λ', Ord}, "Upomega": {'Ω', Ord},
	"Upphi": {'Φ', Ord}, "Uppi": {'Π', Ord}, "Uppsi": {'Ψ', Ord}, "Upsigma": {'Σ', Ord},
	"Uptheta": {'Θ', Ord}, "Upupsilon": {'Υ', Ord}, "Upxi": {'Ξ', Ord},
	// amssymb / wasysym / fontmath ordinaries and relations the corpus drops on.
	// Codepoints read from unicode-math-table.tex and confirmed against
	// UnicodeData.txt, not from the AMS font slot, which is an encoding not a
	// character:
	//   U+2713 CHECK MARK                 \checkmark      amssymb AMSa "58
	//   U+21E2 RIGHTWARDS DASHED ARROW    \dashrightarrow unicode-math \rightdasharrow
	//   U+22E1 DOES NOT SUCCEED OR EQUAL  \nsucceq        amssymb AMSb "0F
	//   U+00A7 SECTION SIGN               \mathsection    fontmath.ltx
	//   U+2394                            \hexagon        wasysym
	"checkmark": {'✓', Ord}, "mathsection": {'§', Ord},
	"dashrightarrow": {'⇢', Rel}, "nsucceq": {'⋡', Rel},
	"hexagon": {'\u2394', Ord},
	// \varTheta is amsmath's, and it is NOT U+03F4: amsmath.sty:388 declares it at
	// cmmi slot "02, the same slot \Theta occupies in the letters family, so it is
	// the ITALIC capital theta — the upright U+03F4 "greek capital theta symbol"
	// would be a different letter.
	"varTheta": {'Θ', Ord},
	// \mathellipsis is not a glyph at all: fontmath.ltx:512 defines it as
	// \mathinner{\ldotp\ldotp\ldotp}, so it sets exactly as \ldots.
	"mathellipsis": {'…', Ord},
	// dotless letters and angle ordinaries (amssymb) seen in the corpus
	"imath": {'ı', Ord}, "jmath": {'ȷ', Ord},
	"measuredangle": {'∡', Ord}, "sphericalangle": {'∢', Ord},
	// ── 57 symbols the dropped-equation census named, each sourced before it was
	// added. go-tex/engine#466: these are the entries a 999-paper census of the arXiv
	// corpus reports as the first unknown command of an equation the maths layer
	// refused, and together they account for 1930 of the 11292 equations it drops.
	//
	// The CODEPOINT comes from unicode-math-table.tex. The CLASS comes from the
	// package that actually declares the command — amssymb, amsfonts, fontmath.ltx,
	// latexsym or stmaryrd — because that is what a document loading it compiles
	// with, and only from unicode-math where none of them defines it. Where the two
	// disagree the line says so; it is not a typo. Reading the sources first changed
	// four entries a plausible guess would have got wrong:
	//
	//   \blacktriangleleft is the LARGE triangle U+25C0, not the small U+25C2, while
	//   \blacktriangle IS the small U+25B4 — the up/down pair and the left/right pair
	//   take different sizes (unicode-math-table.tex:691,703).
	//
	//   \triangledown is U+25BF, the SMALL open triangle; U+25BD is \bigtriangledown,
	//   which this table already holds. Mapping both to U+25BD would have made two
	//   amssymb symbols identical.
	//
	//   \blacktriangleleft/right are amssymb RELATIONS (amssymb.sty:120-121), which
	//   unicode-math classes as ordinary. \Join is a relation too (latexsym.sty:60),
	//   not the n-ary operator unicode-math gives it.
	//
	//   \smallsetminus takes U+2216, which is also this table's \setminus. Unicode has
	//   no small variant; unicode-math resolves the clash by moving \setminus to
	//   U+29F5, which we do not do because U+2216 is what LaTeX's \setminus means. So
	//   the two render alike — right in meaning, one size too large.
	//
	// Four census entries are deliberately NOT here: \widecheck (97 equations),
	// \underrightarrow (39), \underleftarrow (1) and \dddot (1) are stretchy or
	// combining ACCENTS, and a combining character standing alone in a symbol table
	// is a defect, not a symbol. They need the accent machinery.
	//
	// Every rune below has a glyph in the embedded font, asserted for the whole table
	// by TestEverySymbolHasAGlyphInTheEmbeddedFont — a rune without one is typeset as
	// .notdef, which looks like content and is worse than the drop it replaced.
	"blacktriangleleft": {'◀', Rel},   // U+25C0 (large) left triangle, filled — amssymb.sty:121 [202 eq; u-m says mathord]
	"Bbbk":              {'𝕜', Ord},   // U+1D55C mathematical double-struck small k — amssymb.sty:261 [188 eq; u-m says mathalpha]
	"restriction":       {'↾', Rel},   // U+21BE up harpoon-right — amssymb.sty:68 (\let\restriction\upharpoonright) [165 eq]
	"lrcorner":          {'⌟', Close}, // U+231F lower right corner — amsfonts.sty:77 [152 eq]
	"blacktriangle":     {'▴', Ord},   // U+25B4 up triangle, filled — amssymb.sty:123 [139 eq]
	// ── The CLOSING half of three pairs, added because the 57 above SURFACED them. A
	// paper writing \llparenthesis x \rrparenthesis failed on the opener, so the
	// census only ever named the opener; supplying it moved the failure one command to
	// the right. Measured, not guessed: \rrparenthesis appeared with 31 equations,
	// \urcorner with 11 and \rbrack with 1 in the A/B that added the others.
	//
	// \llcorner is deliberately still absent. If any paper wrote it, IT would be the
	// first unknown and the census would name it — the absence is in the place that
	// answers the question, which is why this is not the same argument as the three
	// above.
	// ── 9 more the census named, and the reason they were missed the first time is a
	// defect in my own sweep: it looked unicode-math-table.tex up BY NAME, and
	// unicode-math uses DIFFERENT names for glyphs amssymb already had. So \rhd,
	// \bigcirc and \blacklozenge all reported "no codepoint exists" when the
	// codepoint was in the table under \vartriangleright, \mdlgwhtcircle and
	// \mdlgblklozenge. Each entry below is verified twice: by the Unicode character
	// NAME, and by unicode-math's own entry at that codepoint.
	//
	// Four of them are amsfonts' \mathbin ALIASES of glyphs this table already holds
	// as \mathrel — the documented amssymb pair, declared on four consecutive lines
	// (amsfonts.sty:157-160). The class is the only difference and it is the whole
	// point: \lhd is a binary operation, \vartriangleleft a relation, and they take
	// different spacing. \unrhd is not in the census; it is here because it is the
	// fourth line of the same declaration.
	// MnSymbol's own two, verified by Unicode name and by unicode-math's entry at each
	// codepoint. 159 equations over 2 papers.
	//
	// ⛔ The paper carrying 157 of them redeclares \medsquare as \mathrel in its own
	// modalops.sty, while MnSymbol.sty:998 says \mathbin. MnSymbol wins here because it
	// is the package of record — and the choice is UNOBSERVABLE in that paper anyway: it
	// writes \boxmodal{\medsquare}, which wraps the symbol in
	// \modalop{\text{\raisebox{\scalebox{\ensuremath{…}}}}}, so an outer class governs.
	//
	// The paper's five other names — \medsquaredot, \medsquareminus, \medsquarevert,
	// \medsquareplus, \medsquaretimes — are NOT MnSymbol commands, they are declared in
	// that paper's own .sty. They are deliberately absent: a general table is not the
	// place for one paper's names, and the fix that serves them is the engine honouring
	// a paper's \DeclareMathSymbol. Each is used exactly ONCE in the body, against 480
	// uses of \medsquare and 93 of \medsquaredot, so almost nothing rests on them.
	//
	// And \medsquarevert could not be served in any case: Unicode has no squared
	// vertical bar, and in a paper whose subject is telling seven modalities apart, a
	// substituted glyph would merge two of them.
	"medsquare":  {'◻', Bin}, // U+25FB WHITE MEDIUM SQUARE — MnSymbol.sty:998 [157 eq]
	"meddiamond": {'◇', Bin}, // U+25C7 WHITE DIAMOND — MnSymbol.sty:1005 [2 eq]
	"lhd":        {'⊲', Bin}, // U+22B2 NORMAL SUBGROUP OF — amsfonts.sty:157 [87 eq]
	"rhd":        {'⊳', Bin}, // U+22B3 CONTAINS AS NORMAL SUBGROUP — amsfonts.sty:159 [92 eq]
	"unlhd":      {'⊴', Bin}, // U+22B4 NORMAL SUBGROUP OF OR EQUAL TO — amsfonts.sty:158 [2 eq]
	"unrhd":      {'⊵', Bin}, // U+22B5 CONTAINS AS NORMAL SUBGROUP OR EQUAL — amsfonts.sty:160
	// \bigcirc is NOT \circ: U+25CB is the WHITE CIRCLE, U+2218 the much smaller RING
	// OPERATOR this table already holds as \circ. Here fontmath and unicode-math agree
	// on the class for once.
	"bigcirc": {'○', Bin}, // U+25CB WHITE CIRCLE — fontmath.ltx:294 [70 eq]
	// amssymb says \mathord where unicode-math's \mdlgblklozenge says mathbin; the
	// package that declares the command wins, as everywhere in this table.
	"blacklozenge": {'⧫', Ord}, // U+29EB BLACK LOZENGE — amssymb.sty:51 [62 eq]
	// stmaryrd says \mathord where unicode-math's \downzigzagarrow says mathrel.
	"lightning": {'↯', Ord}, // U+21AF DOWNWARDS ZIGZAG ARROW — stmaryrd.sty:122 [14 eq]
	// AMS's small variants. Unicode has no small smile or frown, exactly as it has no
	// small set minus, so these take the same codepoints as \smile and \frown and set
	// alike — right in meaning, one size too large. Stated rather than hidden.
	"smallsmile": {'⌣', Rel}, // U+2323 — amssymb.sty:140, renders as \smile [3 eq]
	"smallfrown": {'⌢', Rel}, // U+2322 — amssymb.sty:141, renders as \frown [2 eq]
	// \intop is \int with \displaylimits (fontmath.ltx:253); same glyph, and Op is
	// what puts the limits where \nolimits/\limits then move them.
	"intop": {'∫', Op}, // U+222B — fontmath.ltx:253 [2 eq]
	//
	// ⛔ LEFT OUT, and each for the reason \llceil is out — no codepoint carries the
	// meaning, and a wrong character on a display is worse than a reported drop:
	//
	//   \moo (52 eq) and \fatsemi (1) are stmaryrd's own glyphs in the stmry font.
	//   \lhook (3) is a Computer Modern PIECE of \hookrightarrow, not a character.
	//   \nsubseteqq (1) has no precomposed negation: U+2AC5 is \subseteqq ITSELF,
	//   which is what my first proposal got wrong until the Unicode name was checked.
	"rrparenthesis":      {'\u2988', Close}, // z notation right image bracket — stmaryrd.sty:170 [31 eq]
	"urcorner":           {'\u231D', Close}, // upper right corner — amsfonts.sty:75 [11 eq]
	"rbrack":             {']', Close},      // latex.ltx:565 is \def\rbrack{]} — literally the character [1 eq]
	"Lbag":               {'⟅', Open},       // U+27C5 left s-shaped bag delimiter (Unicode name) — stmaryrd.sty:166 [109 eq]
	"triangledown":       {'▿', Ord},        // U+25BF down triangle, open — amssymb.sty:124 [103 eq]
	"smallsetminus":      {'∖', Bin},        // U+2216 small set minus (cf. reverse solidus) — amssymb.sty:251 [85 eq]
	"blacktriangleright": {'▶', Rel},        // U+25B6 (large) right triangle, filled — amssymb.sty:120 [74 eq; u-m says mathord]
	"rightharpoonup":     {'⇀', Rel},        // U+21C0 right harpoon-up — fontmath.ltx:351 [67 eq]
	"blacktriangledown":  {'▾', Ord},        // U+25BE down triangle, filled — amssymb.sty:119 [59 eq]
	"longleftrightarrow": {'⟷', Rel},        // U+27F7 long left right arrow — unicode-math-table.tex:865 [56 eq]
	"upharpoonright":     {'↾', Rel},        // U+21BE /upharpoonright /restriction a: up harpoon-right — amssymb.sty:67 [56 eq]
	"Downarrow":          {'⇓', Rel},        // U+21D3 down double arrow — fontmath.ltx:479 [55 eq]
	"frown":              {'⌢', Rel},        // U+2322 down curve — fontmath.ltx:348 [40 eq]
	"Uparrow":            {'⇑', Rel},        // U+21D1 up double arrow — fontmath.ltx:477 [38 eq]
	"looparrowright":     {'↬', Rel},        // U+21AC right arrow-looped — amssymb.sty:81 [38 eq]
	"eqdef":              {'≝', Rel},        // U+225D equals by definition — unicode-math-table.tex:421 [30 eq]
	"llparenthesis":      {'⦇', Open},       // U+2987 z notation left image bracket — stmaryrd.sty:169 [28 eq]
	"rightleftharpoons":  {'⇌', Rel},        // U+21CC right harpoon over left — amssymb.sty:55 [24 eq]
	"nrightarrow":        {'↛', Rel},        // U+219B not right arrow — amssymb.sty:228 [22 eq]
	"sslash":             {'⫽', Bin},        // U+2AFD double solidus operator — stmaryrd.sty:102 [22 eq]
	"backsim":            {'∽', Rel},        // U+223D reverse similar — amssymb.sty:163 [18 eq]
	"bigsqcap":           {'⨅', Op},         // U+2A05 n-ary square intersection operator — stmaryrd.sty:196 [17 eq]
	"mho":                {'℧', Ord},        // U+2127 conductance — amssymb.sty:239 [14 eq]
	"eth":                {'ð', Ord},        // U+00F0 eth — amssymb.sty, KaTeX, a request of zuzu on 2026-11-20
	"Finv":               {'Ⅎ', Ord},        // U+2132 turned capital F — amssymb.sty, KaTeX, a request of zuzu on 2026-11-20
	"Game":               {'⅁', Ord},        // U+2141 turned sans-serif capital G — amssymb.sty, KaTeX, a request of zuzu on 2026-11-20
	"rightleftarrows":    {'⇄', Rel},        // U+21C4 right arrow over left arrow — amssymb.sty:75 [12 eq]
	"ulcorner":           {'⌜', Open},       // U+231C upper left corner — amsfonts.sty:74 [11 eq]
	"succsim":            {'≿', Rel},        // U+227F succeeds, similar — amssymb.sty:83 [8 eq]
	"nsubseteq":          {'⊈', Rel},        // U+2288 not subset, equals — amssymb.sty:213 [7 eq]
	"Cup":                {'⋓', Bin},        // U+22D3 /cup /doublecup b: double union — amssymb.sty:144 [6 eq]
	"geqq":               {'≧', Rel},        // U+2267 greater, double equals — amssymb.sty:107 [6 eq]
	"multimap":           {'⊸', Rel},        // U+22B8 /multimap a: — amssymb.sty:86 [6 eq]
	"rightarrowtail":     {'↣', Rel},        // U+21A3 right arrow-tailed — amssymb.sty:72 [6 eq]
	"Join":               {'⨝', Rel},        // U+2A1D join — latexsym.sty:60 [5 eq; u-m says mathop]
	"Rbag":               {'⟆', Close},      // U+27C6 right s-shaped bag delimiter (Unicode name) — stmaryrd.sty:167 [5 eq]
	"dotminus":           {'∸', Bin},        // U+2238 minus sign, dot above — unicode-math-table.tex:382 [5 eq]
	"mapsfrom":           {'↤', Rel},        // U+21A4 maps to, leftward — unicode-math-table.tex:242 [5 eq]
	"nleq":               {'≰', Rel},        // U+2270 not less-than-or-equal — amssymb.sty:173 [5 eq]
	"nvDash":             {'⊭', Rel},        // U+22AD not vertical, double dash — amssymb.sty:221 [5 eq]
	"leftrightarrows":    {'⇆', Rel},        // U+21C6 left arrow over right arrow — amssymb.sty:74 [4 eq]
	"leqq":               {'≦', Rel},        // U+2266 less, double equals — amssymb.sty:100 [4 eq]
	"nprec":              {'⊀', Rel},        // U+2280 not precedes — amssymb.sty:177 [4 eq]
	"Subset":             {'⋐', Rel},        // U+22D0 double subset — amssymb.sty:142 [3 eq]
	"lll":                {'⋘', Rel},        // U+22D8 /ll /lll /llless r: triple less-than — amssymb.sty:156 [3 eq]
	"nRightarrow":        {'⇏', Rel},        // U+21CF not implies — amssymb.sty:230 [3 eq]
	"precneqq":           {'⪵', Rel},        // U+2AB5 precedes above not equal to — amssymb.sty:193 [3 eq]
	"leftrightharpoons":  {'⇋', Rel},        // U+21CB left harpoon over right — amssymb.sty:56 [2 eq]
	"smile":              {'⌣', Rel},        // U+2323 up curve — fontmath.ltx:347 [2 eq]
	"approxeq":           {'≊', Rel},        // U+224A approximate, equals — amssymb.sty:254 [1 eq]
	"fint":               {'⨏', Op},         // U+2A0F integral average with slash — unicode-math-table.tex:1145 [1 eq]
	"gtrapprox":          {'⪆', Rel},        // U+2A86 greater-than or approximate — amssymb.sty:85 [1 eq]
	"gtreqqless":         {'⪌', Rel},        // U+2A8C greater-than above double-line equal above less-than — amssymb.sty:129 [1 eq]
	"lbrack":             {'[', Open},       // U+005B left square bracket — unicode-math-table.tex:22 [1 eq]
	"lessapprox":         {'⪅', Rel},        // U+2A85 less-than or approximate — amssymb.sty:94 [1 eq]
	"lneq":               {'⪇', Rel},        // U+2A87 less-than and single-line not equal to — amssymb.sty:183 [1 eq]
	"lnsim":              {'⋦', Rel},        // U+22E6 less, not similar — amssymb.sty:189 [1 eq]
	"ncong":              {'≇', Rel},        // U+2247 not congruent with — amssymb.sty:200 [1 eq]
}

// ── the layout halves ───────────────────────────────────────────────────────

// kern returns an empty box of the width w.
func (l *layout) kern(w float64) *box { b := newBox(Ord); b.w = w; return b }

// primeBox sets n primes (′ ″ ‴ …) side by side, in the style st.
//
//go:noinline
func (l *layout) primeBox(n int, st Style) *box {
	px := l.px(st)
	out := newBox(Ord)
	for range n {
		b, ok := l.glyphBox('′', px, Ord)
		if !ok {
			l.missing('′')
			continue
		}
		bw := b.w
		out.h, out.d = max(out.h, b.h), max(out.d, b.d)
		place(out, b, out.w, 0)
		out.w += bw
	}
	return out
}

// mustGlyph returns the box of the glyph of r, or an empty box of width zero
// when the face has no glyph for r.
//
// okage: the character without a glyph is logged with WithLogger.
//
//go:noinline
func (l *layout) mustGlyph(r rune, px float64, cls Class) *box {
	if b, ok := l.glyphBox(r, px, cls); ok {
		return b
	}
	l.missing(r)
	return newBox(cls)
}

// opName sets the characters of s side by side, with no space between them,
// each mapped to the variant v: the text of an Atom, such as "sin" or a
// letter, and a TextRun. The box is an Ord. It is e.opName of the source,
// and mapAlpha with e.mustGlyph for one character. The parser makes the thin
// space of "lim sup" a Space, so s holds no space between words, and a
// space of a TextRun is the glyph of the space of the face.
//
//go:noinline
func (l *layout) opName(s string, v Variant, px float64) *box {
	out := newBox(Ord)
	for i, c := range s {
		r := mapVariant(c, v)
		b := l.mustGlyph(r, px, Ord)
		if i == 0 && len(b.items) == 1 && utf8.RuneLen(c) == len(s) {
			// One glyph of one character.
			out.char = !l.face.IsMathExtendedShape(b.items[0].Glyph)
			out.glyph, out.kern, out.gid, out.gpx = b.glyph, b.kern, b.gid, b.gpx
		}
		// okage: the italic correction of the box is that of its last
		// glyph (phase 26 of D78).
		out.ic = b.ic
		bw := b.w
		out.h, out.d = max(out.h, b.h), max(out.d, b.d)
		place(out, b, out.w, 0)
		out.w += bw
	}
	return out
}
