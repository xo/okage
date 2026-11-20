package texmath

import (
	"errors"
	"log/slog"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
)

// This file holds the helpers of the tests of the layout, the gate of phase
// 24 of D78 and the fuzz target FuzzLayout.
//
// The tests of the source compare metrics in whole pixels, which go-opentype
// rounds. The tests of okage compare the exact metrics: each expected value is
// computed again from the font, with the rules of the layout, and a test says
// so in a comment (D78 section 7, item 1).

// testFace returns a face of STIX Two Math. Each test takes its own face,
// because a face is not safe for concurrent use.
func testFace(tb testing.TB) *font.Face {
	tb.Helper()
	f, err := stixtwomath.Font()
	if err != nil {
		tb.Fatal(err)
	}
	return font.NewFace(f)
}

// mustLayout lays out tex with STIX Two Math at size in style st.
func mustLayout(tb testing.TB, tex string, size float64, st Style) *Box {
	tb.Helper()
	b, err := LayoutTeX(tex, testFace(tb), size, st)
	if err != nil {
		tb.Fatalf("LayoutTeX(%q): %v", tex, err)
	}
	return b
}

// glyphOf returns the glyph of r in face, and fails when there is none.
func glyphOf(tb testing.TB, face *font.Face, r rune) font.GID {
	tb.Helper()
	g, ok := face.NominalGlyph(r)
	if !ok {
		tb.Fatalf("the face has no glyph for %q", r)
	}
	return g
}

// glyphMetrics returns the advance, the height and the depth of the glyph of
// r at size, as the layout computes them: the advance, and the ink of the
// extents of typesetting, clamped at the baseline.
func glyphMetrics(tb testing.TB, face *font.Face, r rune, size float64) (w, h, d float64) {
	tb.Helper()
	return gidMetrics(face, glyphOf(tb, face, r), size)
}

// gidMetrics returns the advance, the height and the depth of the glyph g at
// size.
func gidMetrics(face *font.Face, g font.GID, size float64) (w, h, d float64) {
	s := size / float64(face.Upem())
	w = float64(float64(face.HorizontalAdvance(g)) * s)
	e, _ := face.GlyphExtents(g)
	top := float64(float64(e.YBearing) * s)
	bot := float64((float64(e.YBearing) + float64(e.Height)) * s)
	return w, max(top, 0), -min(bot, 0)
}

// italicOf returns the italic correction of the glyph of r at size, which
// the layout puts after the glyph in a list (phase 26 of D78).
func italicOf(tb testing.TB, face *font.Face, r rune, size float64) float64 {
	tb.Helper()
	return gidItalic(face, glyphOf(tb, face, r), size)
}

// gidItalic returns the italic correction of the glyph g at size.
func gidItalic(face *font.Face, g font.GID, size float64) float64 {
	return float64(float64(face.MathItalicsCorrection(g)) * (size / float64(face.Upem())))
}

// mathConst returns the MATH constant c at size.
func mathConst(face *font.Face, c font.MathConstant, size float64) float64 {
	return float64(float64(face.MathConstant(c)) * (size / float64(face.Upem())))
}

// scriptSize returns the size of script style for the text size size.
func scriptSize(face *font.Face, size float64) float64 {
	return float64(size*float64(face.MathConstant(font.ScriptPercentScaleDown))) / 100
}

// itemsOf returns the items of b of the kind k.
func itemsOf(b *Box, k ItemKind) []Item {
	var out []Item
	for _, it := range b.Items {
		if it.Kind == k {
			out = append(out, it)
		}
	}
	return out
}

// near reports whether a and b are equal within a rounding of the last bits.
func near(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*max(1, math.Abs(a), math.Abs(b))
}

// The four styles, for the tests that lay out in each.
var allStyles = []Style{Display, Text, Script, ScriptScript}

// TestLayoutGate is the gate of phase 24 of D78: each input of the tests of
// the source that parses, lays out in each style, at a size in pixels and at
// a size in points, with no error. testdata/gate.txt holds the inputs, as for
// the gate of phase 23.
func TestLayoutGate(t *testing.T) {
	face := testFace(t)
	n := 0
	for _, in := range gateInputs(t) {
		if in.err != "" {
			continue
		}
		tree, err := Parse(in.tex)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in.tex, err)
		}
		for _, st := range allStyles {
			for _, size := range []float64{32, 14.4} {
				b, err := Layout(tree, face, size, st)
				if err != nil {
					t.Errorf("Layout(%q, %v, %v): %v", in.tex, size, st, err)
					continue
				}
				checkBox(t, in.tex, b)
				n++
			}
		}
	}
	t.Logf("%d layouts", n)
}

// checkBox checks that each metric and each item of b is finite.
func checkBox(t *testing.T, tex string, b *Box) {
	t.Helper()
	finite := func(vs ...float64) bool {
		for _, v := range vs {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return false
			}
		}
		return true
	}
	if !finite(b.Width, b.Height, b.Depth, b.Italic) {
		t.Errorf("%q: the box %v %v %v is not finite", tex, b.Width, b.Height, b.Depth)
	}
	for _, it := range b.Items {
		if !finite(it.X, it.Y, it.W, it.H, it.Size) {
			t.Errorf("%q: the item %+v is not finite", tex, it)
		}
	}
}

func TestLayoutErrors(t *testing.T) {
	face := testFace(t)
	if _, err := LayoutTeX(`x`, nil, 10, Text); !errors.Is(err, okage.ErrMathNoMathTable) {
		t.Errorf("a nil face: %v, want ErrMathNoMathTable", err)
	}
	// A font that okage already holds, with its license, and no MATH table
	// (D78 section 4, item 5).
	f, err := os.Open("../webfont/testdata/cff.otf")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	plain, err := font.ParseTTF(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LayoutTeX(`x`, plain, 10, Text); !errors.Is(err, okage.ErrMathNoMathTable) {
		t.Errorf("a face without a MATH table: %v, want ErrMathNoMathTable", err)
	}
	if got := err2string(nil, okage.ErrMathNoMathTable); got != "laying out math: the face has no MATH table" {
		t.Errorf("the text of the error is %q", got)
	}
	for _, size := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := LayoutTeX(`x`, face, size, Text); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("the size %v: %v, want ErrMathLimit", size, err)
		}
	}
	// An error of the parser comes back as it is.
	_, err = LayoutTeX(`\foo`, face, 10, Text)
	var se *SyntaxError
	if !errors.As(err, &se) || se.Command != `\foo` {
		t.Errorf(`LayoutTeX(\foo): %v, want a *SyntaxError for \foo`, err)
	}
	// A style past ScriptScript is ScriptScript.
	a := mustLayout(t, `x`, 10, ScriptScript)
	b, err := LayoutTeX(`x`, face, 10, ScriptScript+3)
	if err != nil || a.Width != b.Width {
		t.Errorf("the style %d: %v, %v, want %v", ScriptScript+3, b, err, a)
	}
}

// TestLayoutDepth checks the limit of the depth of the walk. A tree that the
// parser makes is at most twice as deep as the levels of the parser, so the
// deepest input of each shape that parses also lays out. A tree that a
// program makes deeper than that gives ErrMathLimit, and no overflow of the
// stack.
func TestLayoutDepth(t *testing.T) {
	face := testFace(t)
	for _, s := range []struct{ open, mid, close string }{
		{"{", "x", "}"},
		{`\frac{`, "x", "}{y}"},
		{`\dfrac{`, "x", "}{y}"},
		{`\not `, "x", ""},
		{`a \over {`, "b", "}"},
		{`a \over `, "b", ""},
		{`x^{`, "y", "}"},
		{`{x^{`, "y", "}}"},
		{`\displaystyle{`, "x", "}"},
		{`\displaystyle{\textstyle{`, "x", "}}"},
		{`\left(`, "x", `\right)`},
		{`\sqrt{`, "x", "}"},
		{`\begin{matrix}`, "x", `\end{matrix}`},
		{`\mathchoice{`, "x", "}{}{}{}"},
		{`\text{`, "x", "}"},
		{`\scalebox{2}{`, "x", "}"},
		{`\lefteqn{`, "x", "}"},
		{`\ooalign{`, "x", "}"},
		{`\binom{`, "x", "}{y}"},
		{`\pmod{`, "x", "}"},
		{`\ket{`, "x", "}"},
		{`\sum_{`, "x", "}"},
		{`\xrightarrow{`, "x", "}"},
	} {
		last := 0
		for n := 1; n < 200; n++ {
			tex := strings.Repeat(s.open, n) + s.mid + strings.Repeat(s.close, n)
			if _, err := Parse(tex); err != nil {
				break
			}
			last = n
		}
		if last < 10 {
			t.Errorf("%q: only %d levels parse", s.open, last)
			continue
		}
		tex := strings.Repeat(s.open, last) + s.mid + strings.Repeat(s.close, last)
		if _, err := LayoutTeX(tex, face, 10, Display); err != nil {
			t.Errorf("%q, %d levels: %v", s.open, last, err)
		}
	}
	// A tree of a program, deeper than the limit: each Fraction is a level.
	// A deeper tree with a higher limit needs a larger stack than the 64 KiB
	// of a goroutine of TinyGo, so the test stays near the default.
	nest := func(levels int) Node {
		var n Node = Atom{Text: "x"}
		for range levels {
			n = Fraction{Num: n, Den: Atom{Text: "y"}}
		}
		return n
	}
	if _, err := Layout(nest(100000), face, 10, Display); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("100000 nested fractions: %v, want ErrMathLimit", err)
	}
	if _, err := Layout(nest(210), face, 10, Display); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("210 nested fractions: %v, want ErrMathLimit", err)
	}
	if _, err := Layout(nest(210), face, 10, Display, WithMaxDepth(106)); err != nil {
		t.Errorf("210 nested fractions with WithMaxDepth(106): %v", err)
	}
	var n Node
	// A StyleChange takes no level, so a long chain of them is fine.
	n = Atom{Text: "x"}
	for range 100000 {
		n = StyleChange{Style: Text, Body: n}
	}
	if _, err := Layout(n, face, 10, Display); err != nil {
		t.Errorf("100000 nested StyleChange nodes: %v", err)
	}
}

// TestLayoutDepthTextFace lays out the deepest \text that parses with a text
// face, so that the shaper runs at the deepest level of the walk, in the
// stack of 64 KiB of a goroutine of TinyGo for wasip1.
func TestLayoutDepthTextFace(t *testing.T) {
	face := testFace(t)
	text := textFace(t, "NotoSans-Regular.ttf")
	last := 0
	for n := 1; n < 200; n++ {
		if _, err := Parse(strings.Repeat(`\text{`, n) + "x" + strings.Repeat("}", n)); err != nil {
			break
		}
		last = n
	}
	tex := strings.Repeat(`\textbf{`, last) + "fix" + strings.Repeat("}", last)
	if _, err := LayoutTeX(tex, face, 10, Display, WithTextFace(text, text, nil, nil)); err != nil {
		t.Errorf("%d levels of \\textbf with a text face: %v", last, err)
	}
}

// TestLayoutItemLimit checks the limit of the items of a box.
func TestLayoutItemLimit(t *testing.T) {
	face := testFace(t)
	row := make(List, 0, 1000)
	for range 1000 {
		row = append(row, Atom{Class: Ord, Text: "x"})
	}
	rows := make([][]Node, 1001)
	for i := range rows {
		rows[i] = []Node{row}
	}
	if _, err := Layout(Array{Kind: ArrayMatrix, Rows: rows}, face, 10, Text); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("1,001,000 glyphs: %v, want ErrMathLimit", err)
	}
}

// TestLayoutLogsAMissingGlyph checks WithLogger: a character without a glyph
// gives an empty box and one warning.
func TestLayoutLogsAMissingGlyph(t *testing.T) {
	var sb strings.Builder
	lg := slog.New(slog.NewTextHandler(&sb, nil))
	b, err := LayoutTeX(`\char"10FFFD \char"10FFFD`, testFace(t), 10, Text, WithLogger(lg))
	if err != nil {
		t.Fatal(err)
	}
	if b.Width != 0 || len(b.Items) != 0 {
		t.Errorf("a missing glyph gives %+v, want an empty box", b)
	}
	if got := strings.Count(sb.String(), "level=WARN"); got != 1 {
		t.Errorf("%d warnings, want 1:\n%s", got, sb.String())
	}
}

// FuzzLayout gives random text to LayoutTeX with STIX Two Math, in each
// style and at random sizes (D78 section 5). LayoutTeX must not panic or
// hang, an error must wrap a sentinel error of okage, and a box must be
// finite. The seeds are the inputs of the gate and the formulas of the
// snapshots, which hold the corpus of test/texmath, in each style.
//
// The bits of style over the style choose the options of phase 26 of D78: a
// text face, and no italic correction, no math kerning or no top accent
// attachment.
func FuzzLayout(f *testing.F) {
	for i, tex := range fuzzSeeds(f) {
		f.Add(tex, uint8(i%256), 10.0)
	}
	face := testFace(f)
	text := textFace(f, "NotoSans-Regular.ttf")
	f.Fuzz(func(t *testing.T, tex string, style uint8, size float64) {
		var opts []Option
		if style&4 != 0 {
			opts = append(opts, WithTextFace(text, nil, text, nil))
		}
		if style&8 != 0 {
			opts = append(opts, WithItalicCorrection(false))
		}
		if style&16 != 0 {
			opts = append(opts, WithMathKern(false))
		}
		if style&32 != 0 {
			opts = append(opts, WithAccentAttachment(false))
		}
		b, err := LayoutTeX(tex, face, size, Style(style%4), opts...)
		if err != nil {
			if !errors.Is(err, okage.ErrTeXMathSyntax) && !errors.Is(err, okage.ErrMathLimit) {
				t.Fatalf("LayoutTeX(%q, %v): %v wraps no sentinel error", tex, size, err)
			}
			return
		}
		if size < 1e100 {
			checkBox(t, tex, b)
		}
		if len(b.Items) > maxItems {
			t.Fatalf("LayoutTeX(%q): %d items", tex, len(b.Items))
		}
	})
}
