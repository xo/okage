package texmath

import (
	"math"
	"testing"
)

// This file tests the commands of KaTeX that the maintainers asked for on
// 2026-11-25, and the alignment of \cfrac, the dashed rule of \hdashline and
// the label of \tag at the right margin (D95).

// TestD95Parse checks that each new command parses, and that the commands
// that stand for others parse as them.
func TestD95Parse(t *testing.T) {
	for _, tex := range []string{
		`\underrightarrow{AB}`, `\underleftarrow{AB}`, `\underleftrightarrow{AB}`,
		`\overrightharpoon{ac}`, `\overleftharpoon{ac}`, `\overlinesegment{AB}`, `\underlinesegment{AB}`,
		`\overbracket{AB}^{n}`, `\underbracket{AB}_{n}`, `\overgroup{AB}`, `\undergroup{AB}`,
		`\utilde{AB}`, `\Overrightarrow{AB}`, `\vcenter{\frac{a}{b}}`, `a_{\angl n}`,
		`\text{\sout{abc}}`, `\sideset{_1^2}{_3^4}\sum_a^b`, `\sideset{}{'}\sum`,
		`\mathreflectbox{x^2}`, `\verb!x^2!`, `\url{https://katex.org/}`,
		`\href{https://katex.org/}{\KaTeX}`, `\htmlId{bar}{x}`,
		`\includegraphics[height=0.8em]{a.png}`, `\xtwoheadrightarrow{abc}`,
		`\xlongequal{abc}`, `\xtofrom[d]{abc}`, `{a \above{2pt} b}`, `{n\brace k}`,
		`{n\brack k}`, `\smallint`, `\\`, `a\\[2pt]b`, `\LaTeX`, `\TeX`, `\KaTeX`,
		`\begin{align} a&=b\\c&=d \end{align}`, `\begin{align*} a&=b \end{align*}`,
		`\begin{alignat}{2} a&=b&c&=d \end{alignat}`, `\begin{gather} a\\b \end{gather}`,
		`\begin{equation} a=b \end{equation}`, `\begin{rcases} a&b\\c&d \end{rcases}`,
		`\cosec x`, `\varGamma`, `\Bra{\phi}`, `\Ket{\psi}`, `\pmb{\mu}`,
		`\text{\textcircled a}`, `\minuso`,
	} {
		mustParse(t, tex)
		mustLayout(t, tex, 10, Display)
	}
	for _, c := range [][2]string{
		{`\lang x\rang`, `\langle x\rangle`},
		{`\dotso`, `\ldots`},
		{`\nsubseteqq`, `\not\subseteqq`},
		{`\href{u}{x+y}`, `{x+y}`},
		{`\htmlClass{c}{x}`, `{x}`},
		{`\varinjlim`, `\mathop{\underrightarrow{\mathrm{lim}}}\limits`},
	} {
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q is not %q", c[0], c[1])
		}
	}
	for _, tex := range []string{
		`\sideset{a}{}\sum`, `\sideset{_1}`, `\verb`, `\verb!x`, `\url{x`,
		`\includegraphics[height=x]{a}`, `\includegraphics`, `\href{u}`,
		`{a \above b}`, `\cfrac[x]{a}{b}`, `\begin{alignat} a \end{alignat}`, `\text{\'}`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("Parse(%q): want an error", tex)
		}
	}
}

// TestD95Text checks the accents and the symbols of the text mode, \verb and
// \url.
func TestD95Text(t *testing.T) {
	for tex, want := range map[string]string{
		`\text{\'{a}}`:          "á",
		`\text{\"o}`:            "ö",
		`\text{\v{ab}}`:         "ǎb",
		`\text{\textcircled a}`: "ⓐ",
		`\text{\textcircled 3}`: "③",
		`\text{\OE\ss}`:         "Œß",
		`\text{\textendash}`:    "–",
		`\verb!x^2 y!`:          "x^2 y",
		`\verb*|a b|`:           "a␣b",
		`\url{a.b/c\_d\#e}`:     "a.b/c_d#e",
	} {
		l := mustParse(t, tex)
		var text string
		var walk func(n Node)
		walk = func(n Node) {
			switch n := n.(type) {
			case List:
				for _, it := range n {
					walk(it)
				}
			case Atom:
				walk(n.Body)
			case TextRun:
				text += n.Text
			}
		}
		walk(l)
		if text != want {
			t.Errorf("%q has the text %q, want %q", tex, text, want)
		}
	}
	if r, ok := mustParse(t, `\verb!x!`)[0].(TextRun); !ok || r.Family != FamilyMono {
		t.Errorf(`\verb is not monospace text`)
	}
}

// TestD95Fractions checks \above, \brace and \brack, and the alignment of
// \cfrac.
func TestD95Fractions(t *testing.T) {
	// inner returns the first node in the group of tex.
	inner := func(tex string) Fraction {
		body := as[List](t, as[Atom](t, mustParse(t, tex)[0]).Body)
		return as[Fraction](t, body[0])
	}
	brace := inner(`{n\brace k}`)
	if !brace.NoRule || brace.Open != '{' || brace.Close != '}' {
		t.Errorf(`\brace = %+v`, brace)
	}
	brack := inner(`{n\brack k}`)
	if !brack.NoRule || brack.Open != '[' || brack.Close != ']' {
		t.Errorf(`\brack = %+v`, brack)
	}
	if f := inner(`{a\above{2pt}b}`); f.NoRule {
		t.Error(`\above{2pt} has no rule`)
	}
	if f := inner(`{a\above0pt b}`); !f.NoRule {
		t.Error(`\above0pt has a rule`)
	}
	// The numerator a of \cfrac[l] is at the left of the denominator, and
	// that of \cfrac[r] at its right.
	x := func(tex string) float64 {
		b := mustLayout(t, tex, 10, Display)
		return itemsOf(b, ItemGlyph)[0].X
	}
	l, c, r := x(`\cfrac[l]{a}{bbbb}`), x(`\cfrac{a}{bbbb}`), x(`\cfrac[r]{a}{bbbb}`)
	if !(l < c && c < r) {
		t.Errorf(`the numerator of \cfrac[l], \cfrac and \cfrac[r] is at %v, %v and %v`, l, c, r)
	}
	if !near(x(`\cfrac[c]{a}{bbbb}`), c) {
		t.Error(`\cfrac[c] is not \cfrac`)
	}
}

// TestD95HDashline checks that \hdashline draws a dashed rule where \hline
// draws a solid one, with the same height of the array.
func TestD95HDashline(t *testing.T) {
	solid := mustLayout(t, `\begin{array}{cc}a&b\\\hline c&d\end{array}`, 10, Display)
	dashed := mustLayout(t, `\begin{array}{cc}a&b\\\hdashline c&d\end{array}`, 10, Display)
	if solid.Height != dashed.Height || solid.Depth != dashed.Depth || solid.Width != dashed.Width {
		t.Errorf("the dashed array is %v by %v+%v, the solid one %v by %v+%v",
			dashed.Width, dashed.Height, dashed.Depth, solid.Width, solid.Height, solid.Depth)
	}
	sr, dr := itemsOf(solid, ItemRule), itemsOf(dashed, ItemRule)
	if len(sr) != 1 || len(dr) < 2 {
		t.Fatalf("%d solid rules and %d dashes", len(sr), len(dr))
	}
	total := 0.0
	for _, d := range dr {
		if d.Y != sr[0].Y || d.H != sr[0].H {
			t.Errorf("a dash is at %v, %v high, and the rule at %v, %v high", d.Y, d.H, sr[0].Y, sr[0].H)
		}
		if d.W > hdashLength*10+1e-9 {
			t.Errorf("a dash is %v wide", d.W)
		}
		total += d.W
	}
	if !(total > sr[0].W*0.4 && total < sr[0].W*0.75) {
		t.Errorf("the dashes cover %v of a rule of %v", total, sr[0].W)
	}
	if got := as[Array](t, mustParse(t, `\begin{array}{c}\hline a\\\hdashline b\end{array}`)[0]).HDashed; len(got) != 2 || got[0] || !got[1] {
		t.Errorf("HDashed = %v", got)
	}
	if got := as[Array](t, mustParse(t, `\begin{array}{c}\hline a\end{array}`)[0]).HDashed; got != nil {
		t.Errorf("HDashed = %v without \\hdashline", got)
	}
}

// TestD95LineWidth checks that WithLineWidth sets the label of \tag at the
// right margin, with the body centered, and changes no other formula.
func TestD95LineWidth(t *testing.T) {
	face := testFace(t)
	plain, err := LayoutTeX(`a=b\tag{1}`, face, 10, Display)
	if err != nil {
		t.Fatal(err)
	}
	wide, err := LayoutTeX(`a=b\tag{1}`, face, 10, Display, WithLineWidth(300))
	if err != nil {
		t.Fatal(err)
	}
	if wide.Width != 300 || wide.Height != plain.Height || wide.Depth != plain.Depth {
		t.Errorf("the tagged formula is %v by %v+%v", wide.Width, wide.Height, wide.Depth)
	}
	last := wide.Items[len(wide.Items)-1]
	w, _, _ := gidMetrics(face, last.Glyph, last.Size)
	if !near(last.X+w, 300) {
		t.Errorf("the label ends at %v, want 300", last.X+w)
	}
	body := plain.Items[0]
	if first := wide.Items[0]; first.X <= body.X+100 {
		t.Errorf("the body starts at %v, not centered", first.X)
	}
	// A line that is too short grows to the formula, which keeps its quad.
	short, err := LayoutTeX(`a=b\tag{1}`, face, 10, Display, WithLineWidth(1))
	if err != nil {
		t.Fatal(err)
	}
	if !equalBoxes(short, plain) {
		t.Error("a short line changes the formula")
	}
	// A formula without \tag does not change.
	for _, w := range []float64{300, math.Inf(1), math.NaN(), -1} {
		got, err := LayoutTeX(`a=b`, face, 10, Display, WithLineWidth(w))
		if err != nil {
			t.Fatal(err)
		}
		if !equalBoxes(got, onFace(t, face, `a=b`, 10, Display)) {
			t.Errorf("WithLineWidth(%v) changes a formula without \\tag", w)
		}
	}
}

// TestD95Decorations checks the arrows, the harpoons, the segments and the
// stretched characters over and under a base.
func TestD95Decorations(t *testing.T) {
	base := mustLayout(t, `AB`, 10, Display)
	for _, c := range []struct {
		tex   string
		under bool
	}{
		{`\underrightarrow{AB}`, true}, {`\underlinesegment{AB}`, true}, {`\undergroup{AB}`, true},
		{`\utilde{AB}`, true}, {`\underbracket{AB}`, true}, {`\overrightharpoon{AB}`, false},
		{`\overlinesegment{AB}`, false}, {`\overgroup{AB}`, false}, {`\Overrightarrow{AB}`, false},
		{`\overbracket{AB}`, false},
	} {
		b := mustLayout(t, c.tex, 10, Display)
		if len(b.Items) <= len(base.Items) {
			t.Errorf("%s draws nothing more than its base", c.tex)
		}
		if c.under && !(b.Depth > base.Depth+1 && near(b.Height, base.Height)) {
			t.Errorf("%s is %v+%v, not under the base", c.tex, b.Height, b.Depth)
		}
		if !c.under && !(b.Height > base.Height+1 && near(b.Depth, base.Depth)) {
			t.Errorf("%s is %v+%v, not over the base", c.tex, b.Height, b.Depth)
		}
	}
	if n := len(itemsOf(mustLayout(t, `\overlinesegment{AB}`, 10, Display), ItemRule)); n != 3 {
		t.Errorf(`\overlinesegment has %d rules, want the line and two bars`, n)
	}
	if n := len(itemsOf(mustLayout(t, `\overrightharpoon{AB}`, 10, Display), ItemPath)); n != 1 {
		t.Errorf(`\overrightharpoon has %d paths, want one head`, n)
	}
	for _, tex := range []string{`\xtwoheadrightarrow{abc}`, `\xtwoheadleftarrow{abc}`} {
		if n := len(itemsOf(mustLayout(t, tex, 10, Display), ItemPath)); n != 2 {
			t.Errorf("%s has %d heads, want 2", tex, n)
		}
	}
	for _, tex := range []string{`\xrightleftharpoons{abc}`, `\xleftrightharpoons{abc}`, `\xtofrom{abc}`, `\xlongequal{abc}`} {
		b := mustLayout(t, tex, 10, Display)
		if n := len(itemsOf(b, ItemRule)); n != 2 {
			t.Errorf("%s has %d shafts, want 2", tex, n)
		}
	}
}

// TestD95Boxes checks \vcenter, \angl, \sout, \smallint, \sideset and
// \includegraphics.
func TestD95Boxes(t *testing.T) {
	v := mustLayout(t, `\vcenter{\frac{\frac a b}c}`, 10, Display)
	f := mustLayout(t, `\frac{\frac a b}c`, 10, Display)
	if !near(v.Height+v.Depth, f.Height+f.Depth) {
		t.Errorf(`\vcenter changes the extent: %v, want %v`, v.Height+v.Depth, f.Height+f.Depth)
	}
	a := mustLayout(t, `\angl n`, 10, Display)
	n := mustLayout(t, `n`, 10, Display)
	if rules := itemsOf(a, ItemRule); len(rules) != 2 || !(a.Width > n.Width) || !near(a.Depth, 2.5) {
		t.Errorf(`\angl n is %v by %v+%v with %d rules`, a.Width, a.Height, a.Depth, len(rules))
	}
	s := mustLayout(t, `\text{\sout{abc}}`, 10, Display)
	plain := mustLayout(t, `\text{abc}`, 10, Display)
	if rules := itemsOf(s, ItemRule); len(rules) != 1 || s.Width != plain.Width || rules[0].W != s.Width {
		t.Errorf(`\sout{abc} has %d rules in a width %v`, len(rules), s.Width)
	}
	small := mustLayout(t, `\smallint`, 10, Display)
	large := mustLayout(t, `\int`, 10, Display)
	if !(small.Height+small.Depth < large.Height+large.Depth) {
		t.Error(`\smallint grows in display style`)
	}
	side := mustLayout(t, `\sideset{_1^2}{_3^4}\sum_a^b`, 10, Display)
	sum := mustLayout(t, `\sum_a^b`, 10, Display)
	if !(side.Width > sum.Width) || len(side.Items) != len(sum.Items)+4 {
		t.Errorf(`\sideset is %v wide with %d items, \sum %v with %d`, side.Width, len(side.Items), sum.Width, len(sum.Items))
	}
	img := mustLayout(t, `\includegraphics[height=0.8em, totalheight=0.9em, width=2em]{a.png}`, 10, Display)
	if !near(img.Width, 20) || !near(img.Height, 8) || !near(img.Depth, 1) || len(img.Items) != 0 {
		t.Errorf(`\includegraphics is %v by %v+%v with %d items`, img.Width, img.Height, img.Depth, len(img.Items))
	}
	if img := mustLayout(t, `\includegraphics{a.png}`, 10, Display); !near(img.Width, 9) || !near(img.Height, 9) {
		t.Errorf(`\includegraphics without options is %v by %v`, img.Width, img.Height)
	}
}
