// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// This file ports the tests of math_test.go of the source. The first part
// reads the parser: the errors, the scripts with no nucleus, the alphabets,
// the classes and \ifmmode. The second part, after mathLayoutOK, lays out:
// a test that read the SVG text reads the items of the Box, and a test that
// read a metric in whole pixels reads the exact metric, computed again from
// the font (D78 section 7, item 1).

// mustParse parses tex and fails the test on an error.
func mustParse(t *testing.T, tex string) List {
	t.Helper()
	l, err := Parse(tex)
	if err != nil {
		t.Fatalf("Parse(%q): %v", tex, err)
	}
	return l
}

// as returns n as a T, and fails the test when it is not one.
func as[T any](t *testing.T, n any) T {
	t.Helper()
	v, ok := n.(T)
	if !ok {
		t.Fatalf("%#v is not a %T", n, v)
	}
	return v
}

// sameTree reports whether a and b parse to the same tree. The positions of
// the tokens are not in the tree, so two spellings of one formula compare
// equal.
func sameTree(t *testing.T, a, b string) bool {
	t.Helper()
	return reflect.DeepEqual(mustParse(t, a), mustParse(t, b))
}

func TestErrors(t *testing.T) {
	for _, tex := range []string{
		`\nope`,                          // unknown command
		`a}`,                             // unexpected close brace
		`{a`,                             // missing close brace
		`&`,                              // an alignment tab outside a matrix
		`\frac{a}`,                       // missing frac arg
		`x^`,                             // missing script atom
		`\sqrt[3]{`,                      // missing radical body
		`\sqrt[3`,                        // missing ]
		`\left(`,                         // \left without \right
		`\left`,                          // \left without delimiter
		`\begin`,                         // \begin without {env}
		`\begin{nope} a \end{nope}`,      // unknown env
		`\begin{matrix} a`,               // env without \end
		`\begin{matrix`,                  // unterminated \begin{
		`\begin{matrix} a \end{bmatrix}`, // mismatched \end
		`\begin{matrix} a \end`,          // \end without {env}
		`\begin{matrix} a \end{matrix`,   // unterminated \end{
		`\hat`,                           // accent without arg
		`\mathbb`,                        // alphabet without arg
		`\frac{\nope}{b}`, `\frac{a}{\nope}`, `\sqrt{\nope}`, `\sqrt[\nope]{x}`,
		`\hat{\nope}`, `\overline{\nope}`, `\underline{\nope}`, `\mathbb{\nope}`,
		`\left(\nope\right)`, `\left(a\nope`, `\begin{matrix}\nope\end{matrix}`,
		`x^\nope`, `x_\nope`, `{\nope}`, `\left\zzz a \right)`,
		`\left( x \right\zzz`,
	} {
		t.Run(tex, func(t *testing.T) {
			_, err := Parse(tex)
			if err == nil {
				t.Fatalf("Parse(%q) should fail", tex)
			}
			var se *SyntaxError
			if !errors.As(err, &se) || !errors.Is(err, okage.ErrTeXMathSyntax) {
				t.Errorf("Parse(%q) = %v, want a *SyntaxError that wraps ErrTeXMathSyntax", tex, err)
			}
		})
	}
}

func TestCoverageCorners(t *testing.T) {
	for _, ok := range []string{`\ifmmode a+b`, `x\fi`, `y\else z`, `\mathpunct{,}x`, `\mathinner{x}`, `\mathop{X}`} {
		if _, err := Parse(ok); err != nil {
			t.Errorf("Parse(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{
		`\left^ a \right)`, `\left_ a \right)`, `\left' a \right)`,
		`\binom{\zz}{k}`, `\binom{a}{\zz}`, `\overset{\zz}{x}`, `\overset{a}{\zz}`,
		`\underset{\zz}{x}`, `\substack{\zz}`, `\substack{a`,
		`\not\zz`, `\ket{\zz}`, `\ketbra{\zz}{y}`, `\ketbra{x}{\zz}`,
		`\boxed{\zz}`, `\underbrace{\zz}`, `\phantom{\zz}`, `\bigl\zz`, `\xrightarrow{\zz}`, `\xrightarrow[\zz]{x}`,
		`\ensuremath{\zz}`, `\mathbin{\zz}`, `\mathscr{\zz}`,
		`\smash[t`, `\smash{a \over \sqrt}`, `\overrightarrow{a \over \sqrt}`,
		`{a \over \sqrt}`, `\left(x\middle\right)`,
	} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): want an error", bad)
		}
	}
}

// A script or a prime with no nucleus attaches to an empty nucleus, as a
// footnote mark such as $^1$.
func TestEmptyBaseScripts(t *testing.T) {
	mustParse(t, `x + ^1`) // the script attaches to the +, as in the source
	for _, tex := range []string{`^1`, `_2`, `^{1}`, `_{ab}`, `'`, `^a_b`} {
		l := mustParse(t, tex)
		s, ok := l[len(l)-1].(Scripts)
		if !ok {
			t.Errorf("Parse(%q) ends with %T, want Scripts", tex, l[len(l)-1])
			continue
		}
		if b, ok := s.Base.(List); !ok || len(b) != 0 {
			t.Errorf("Parse(%q): the base is %#v, want the empty List", tex, s.Base)
		}
	}
	got := as[Scripts](t, mustParse(t, `x'^{2}`)[0])
	if got.Primes != 1 || got.Sup == nil || got.Sub != nil {
		t.Errorf(`x'^{2} = %#v, want one prime and a superscript`, got)
	}
}

func TestDefaultSizeAndEmpty(t *testing.T) {
	l := mustParse(t, `{}`)
	if a, ok := l[0].(Atom); !ok || a.Class != Ord || len(as[List](t, a.Body)) != 0 {
		t.Errorf(`{} = %#v, want an empty Ord group`, l)
	}
	if l := mustParse(t, ``); len(l) != 0 {
		t.Errorf(`Parse("") = %#v, want the empty List`, l)
	}
}

func TestAlphabets(t *testing.T) {
	cases := []struct {
		name string
		in   rune
		want rune
	}{
		{"mathbb", 'R', 0x211D}, {"mathbb", 'A', 0x1D538}, {"mathbb", '0', 0x1D7D8},
		{"mathcal", 'L', 0x2112}, {"mathcal", 'A', 0x1D49C},
		{"mathfrak", 'H', 0x210C}, {"mathfrak", 'a', 0x1D51E},
		{"mathbf", 'a', 0x1D41A}, {"mathsf", 'A', 0x1D5A0}, {"mathtt", 'a', 0x1D68A},
		{"mathit", 'h', 0x210E}, {"text", 'x', 'x'}, {"mathrm", 'y', 'y'},
		{"boldsymbol", 'a', 0x1D482}, {"mathbf", '5', 0x1D7D3}, {"mathbf", '+', '+'},
		{"mathbbm", '1', 0x1D7D9}, {"mathbbm", 'R', 0x211D}, {"mathbbm", 'A', 0x1D538},
		{"mathbbmss", '1', 0x1D7D9}, {"mathbbb", 'Z', 0x2124},
	}
	for _, c := range cases {
		if got := mapVariant(c.in, alphabetFor(c.name).variant); got != c.want {
			t.Errorf("%s(%q) = %U, want %U", c.name, c.in, got, c.want)
		}
	}
	if alphabetFor("bogus").set {
		t.Error("an unknown alphabet must not be set")
	}
	if got := mapVariant('7', Calligraphic); got != '7' {
		t.Errorf("mathcal digit = %q", got)
	}
}

func TestCharClassAndMathItalic(t *testing.T) {
	pairs := map[rune]Class{'+': Bin, '=': Rel, '(': Open, ')': Close, ',': Punct, 'a': Ord}
	for r, c := range pairs {
		if got := charClass(r); got != c {
			t.Errorf("charClass(%q) = %d, want %d", r, got, c)
		}
	}
	if mathItalic('a') != 0x1D44E || mathItalic('A') != 0x1D434 || mathItalic('h') != 0x210E || mathItalic('1') != '1' {
		t.Error("mathItalic mapping wrong")
	}
}

// \ifmmode is always true here: its true branch is kept and the \else branch
// dropped.
func TestIfmmodeResolves(t *testing.T) {
	for _, c := range [][2]string{
		{`\ifmmode a+b\else\nosuchcommand\fi`, `a+b`},
		{`x = \ifmmode\mathbb{R}\else R\fi`, `x = \mathbb{R}`},
		{`\sum_{\ifmmode i=1\else i\fi}^{n} a_i`, `\sum_{i=1}^{n} a_i`},
		{`\ifmmode\ifmmode y\fi\else z\fi`, `y`},
		{`k\ifmmode+1\fi`, `k+1`},
		{`\ifmmode a+b`, `a+b`},
		{`x\fi`, `x`},
		{`y\else z`, `yz`},
		{`\ifmmode a \else b \ifmmode c \else d \fi e \fi f`, `af`},
		{`\ifmmode a \ifmmode b \fi c \else d \fi`, `abc`},
		{`\ifmmode a \ifmmode b \fi`, `ab`},
	} {
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q does not parse as %q", c[0], c[1])
		}
	}
}

// mathLayoutOK lays out tex in display and in text style, and checks that the
// box is finite. It is renderOK of the source, which checked the SVG text.
func mathLayoutOK(t *testing.T, tex string) *Box {
	t.Helper()
	var b *Box
	for _, st := range []Style{Display, Text} {
		b = mustLayout(t, tex, 32, st)
		checkBox(t, tex, b)
	}
	return b
}

// mathLayoutAll lays out each of texs, each in a subtest.
func mathLayoutAll(t *testing.T, texs []string) {
	t.Helper()
	for _, tex := range texs {
		t.Run(tex, func(t *testing.T) { mathLayoutOK(t, tex) })
	}
}

func TestFeatures(t *testing.T) {
	mathLayoutAll(t, []string{
		`a + b = c`,
		`x^2 - 2x + 1 \leq 0`,
		`a_i + b^j - c_k^l`,
		`\frac{a}{b}`, `\dfrac{a}{b}`, `\tfrac{a}{b}`,
		`\sqrt{x}`, `\sqrt[3]{x+y}`,
		`\left( a \right)`, `\left[ \frac{a}{b} \right]`, `\left\{ x \right\}`,
		`\left. a \right|`, `\left\langle x \right\rangle`,
		`\hat{x} \bar{y} \vec{v} \tilde{n} \dot{a} \ddot{b} \check{c} \breve{d} \acute{e} \grave{f}`,
		`\overline{a+b}`, `\underline{xyz}`,
		`\mathbb{RCHNPQZ} \mathbb{R}`, `\mathcal{ABEFHILMR}`, `\mathfrak{CHIRZg}`,
		`\mathbf{x} \mathsf{y} \mathtt{z} \mathit{w} \mathrm{d} \text{if}`,
		`f'(x) g''(x) h'''`,
		`\sum_{i=1}^{n} i`, `\prod_k a_k`, `\int_0^1 f`, `\lim_{x\to 0} f`,
		`\begin{matrix} a & b \\ c & d \end{matrix}`,
		`\begin{pmatrix} a & b \\ c & d \end{pmatrix}`,
		`\begin{bmatrix} 1 \\ 2 \\ 3 \end{bmatrix}`,
		`\begin{vmatrix} a & b \\ c & d \end{vmatrix}`,
		`\begin{Vmatrix} a \end{Vmatrix}`, `\begin{Bmatrix} a \end{Bmatrix}`,
		`\begin{cases} 1 & x>0 \\ 0 & x\leq 0 \end{cases}`,
		`a \, b \: c \; d \! e \quad f \qquad g`,
		`\alpha\beta\gamma\Delta\Omega \infty \partial \nabla \forall \exists`,
		`x \in A \cup B, \ y \notin C \cap D`,
		`{a}`, `{}`, `\displaystyle \frac{a}{b}`, `\textstyle x`, `\scriptstyle y`,
		`\mathbb{R}^n \to \mathbb{R}^m`,
	})
}

// The paired delimiters of amsmath, the relations with a colon and the other
// symbols that real papers need lay out.
func TestAddedSymbols(t *testing.T) {
	mathLayoutAll(t, []string{
		`\lvert x \rvert`, `\lVert v \rVert`, `\left\lvert a \right\rvert`,
		`f \colon A \to B`, `x \coloneqq y`, `a \eqqcolon b`,
		`p \nmid q`, `A \subsetneq B`, `x \leqslant y \geqslant z`,
		`u \lesssim v \gtrsim w`, `\Diamond \square \blacksquare`,
		`50\%`, `a\#b`, `x\_y`, `p\&q`, `\$5`,
		`a \vcentcolon= b`, `x \dblcolon y`,
		`\imath + \jmath`, `\measuredangle ABC`, `\varnothing \sphericalangle`,
		`\varkappa`, `\kappa \ne \varkappa`, `\digamma`, `\varkappa_{ij} + \digamma`,
		`\aleph \beth \gimel \daleth`,
		`\normalfont x = y`, `{\bfseries a} + b`, `\small z^2`, `a \par b`,
		`\smash{\frac{1}{2}}`, `\smash[t]{x^2} + 1`, `\relax A \ignorespaces B`,
		`{a \over b}`, `x={n+1 \over 2}+y`, `{a \atop b}`, `{n \choose k}`,
	})
}

// \mathds of dsfont is the double-struck alphabet, with digits: \mathds{1}
// is the indicator 𝟙.
func TestMathdsAlphabet(t *testing.T) {
	mathLayoutAll(t, []string{`\mathds{R}`, `\mathds{1}`, `\mathds{Z}_{\geq 0}`, `\mathbb{1}`, `\mathbb{0}`})
	b := mathLayoutOK(t, `\mathds{1}`)
	if g := itemsOf(b, ItemGlyph); len(g) != 1 || g[0].Text != "𝟙" {
		t.Errorf(`\mathds{1} draws %+v, want one glyph for 𝟙`, g)
	}
}

// The symbols and alphabets of the census of the arXiv corpus lay out.
func TestCorpusCensusSymbols(t *testing.T) {
	mathLayoutAll(t, []string{
		`x^\intercal`, `A^\intercal B`, `a \dotplus b`,
		`a \Coloneqq b`, `a \Coloneq b`,
		`\mathbbm{1}`, `\mathbbm{R}`, `\mathbbm{1}_{\{x>0\}}`,
		`\mathbbmss{X}`, `\mathbbb{Z}`,
	})
}

// An arrow over a nucleus spans it: \overrightarrow has a head on the right,
// \overleftarrow on the left, and \overleftrightarrow on both. A head is a
// path item, and the shaft a rule as wide as the nucleus.
func TestOverArrow(t *testing.T) {
	mathLayoutAll(t, []string{
		`\overrightarrow{AB}`, `\overleftarrow{v}`, `\overleftrightarrow{PQ}`,
		`\vv{F} = m\vv{a}`, `\overrightarrow{OM} \cdot \overrightarrow{ON}`,
	})
	for _, c := range []struct {
		tex   string
		heads int
	}{
		{`\overrightarrow{AB}`, 1}, {`\overleftarrow{AB}`, 1}, {`\overleftrightarrow{AB}`, 2},
	} {
		b := mustLayout(t, c.tex, 32, Text)
		base := mustLayout(t, `AB`, 32, Text)
		if got := len(itemsOf(b, ItemPath)); got != c.heads {
			t.Errorf("%s: %d heads, want %d", c.tex, got, c.heads)
		}
		rules := itemsOf(b, ItemRule)
		if len(rules) != 1 || rules[0].X != 0 || rules[0].W != base.Width {
			t.Errorf("%s: the shaft is %+v, want a rule as wide as AB, %v", c.tex, rules, base.Width)
		}
	}
}

// \middle sets a delimiter inside \left and \right, of a fixed size of 1.8 em,
// as the source does.
func TestMiddle(t *testing.T) {
	mathLayoutAll(t, []string{
		`\left\{ x \middle| P(x) \right\}`,
		`\left( a \middle/ b \right)`,
		`\left. \frac{a}{b} \middle| c \right.`,
	})
}

// \binom, the \big family, \overset, \underset and \stackrel lay out.
func TestBinomBigOverset(t *testing.T) {
	mathLayoutAll(t, []string{
		`\binom{n}{k}`, `\dbinom{n}{2}`, `\tbinom{a}{b}`,
		`\bigl( x \bigr)`, `\Big[ y \Big]`, `\bigg\{ z \bigg\}`, `\Bigg| w \Bigg|`,
		`\big| a \big|`, `\bigm| b`,
		`\overset{a}{b}`, `\underset{c}{d}`, `x \stackrel{f}{=} y`,
	})
	// \binom draws no rule, unlike \frac.
	if n := len(itemsOf(mustLayout(t, `\binom{n}{k}`, 32, Text), ItemRule)); n != 0 {
		t.Errorf(`\binom draws %d rules, want none`, n)
	}
	// \bigl( is taller than a plain (.
	big := mustLayout(t, `\bigl(`, 40, Text)
	plain := mustLayout(t, `(`, 40, Text)
	if big.Height+big.Depth <= plain.Height+plain.Depth {
		t.Errorf(`\bigl( (%v) is not taller than ( (%v)`, big.Height+big.Depth, plain.Height+plain.Depth)
	}
}

// \not sets a slash over the next atom, and keeps its width. \substack stacks
// its lines in script style.
func TestNotAndSubstack(t *testing.T) {
	mathLayoutAll(t, []string{
		`a \not= b`, `A \not\subset B`, `x \not\in S`,
		`\substack{i<n \\ i \ne j}`, `\sum_{\substack{a \\ b \\ c}} x`,
	})
	plain := mustLayout(t, `=`, 40, Text)
	neg := mustLayout(t, `\not=`, 40, Text)
	if pn, nn := len(itemsOf(plain, ItemGlyph)), len(itemsOf(neg, ItemGlyph)); nn != pn+1 {
		t.Errorf(`\not= draws %d glyphs, = draws %d: the slash is missing`, nn, pn)
	}
	if neg.Width != plain.Width {
		t.Errorf(`\not= is %v wide, = is %v: the slash does not change the width`, neg.Width, plain.Width)
	}
}

// The commands of the census lay out. \boxed draws a frame of four rules,
// and \phantom draws nothing and keeps the width of its body.
func TestCensusBatch(t *testing.T) {
	mathLayoutAll(t, []string{
		`\ensuremath{x+1}`, `\mathscr{ABH}`, `\mathbin{\star}`, `\mathrel{R}`,
		`\mathord{!}`, `\mathopen{[}x\mathclose{]}`,
		`\hbox{note}`, `\mbox{if } x>0`, `\boxed{E=mc^2}`,
		`\underbrace{a+b}_{s}`, `\overbrace{x+y}^{t}`,
		`\phantom{xy}z`, `\hphantom{x}y`, `\vphantom{X}y`,
		`A\vDash B`, `p\land q\lor r`, `\llbracket x\rrbracket`, `a\rtimes b`,
		`x\hdots y`, `\sum\limits_{i}a_i`, `\therefore x`, `a\boxplus b`, `\bigstar`,
	})
	if n := len(itemsOf(mustLayout(t, `\boxed{x}`, 32, Text), ItemRule)); n != 4 {
		t.Errorf(`\boxed{x} draws %d rules, want the 4 sides of a frame`, n)
	}
	if n := len(mustLayout(t, `\phantom{x}`, 32, Text).Items); n != 0 {
		t.Errorf(`\phantom{x} draws %d items, want none`, n)
	}
	ph := mustLayout(t, `\phantom{MMM}`, 40, Text)
	if want := mustLayout(t, `MMM`, 40, Text).Width; ph.Width != want {
		t.Errorf(`\phantom{MMM} is %v wide, want %v`, ph.Width, want)
	}
}

// The notation of braket and physics puts its argument between delimiters.
func TestBraketNotation(t *testing.T) {
	mathLayoutAll(t, []string{
		`\ket{\psi}`, `\bra{\phi}`, `\braket{\phi|\psi}`, `\ketbra{\psi}{\phi}`,
		`\norm{x}`, `\abs{-y}`, `\Set{x}`, `\ket{0}+\ket{1}`,
	})
	k := len(itemsOf(mustLayout(t, `\ket{x}`, 32, Text), ItemGlyph))
	x := len(itemsOf(mustLayout(t, `x`, 32, Text), ItemGlyph))
	if k != x+2 {
		t.Errorf(`\ket{x} draws %d glyphs and x %d: the two delimiters are missing`, k, x)
	}
}

func TestStructure(t *testing.T) {
	for _, tex := range []string{`\frac{a}{b}`, `\sqrt{x}`, `\overline{x}`, `\underline{x}`} {
		if len(itemsOf(mustLayout(t, tex, 32, Text), ItemRule)) == 0 {
			t.Errorf("%s draws no rule", tex)
		}
	}
	// The delimiters of pmatrix are glyphs beside the four cells.
	if n := len(itemsOf(mustLayout(t, `\begin{pmatrix} a & b \\ c & d \end{pmatrix}`, 32, Text), ItemGlyph)); n < 6 {
		t.Errorf("pmatrix draws %d glyphs", n)
	}
}

// A script or a prime with no nucleus attaches to an empty nucleus, as the
// mark $^1$ of a footnote.
func TestEmptyBaseScriptsLayOut(t *testing.T) {
	for _, tex := range []string{`^1`, `_2`, `^{1}`, `_{ab}`, `'`, `^a_b`, `x + ^1`} {
		mathLayoutOK(t, tex)
	}
	// okage: a subscript whose top is over the baseline counts in the
	// height. The source gave _2 no height.
	if b := mustLayout(t, `_2`, 32, Text); !(b.Height > 0) {
		t.Errorf("_2 has the height %v, want the part of the 2 over the baseline", b.Height)
	}
}

func TestClampsAndDelimiters(t *testing.T) {
	for _, tex := range []string{
		`x^{aaaaaa}`, `x_{bbbbbb}`, `I^{WWWW}_{MMMM}`,
		`\sum_{kkkkkk}^{j} a`, `\prod^{mmmmmm} b`,
		`\left\{ x \right\}`, `\left. x \right.`, `\left\lfloor x \right\rfloor`,
		`\begin{matrix} a \\ b & c \end{matrix}`,
		`x'^{2}`, `f''_{n}`, `\bigcup_{i} A_i`, `-x`, `= + a`,
	} {
		mathLayoutOK(t, tex)
	}
	// A size of 1. The source tests the floor of a whole pixel of its script
	// sizes. The sizes here are exact, so a small size is a small box.
	for _, tex := range []string{`x^2_3`, `\sqrt[3]{x}`} {
		b := mustLayout(t, tex, 1, Text)
		checkBox(t, tex, b)
		if !(b.Width > 0) {
			t.Errorf("%s at the size 1 is %v wide", tex, b.Width)
		}
	}
}

// An empty group draws nothing, and a character without a glyph gives an
// empty box.
func TestAnEmptyGroupDrawsNothing(t *testing.T) {
	if b := mustLayout(t, `{}`, 32, Text); len(b.Items) != 0 || b.Width != 0 {
		t.Errorf("{} = %+v, want an empty box", b)
	}
	face := testFace(t)
	l := &layout{face: face, size: 32, upem: float64(face.Upem()), cfg: newConfig(nil), color: okage.ColorBlack}
	if _, ok := l.glyphBox(0x10FFFF, 32, Ord); ok {
		t.Error("a character without a glyph reported a glyph")
	}
	if b := l.mustGlyph(0x10FFFF, 32, Ord); b.w != 0 || len(b.items) != 0 {
		t.Errorf("mustGlyph of a character without a glyph = %+v", b)
	}
}

// The metrics of a box are those of its ink: the outline of Box.Path reaches
// the height above the baseline and the depth below it. The source checks
// the viewBox of its tight SVG text.
func TestRenderMetrics(t *testing.T) {
	for _, st := range []Style{Text, Display} {
		b := mustLayout(t, `x+1`, 40, st)
		if b.Width <= 0 || b.Height <= 0 || b.Depth < 0 {
			t.Fatalf("x+1 in %v = %+v, want a positive width and height", st, b)
		}
		r, ok := b.Path().ComputeTightBounds()
		if !ok {
			t.Fatal("x+1 has no outline")
		}
		if math.Abs(float64(r.Top())+b.Height) > 1e-3 || math.Abs(float64(r.Bottom())-b.Depth) > 1e-3 {
			t.Errorf("x+1 in %v: the outline goes from %v to %v, want %v to %v", st, r.Top(), r.Bottom(), -b.Height, b.Depth)
		}
	}
	// A descender gives a depth, and an ascender more height than depth.
	if b := mustLayout(t, `y`, 40, Text); b.Depth <= 0 {
		t.Errorf("y has the depth %v, want more than 0", b.Depth)
	}
	if b := mustLayout(t, `l`, 40, Text); b.Height <= b.Depth {
		t.Errorf("l has the height %v and the depth %v", b.Height, b.Depth)
	}
}

// An accent sits a small gap over its nucleus: the bottom of its ink is
// 0.04 em over the top of the nucleus. The source used the depth of the
// accent, 0 for a glyph over its baseline, so the accent floated a whole
// accent high.
//
// The source checks that the box fits on a line of text at 11. Here the
// height is exact: the height of the nucleus, the gap, and the ink of the
// accent from its bottom to its top, from the font.
func TestAccentSitsJustAboveTheNucleus(t *testing.T) {
	const size = 11
	face := testFace(t)
	_, hx, _ := glyphMetrics(t, face, mathItalic('x'), size)
	base := mustLayout(t, `x`, size, Text)
	for _, c := range []struct {
		cmd string
		acc rune
	}{
		{`\hat`, 'ˆ'}, {`\bar`, '¯'}, {`\dot`, '˙'}, {`\tilde`, '˜'}, {`\vec`, '⃗'}, {`\check`, 'ˇ'},
	} {
		m := mustLayout(t, c.cmd+`{x}`, size, Text)
		_, ha, _ := glyphMetrics(t, face, c.acc, size)
		e, _ := face.GlyphExtents(glyphOf(t, face, c.acc))
		inkBot := float64((float64(e.YBearing) + float64(e.Height)) * (size / float64(face.Upem())))
		want := -(-(hx + float64(size*0.04)) + inkBot) + ha
		if !near(m.Height, want) {
			t.Errorf("%s{x}: the height is %v, want %v", c.cmd, m.Height, want)
		}
		if m.Height <= base.Height {
			t.Errorf("%s{x}: the height %v is not more than that of x, %v", c.cmd, m.Height, base.Height)
		}
		if m.Height+m.Depth > size {
			t.Errorf("%s{x}: a box of %v does not fit on a line of text", c.cmd, m.Height+m.Depth)
		}
	}
}

// The ink of a glyph is the extent of its outline: an accent does not touch
// the baseline. The source read the points of go-opentype that a segment
// does not use.
func TestGlyphInkIgnoresUnusedSegmentPoints(t *testing.T) {
	face := testFace(t)
	l := &layout{face: face, size: 11, upem: float64(face.Upem()), cfg: newConfig(nil), color: okage.ColorBlack}
	top, bot := l.gidInk(glyphOf(t, face, 'ˆ'), 11)
	if bot <= 0 {
		t.Errorf("the ink of the circumflex goes from %v to %v: an accent does not touch the baseline", bot, top)
	}
	if top-bot > 4 {
		t.Errorf("the circumflex is %v high: too thick for an accent", top-bot)
	}
}

// \left( x \right) has the size of ( x ): the smallest delimiter that covers
// the content is the glyph itself, for content no taller than a letter. The
// source allows 0.1pt for its rounding. Here the two are exact: the pair is
// moved onto the axis, and its height and depth add up to those of (x).
func TestLeftRightMatchesPlainDelimiters(t *testing.T) {
	plain := mustLayout(t, `(x)`, 11, Text)
	big := mustLayout(t, `\left(x\right)`, 11, Text)
	if !near(big.Height+big.Depth, plain.Height+plain.Depth) {
		t.Errorf(`the sizes are %v (\left) and %v: a delimiter around a letter does not grow`,
			big.Height+big.Depth, plain.Height+plain.Depth)
	}
	tall := mustLayout(t, `\left(\frac{a}{b}\right)`, 11, Text)
	if tall.Height+tall.Depth <= plain.Height+plain.Depth {
		t.Errorf("a delimiter around a fraction (%v) is not larger than the plain one (%v)",
			tall.Height+tall.Depth, plain.Height+plain.Depth)
	}
}

// A superscript is raised by SuperscriptShiftUp, or more when its bottom
// would come within SuperscriptBottomMin of the baseline. Over a nucleus that
// is a box, its baseline drops at most SuperscriptBaselineDropMax from the
// top of the nucleus.
//
// The source checks that a few formulas stay under fixed heights at 11pt.
// okage takes all the constants of Appendix G, rule 18, and sets one glyph
// of a character without the drop, so here the height, the depth and the
// width are computed again from the font. SpaceAfterScript follows the
// script.
func TestSuperscriptBaselineDrops(t *testing.T) {
	const size = 11
	face := testFace(t)
	ss := scriptSize(face, size)
	w2, h2, d2 := glyphMetrics(t, face, '2', ss)
	space := mathConst(face, font.SpaceAfterScript, size)
	for _, c := range []struct {
		tex     string
		nucleus rune
		char    bool // the nucleus is one glyph of a character
	}{
		{`x^2`, 'x', true},
		{`f^2`, 'f', true},
		{`{x}^2`, 'x', false},
		{`{f}^2`, 'f', false},
	} {
		wn, hn, dn := glyphMetrics(t, face, mathItalic(c.nucleus), size)
		// The superscript follows the italic correction of the nucleus, and
		// so does the end of the list in {x}: phase 26 of D78.
		wn += italicOf(t, face, mathItalic(c.nucleus), size)
		u := max(mathConst(face, font.SuperscriptShiftUp, size), d2+mathConst(face, font.SuperscriptBottomMin, size))
		if !c.char {
			u = max(mathConst(face, font.SuperscriptShiftUp, size), hn-mathConst(face, font.SuperscriptBaselineDropMax, size),
				d2+mathConst(face, font.SuperscriptBottomMin, size))
		}
		b := mustLayout(t, c.tex, size, Text)
		if want := max(hn, u+h2); !near(b.Height, want) {
			t.Errorf("%s: the height is %v, want %v", c.tex, b.Height, want)
		}
		if want := max(dn, d2-u); !near(b.Depth, want) {
			t.Errorf("%s: the depth is %v, want %v", c.tex, b.Depth, want)
		}
		if want := max(wn, wn+w2+space); !near(b.Width, want) {
			t.Errorf("%s: the width is %v, want %v", c.tex, b.Width, want)
		}
	}
	// The drop makes f^2 higher than {f}^2 is not: the tall f of a box
	// lifts its superscript, and the glyph f does not.
	if a, b := mustLayout(t, `f^2`, size, Text), mustLayout(t, `{f}^2`, size, Text); a.Height >= b.Height {
		t.Errorf("f^2 is %v high and {f}^2 %v: one glyph takes no drop", a.Height, b.Height)
	}
	// The superscript is raised: it is not dropped to the baseline.
	if sup, base := mustLayout(t, `x^2`, size, Text), mustLayout(t, `x`, size, Text); sup.Height <= base.Height {
		t.Errorf("x^2 is %v high and x %v: a superscript goes over its base", sup.Height, base.Height)
	}
}
