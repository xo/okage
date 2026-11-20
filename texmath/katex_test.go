package texmath

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/typesetting/font"
)

// This file tests the commands of KaTeX that phase 26 of D78 adds, in
// katex.go, parse.go and macro.go.

// TestCancel checks \cancel, \bcancel and \xcancel: the lines and the padding
// of KaTeX.
func TestCancel(t *testing.T) {
	const size = 10
	for _, c := range []struct {
		tex        string
		lines      int
		padX, padY float64
	}{
		{`\cancel{5}`, 1, 0, 0},
		{`\bcancel{x}`, 1, 0, 0},
		{`\xcancel{5}`, 2, 0, 0},
		{`\cancel{x+y}`, 1, 2, 0},
		{`\xcancel{abc}`, 2, 2, 0},
	} {
		body := c.tex[strings.Index(c.tex, "{")+1 : len(c.tex)-1]
		base := mustLayout(t, body, size, Text)
		b := mustLayout(t, c.tex, size, Text)
		if n := len(itemsOf(b, ItemPath)); n != c.lines {
			t.Errorf("%s draws %d lines, want %d", c.tex, n, c.lines)
		}
		if !near(b.Width, base.Width+2*c.padX) || !near(b.Height, base.Height+c.padY) || !near(b.Depth, base.Depth+c.padY) {
			t.Errorf("%s is %v by %v by %v, want the body %v by %v by %v padded by %v and %v",
				c.tex, b.Width, b.Height, b.Depth, base.Width, base.Height, base.Depth, c.padX, c.padY)
		}
	}
	// The line of \cancel goes up to the right, as a slash.
	p := itemsOf(mustLayout(t, `\cancel{x+y}`, size, Text), ItemPath)[0].Path.Bounds()
	if p.Width() <= 0 || p.Height() <= 0 {
		t.Errorf(`the line of \cancel has the bounds %v`, p)
	}
	// Over one character, the line goes 0.2 em past its top and its bottom.
	five := mustLayout(t, `\cancel{5}`, size, Text)
	if p := itemsOf(five, ItemPath)[0].Path.Bounds(); float64(p.Height()) < five.Height+five.Depth+2*2-0.5 {
		t.Errorf(`the line of \cancel{5} is %v high, the box %v`, p.Height(), five.Height+five.Depth)
	}
}

// TestPhase checks \phase: the tree, and the angle of KaTeX around the body.
func TestPhase(t *testing.T) {
	ph := as[Phase](t, mustParse(t, `\phase{x}`)[0])
	if body := as[List](t, ph.Body); len(body) != 1 || as[Atom](t, body[0]).Text != "x" {
		t.Errorf(`the body of \phase{x} is %#v`, ph.Body)
	}
	if l := mustParse(t, `\phase{x+y}z`); len(l) != 2 || len(as[List](t, as[Phase](t, l[0]).Body)) != 3 {
		t.Errorf(`\phase{x+y}z does not take its group as its body: %#v`, l)
	}
	if _, err := Parse(`\phase`); !errors.Is(err, okage.ErrTeXMathSyntax) {
		t.Errorf(`\phase with no argument: %v`, err)
	}
	const size = 10
	face := testFace(t)
	for _, c := range []struct {
		pre, body string
		st        Style
		factor    float64 // the factor of the size of pre
	}{
		{``, `x`, Display, 1},
		{``, `x`, Text, 1},
		{``, `\frac{a}{b}`, Display, 1},
		{`\Large`, `x`, Display, 1.44},
		{`\scriptstyle`, `x`, Text, 1},
	} {
		tex := c.pre + `\phase{` + c.body + `}`
		base := mustLayout(t, c.pre+`{`+c.body+`}`, size, c.st)
		b := mustLayout(t, tex, size, c.st)
		// The line is 0.6 pt under any size command, and the space is 0.35
		// ex of text style at the size in force: the height of the upright
		// x of the face.
		lw := 0.06 * size
		_, ex, _ := glyphMetrics(t, face, 'x', c.factor*size)
		clear := 0.35 * ex
		ah := base.Height + base.Depth + lw + clear
		if !near(b.Width, base.Width+ah/2+lw) || !near(b.Height, base.Height) || !near(b.Depth, base.Depth+lw+clear) {
			t.Errorf("%s is %v by %v by %v, want %v by %v by %v",
				tex, b.Width, b.Height, b.Depth, base.Width+ah/2+lw, base.Height, base.Depth+lw+clear)
		}
		paths := itemsOf(b, ItemPath)
		if len(paths) != 1 {
			t.Fatalf("%s draws %d paths, want 1", tex, len(paths))
		}
		// The angle fills the box from its left end to its right end, and
		// from the top of the body to the bottom of the box.
		r := paths[0].Path.Bounds()
		if !near32(r.Left(), 0) || !near32(r.Right(), b.Width) || !near32(r.Top(), -b.Height) || !near32(r.Bottom(), b.Depth) {
			t.Errorf("the angle of %s has the bounds %v, the box is %v by %v by %v", tex, r, b.Width, b.Height, b.Depth)
		}
	}
	// The box is an Ord, so a binary operator after it keeps its class.
	if a, b := mustLayout(t, `\phase{x}+y`, size, Text), mustLayout(t, `\phase{x}y`, size, Text); a.Width <= b.Width {
		t.Errorf(`\phase{x}+y is %v wide, and \phase{x}y %v`, a.Width, b.Width)
	}
}

// near32 reports whether the float32 a is b within the rounding of a float32.
func near32(a float32, b float64) bool {
	return math.Abs(float64(a)-b) <= 1e-5*max(1, math.Abs(b))
}

// TestTag checks \tag and \tag*: the label is a quad after the formula, in
// parentheses for \tag.
func TestTag(t *testing.T) {
	const size = 10
	face := testFace(t)
	body := mustLayout(t, `E=mc^2`, size, Display)
	label := mustLayout(t, `\text{(1)}`, size, Text)
	b := mustLayout(t, `E=mc^2 \tag{1}`, size, Display)
	if !near(b.Width, body.Width+size+label.Width) {
		t.Errorf(`\tag: the width is %v, want %v`, b.Width, body.Width+size+label.Width)
	}
	open := glyphItem(t, b, glyphOf(t, face, '('))
	if !near(open.X, body.Width+size) {
		t.Errorf(`\tag: the label starts at %v, want %v`, open.X, body.Width+size)
	}
	star := mustLayout(t, `a \tag*{x} + b`, size, Display)
	if want := mustLayout(t, `a+b`, size, Display).Width + size + mustLayout(t, `\text{x}`, size, Text).Width; !near(star.Width, want) {
		t.Errorf(`\tag*: the width is %v, want %v`, star.Width, want)
	}
	tree, err := Parse(`a \tag{1} b`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree[0].(Tag); len(tree) != 1 || !ok {
		t.Errorf(`Parse(a \tag{1} b) gives %#v, want one Tag`, tree)
	}
	for _, tex := range []string{`{a \tag{1}}`, `a \tag{1} \tag{2}`, `\frac{\tag{1}}{2}`, `\tag`} {
		if _, err := Parse(tex); !errors.Is(err, okage.ErrTeXMathSyntax) {
			t.Errorf("Parse(%q): %v, want a syntax error", tex, err)
		}
	}
	if _, err := Parse(`a \notag \nonumber`); err != nil {
		t.Errorf(`\notag and \nonumber: %v`, err)
	}
}

// TestHline checks \hline in array and in a matrix, and that a \\ before
// \end starts no row.
func TestHline(t *testing.T) {
	const size = 10
	plain := mustLayout(t, `\begin{array}{cc} a & b \\ c & d \end{array}`, size, Text)
	ruled := mustLayout(t, `\begin{array}{cc} \hline a & b \\ \hline c & d \\ \hline \end{array}`, size, Text)
	rules := itemsOf(ruled, ItemRule)
	if len(rules) != 3 {
		t.Fatalf("%d rules, want 3", len(rules))
	}
	thick := rules[0].H
	if want := plain.Height + plain.Depth + 3*thick; !near(ruled.Height+ruled.Depth, want) {
		t.Errorf("the ruled array is %v high, want %v", ruled.Height+ruled.Depth, want)
	}
	if !near(rules[0].Y, -ruled.Height) || !near(rules[2].Y+thick, ruled.Depth) || !near(rules[0].W, ruled.Width) {
		t.Errorf("the rules are %+v in a box of %v by %v by %v", rules, ruled.Width, ruled.Height, ruled.Depth)
	}
	if n := len(itemsOf(mustLayout(t, `\begin{pmatrix} a \\ \hline b \end{pmatrix}`, size, Text), ItemRule)); n != 1 {
		t.Errorf("pmatrix with \\hline has %d rules, want 1", n)
	}
	// A \\ before \end adds no row.
	a, b := mustLayout(t, `\begin{matrix} a \\ b \end{matrix}`, size, Text), mustLayout(t, `\begin{matrix} a \\ b \\ \end{matrix}`, size, Text)
	if a.Height != b.Height || a.Depth != b.Depth {
		t.Errorf(`a \\ b \\ is %v by %v, a \\ b %v by %v`, b.Height, b.Depth, a.Height, a.Depth)
	}
	tree, err := Parse(`\begin{matrix} \\ \end{matrix}`)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := tree[0].(Array); !ok || len(a.Rows) != 1 {
		t.Errorf(`an empty row: %#v, want one row`, tree[0])
	}
}

// TestCfrac checks that \cfrac sets its parts in display style, with a strut
// in the numerator.
func TestCfrac(t *testing.T) {
	const size = 10
	c := mustLayout(t, `\cfrac{1}{x}`, size, Text)
	d := mustLayout(t, `\dfrac{1}{\displaystyle x}`, size, Text)
	if !near(c.Width, d.Width) || !(c.Height > d.Height) {
		t.Errorf(`\cfrac{1}{x} is %v by %v, \dfrac{1}{x} %v by %v`, c.Width, c.Height, d.Width, d.Height)
	}
	if _, err := Parse(`\cfrac[l]{1}{x}`); err != nil {
		t.Errorf(`\cfrac[l]: %v`, err)
	}
}

// TestKernAndHspace checks the lengths of \kern and \hspace.
func TestKernAndHspace(t *testing.T) {
	const size = 10
	ab := mustLayout(t, `ab`, size, Text).Width
	for _, c := range []struct {
		tex  string
		want float64
	}{
		{`a\kern1em b`, 10},
		{`a\kern 1em b`, 10},
		{`a\kern-2pt b`, -2},
		{`a\kern{3mu}b`, 3 * size / 18.0},
		{`a\kern18mu b`, 10},
		{`a\hspace{1em}b`, 10},
		{`a\hspace*{0.5em}b`, 5},
		{`a\hspace{1in}b`, 72.27},
	} {
		if got := mustLayout(t, c.tex, size, Text).Width - ab; !near(got, c.want) {
			t.Errorf("%s adds %v, want %v", c.tex, got, c.want)
		}
	}
	for _, tex := range []string{`\kern`, `\kern1 b`, `\hspace{x}`, `\hspace1em`, `\kern99999em`} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("Parse(%q) gives no error", tex)
		}
	}
}

// TestLaps checks \llap, \rlap and \clap and their math forms.
func TestLaps(t *testing.T) {
	const size = 10
	face := testFace(t)
	x := glyphOf(t, face, 'x')
	wx, _, _ := gidMetrics(face, x, size)
	for _, c := range []struct {
		tex string
		at  float64
	}{
		{`\rlap{x}`, 0},
		{`\llap{x}`, -wx},
		{`\clap{x}`, -wx / 2},
	} {
		b := mustLayout(t, c.tex, size, Text)
		if b.Width != 0 || !near(glyphItem(t, b, x).X, c.at) {
			t.Errorf("%s: the width %v and the x at %v, want 0 and %v", c.tex, b.Width, b.Items[0].X, c.at)
		}
	}
	// The math forms take math: the x is italic.
	b := mustLayout(t, `\mathclap{x}`, size, Text)
	if b.Width != 0 || b.Items[0].Glyph != glyphOf(t, face, mathItalic('x')) {
		t.Errorf(`\mathclap{x}: %+v`, b)
	}
}

// TestNewcommand checks \newcommand and its kin.
func TestNewcommand(t *testing.T) {
	for _, c := range [][2]string{
		{`\newcommand{\pair}[2]{(#1, #2)} \pair{a}{b}`, `(a, b)`},
		{`\newcommand\half{\frac12} \half`, `\frac12`},
		{`\newcommand*{\x}{y}\x`, `y`},
		{`\newcommand{\x}{y}\renewcommand{\x}{z}\x`, `z`},
		{`\newcommand{\x}{y}\providecommand{\x}{z}\x`, `y`},
		{`\providecommand{\x}{z}\x`, `z`},
	} {
		got, err := Parse(c[0])
		if err != nil {
			t.Errorf("Parse(%q): %v", c[0], err)
			continue
		}
		want, err := Parse(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%q) is not Parse(%q)", c[0], c[1])
		}
	}
	// A default value of the first parameter is not read.
	var se *SyntaxError
	if _, err := Parse(`\newcommand{\x}[1][a]{#1}`); !errors.As(err, &se) || se.Command != `\newcommand` {
		t.Errorf(`\newcommand with a default: %v`, err)
	}
}

// TestKaTeXMacros checks that each macro of KaTeX expands to TeX that Parse
// reads, that none of them hides a command of this package, and that a
// formula can define the name again.
func TestKaTeXMacros(t *testing.T) {
	for name, body := range katexMacros {
		if _, ok := commands[name]; ok {
			t.Errorf(`\%s is a command and a macro of KaTeX`, name)
		}
		if _, ok := symbols[name]; ok {
			t.Errorf(`\%s is a symbol and a macro of KaTeX`, name)
		}
		if _, ok := namedOps[name]; ok {
			t.Errorf(`\%s is an operator and a macro of KaTeX`, name)
		}
		tex := `\` + name
		if strings.HasPrefix(body, `\math`) || strings.HasPrefix(body, `\bold`) || strings.HasSuffix(body, `\underline`) || strings.HasSuffix(body, `*`) {
			tex += `{x}`
		}
		got, err := Parse(tex)
		if err != nil {
			t.Errorf("Parse(%q): %v", tex, err)
			continue
		}
		want, err := Parse(body + strings.TrimPrefix(tex, `\`+name))
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("Parse(%q) is not Parse(%q): %v", tex, body, err)
		}
	}
	got, err := Parse(`\def\R{r}\R`)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := Parse(`r`)
	if !reflect.DeepEqual(got, want) {
		t.Errorf(`\def\R{r}\R is not r`)
	}
}

// TestMiddle checks that \middle grows as its \left and \right do, and keeps
// its fixed size outside them.
func TestMiddleGrows(t *testing.T) {
	const size = 10
	face := testFace(t)
	bar := glyphOf(t, face, '|')
	paren := glyphOf(t, face, '(')
	// glyphs returns the glyph of r and the glyphs of its variants and of
	// its assembly.
	glyphs := func(g font.GID) map[font.GID]bool {
		set := map[font.GID]bool{g: true}
		for _, v := range face.MathGlyphVariants(g, true) {
			set[v.Glyph] = true
		}
		if asm, ok := face.MathGlyphAssembly(g, true); ok {
			for _, p := range asm.Parts {
				set[p.Glyph] = true
			}
		}
		return set
	}
	extent := func(b *Box, g font.GID) float64 {
		set := glyphs(g)
		top, bot := math.Inf(1), math.Inf(-1)
		for _, it := range b.Items {
			if it.Kind != ItemGlyph || !set[it.Glyph] {
				continue
			}
			e, _ := face.GlyphExtents(it.Glyph)
			s := it.Size / float64(face.Upem())
			top = min(top, it.Y-float64(e.YBearing)*s)
			bot = max(bot, it.Y-(float64(e.YBearing)+float64(e.Height))*s)
		}
		return bot - top
	}
	for _, tex := range []string{
		`\left( \frac{a}{b} \middle| c \right)`,
		`\left( \begin{matrix} a \\ b \\ c \\ d \end{matrix} \middle| c \right)`,
		`\left( \displaystyle \frac{a}{b} \middle| c \right)`,
	} {
		b := mustLayout(t, tex, size, Display)
		if m, p := extent(b, bar), extent(b, paren); math.Abs(m-p) > 0.25*p {
			t.Errorf("%s: the bar is %v high and the parenthesis %v", tex, m, p)
		}
	}
	// A nest of fences with \middle lays out in a short time.
	deep := strings.Repeat(`\left( a \middle| `, 30) + "x" + strings.Repeat(` \right)`, 30)
	if _, err := LayoutTeX(deep, face, size, Display); err != nil {
		t.Error(err)
	}
	// Outside a fence, \middle keeps its size of 1.8 em.
	if a, b := mustLayout(t, `\middle|`, size, Text), mustLayout(t, `\Bigm|`, size, Text); a.Height != b.Height || a.Depth != b.Depth {
		t.Errorf("\\middle| alone is %v by %v, \\Bigm| %v by %v", a.Height, a.Depth, b.Height, b.Depth)
	}
}

// TestSmash checks \smash and its options.
func TestSmash(t *testing.T) {
	const size = 10
	f := mustLayout(t, `\frac{a}{b}`, size, Display)
	for _, c := range []struct {
		tex  string
		h, d float64
	}{
		{`\smash{\frac{a}{b}}`, 0, 0},
		{`\smash[t]{\frac{a}{b}}`, 0, f.Depth},
		{`\smash[b]{\frac{a}{b}}`, f.Height, 0},
	} {
		b := mustLayout(t, c.tex, size, Display)
		if b.Height != c.h || b.Depth != c.d || !near(b.Width, f.Width) || len(b.Items) != len(f.Items) {
			t.Errorf("%s is %v by %v by %v, want %v by %v by %v", c.tex, b.Width, b.Height, b.Depth, f.Width, c.h, c.d)
		}
	}
}

// TestSizeCommands checks that a size command sets the rest of its group at
// its factor of the size of the layout, and that the factors do not
// multiply.
func TestSizeCommands(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		tex    string
		factor float64
	}{
		{`\tiny x`, 0.5}, {`\scriptsize x`, 0.7}, {`\footnotesize x`, 0.8}, {`\small x`, 0.9},
		{`\normalsize x`, 1}, {`\large x`, 1.2}, {`\Large x`, 1.44}, {`\LARGE x`, 1.728},
		{`\huge x`, 2.074}, {`\Huge x`, 2.488}, {`\Huge\large x`, 1.2}, {`\text{\large x}`, 1.2},
	} {
		b := mustLayout(t, c.tex, 10, Text)
		if got := b.Items[0].Size; !near(got, 10*c.factor) {
			t.Errorf("%s sets x at %v, want %v", c.tex, got, 10*c.factor)
		}
	}
	// The group ends the size.
	b := mustLayout(t, `{\large x} x`, 10, Text)
	if b.Items[0].Size != 12 || b.Items[1].Size != 10 {
		t.Errorf("{\\large x} x sets the sizes %v and %v", b.Items[0].Size, b.Items[1].Size)
	}
	// A factor that is not a number, from a tree of a program, is the
	// smallest factor.
	if b, err := Layout(SizeChange{Factor: math.NaN(), Body: Atom{Text: "x"}}, face, 10, Text); err != nil || !(b.Width > 0) {
		t.Errorf("a SizeChange of NaN: %+v, %v", b, err)
	}
	// A script of a sized formula is at the script size of that size.
	b = mustLayout(t, `\large x^2`, 10, Text)
	if got, want := b.Items[1].Size, scriptSize(face, 12); !near(got, want) {
		t.Errorf("the superscript of \\large x^2 is at %v, want %v", got, want)
	}
}
