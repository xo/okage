package texmath

import (
	"testing"

	"github.com/xo/okage/typesetting/font"
)

// layoutWith lays out tex with the test face at the size 10 in text style,
// with opts.
func layoutWith(t *testing.T, tex string, opts ...Option) *Box {
	t.Helper()
	b, err := LayoutTeX(tex, testFace(t), 10, Text, opts...)
	if err != nil {
		t.Fatalf("LayoutTeX(%q): %v", tex, err)
	}
	return b
}

// TestDotsOfTeX checks that the dots are three dots of the class Punct in
// an atom of the class Inner, and that \dots looks at what follows it
// (D103).
func TestDotsOfTeX(t *testing.T) {
	for _, c := range []struct {
		tex string
		dot string
	}{
		{`\ldots`, "."}, {`\mathellipsis`, "."}, {`\dotsc`, "."}, {`\dotso`, "."},
		{`\cdots`, "⋅"}, {`\dotsb`, "⋅"}, {`\dotsm`, "⋅"},
		{`\dots`, "."}, {`\dots,`, "."}, {`\dots+`, "⋅"}, {`\dots=`, "⋅"},
		{`\dots\le`, "⋅"}, {`\dots\sum`, "⋅"}, {`\dots\times`, "⋅"},
	} {
		l, err := Parse(c.tex)
		if err != nil {
			t.Fatalf("%s: %v", c.tex, err)
		}
		a, ok := l[0].(Atom)
		body, _ := a.Body.(List)
		if !ok || a.Class != Inner || len(body) != 3 {
			t.Errorf("%s: expected an Inner atom of three dots, got %#v", c.tex, l[0])
			continue
		}
		for _, d := range body {
			if d, ok := d.(Atom); !ok || d.Class != Punct || d.Text != c.dot {
				t.Errorf("%s: expected the dot %q of the class Punct, got %#v", c.tex, c.dot, d)
			}
		}
	}
	// Three dots with a thin space after each of the first two, in text
	// style.
	face := testFace(t)
	w, _, _ := glyphMetrics(t, face, '.', 10)
	if b, want := layoutWith(t, `\ldots`), 3*w+2*10*3.0/18; !near(b.Width, want) {
		t.Errorf(`\ldots: expected the width %v, got %v`, want, b.Width)
	}
	// \dotsi is \!\cdots.
	if a, b := layoutWith(t, `\dotsi`), layoutWith(t, `\cdots`); !near(a.Width, b.Width-10*3.0/18) {
		t.Errorf(`\dotsi: expected the width of \cdots less a thin space, got %v and %v`, a.Width, b.Width)
	}
	// In \text, \ldots stays one glyph.
	l, err := Parse(`\text{\ldots}`)
	if err != nil {
		t.Fatal(err)
	}
	outer, _ := l[0].(Atom)
	body, _ := outer.Body.(List)
	var glyph Atom
	if len(body) == 1 {
		glyph, _ = body[0].(Atom)
	}
	if glyph.Text != "…" {
		t.Errorf(`\text{\ldots}: expected the glyph U+2026, got %#v`, l[0])
	}
}

// TestPrimesAtFullSize checks that the primes are at the size of the
// nucleus, as \prime after it, and that a superscript of \prime only is
// primes (D103).
func TestPrimesAtFullSize(t *testing.T) {
	for _, c := range [][2]string{
		{`x'`, `x\prime`},
		{`x''`, `x\prime\prime`},
		{`x'^2`, `x\prime^2`},
		{`x'_i`, `x\prime_i`},
		{`x^\prime`, `x'`},
		{`x^{\prime\prime}`, `x''`},
	} {
		a, b := layoutWith(t, c[0]), layoutWith(t, c[1])
		if !near(a.Width, b.Width) || !near(a.Height, b.Height) || !near(a.Depth, b.Depth) {
			t.Errorf("%s: expected the box of %s, %v %v %v, got %v %v %v",
				c[0], c[1], b.Width, b.Height, b.Depth, a.Width, a.Height, a.Depth)
		}
	}
}

// TestAccentScripts checks that an accent over one character takes its
// scripts as TeX Appendix G, rule 12, says (D103).
func TestAccentScripts(t *testing.T) {
	for _, c := range [][2]string{
		{`\hat y_i`, `y_i`},
		{`\hat{y}_i`, `y_i`},
		{`\tilde x^2_i`, `x^2_i`},
		{`\hat y'_i`, `y'_i`},
	} {
		a, b := layoutWith(t, c[0]), layoutWith(t, c[1])
		if !near(a.Depth, b.Depth) {
			t.Errorf("%s: expected the depth of %s, %v, got %v", c[0], c[1], b.Depth, a.Depth)
		}
		if a.Width < b.Width {
			t.Errorf("%s: expected at least the width of %s, %v, got %v", c[0], c[1], b.Width, a.Width)
		}
	}
	// The accent stays where it is over the character alone.
	if a, b := layoutWith(t, `\hat y^2`), layoutWith(t, `\hat y`); a.Height < b.Height {
		t.Errorf(`\hat y^2: expected at least the height of \hat y, %v, got %v`, b.Height, a.Height)
	}
	// An accent over a formula keeps the drop of a box.
	if a, b := layoutWith(t, `\hat{xy}_i`), layoutWith(t, `{xy}_i`); !near(a.Depth, b.Depth) {
		t.Errorf(`\hat{xy}_i: expected the depth of {xy}_i, %v, got %v`, b.Depth, a.Depth)
	}
}

// TestSubscriptShifts checks WithMathConstant, WithSubSupShiftDown and the
// drop of a subscript at the size of the script (D103).
func TestSubscriptShifts(t *testing.T) {
	face := testFace(t)
	upem := float64(face.Upem())
	_, _, dn := glyphMetrics(t, face, mathItalic('n'), scriptSize(face, 10))

	// K_n with SubscriptShiftDown set to 0.4 em: n goes down 4 units.
	shift := float32(0.4 * upem)
	if b := layoutWith(t, `K_n`, WithMathConstant(font.SubscriptShiftDown, shift)); !near(b.Depth, 4+dn) {
		t.Errorf("K_n: expected the depth %v with the shift 0.4 em, got %v", 4+dn, b.Depth)
	}
	// The face does not change.
	if a, b := layoutWith(t, `K_n`), mustLayout(t, `K_n`, 10, Text); !near(a.Depth, b.Depth) {
		t.Errorf("K_n: expected the face without the constant, got %v and %v", a.Depth, b.Depth)
	}
	// WithSubSupShiftDown moves a subscript with a superscript only.
	sub2 := WithSubSupShiftDown(shift)
	if a, b := layoutWith(t, `K_n`, sub2), layoutWith(t, `K_n`); !near(a.Depth, b.Depth) {
		t.Errorf("K_n: expected no change from WithSubSupShiftDown, got %v and %v", a.Depth, b.Depth)
	}
	if b := layoutWith(t, `K_n^2`, sub2); b.Depth < 4+dn {
		t.Errorf("K_n^2: expected a depth of at least %v, got %v", 4+dn, b.Depth)
	}

	// The drop under a box is at the size of the script.
	drop := mathConst(face, font.SubscriptBaselineDropMin, scriptSize(face, 10))
	_, _, dy := glyphMetrics(t, face, mathItalic('y'), 10)
	b := layoutWith(t, `{y}_n`, WithMathConstant(font.SubscriptShiftDown, 0))
	if want := max(dy, dy+drop+dn); !near(b.Depth, want) {
		t.Errorf("{y}_n: expected the depth %v, got %v", want, b.Depth)
	}
}

// TestAlignedCellsInDisplayStyle checks that the cells of aligned, split
// and gathered are in display style (D103).
func TestAlignedCellsInDisplayStyle(t *testing.T) {
	want := layoutWith(t, `\displaystyle\int`)
	for _, env := range []string{"aligned", "split", "gathered", "alignedat"} {
		arg := ""
		if env == "alignedat" {
			arg = "{1}"
		}
		b := layoutWith(t, `\begin{`+env+`}`+arg+`\int\end{`+env+`}`)
		if b.Height < want.Height {
			t.Errorf("%s: expected an integral of display style, %v high, got %v", env, want.Height, b.Height)
		}
	}
	if a, b := layoutWith(t, `\begin{matrix}\int\end{matrix}`), want; a.Height >= b.Height {
		t.Errorf("matrix: expected an integral of text style, got %v, as high as %v", a.Height, b.Height)
	}
}
