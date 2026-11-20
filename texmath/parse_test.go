package texmath

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// This file tests what okage adds to the parser of the source: the tree, the
// TextRun of \text with its spaces, \limits, the style commands, the limits
// of D78 section 5 and the positions of the errors.

// textOfGroup returns the items of the Ord group that \text makes.
func textOfGroup(t *testing.T, tex string) List {
	t.Helper()
	a, ok := mustParse(t, tex)[0].(Atom)
	if !ok {
		t.Fatalf("%s is not a group", tex)
	}
	return as[List](t, a.Body)
}

func TestTextKeepsItsSpaces(t *testing.T) {
	for _, c := range []struct {
		tex  string
		want List
	}{
		{`\text{if } b`, List{TextRun{Text: "if "}}},
		{`\text{ if  x }`, List{TextRun{Text: " if x "}}},
		{"\\text{a\n\tb}", List{TextRun{Text: "a b"}}},
		{"\\text{a\r\nb}", List{TextRun{Text: "a b"}}},
		{`\text{don't}`, List{TextRun{Text: "don't"}}},
		{`\text{50\% \& more}`, List{TextRun{Text: "50% & more"}}},
		{`\text{a\ b}`, List{TextRun{Text: "a b"}}},
		{`\mbox{if } x`, List{TextRun{Text: "if "}}},
		{`\text x`, List{TextRun{Text: "x"}}},
		// TeX skips the white space after a control word.
		{`\text{\alpha b}`, List{Atom{Class: Ord, Text: "α", Variant: Upright}, TextRun{Text: "b"}}},
		{`\text{a \alpha}`, List{TextRun{Text: "a "}, Atom{Class: Ord, Text: "α", Variant: Upright}}},
		// A $ returns to math up to the next $, and the space before the
		// closing $ is a space of math.
		{`\text{a $x $ b}`, List{TextRun{Text: "a "}, Atom{Class: Ord, Text: "x", Variant: Italic}, TextRun{Text: " b"}}},
		{`\text{a$b$c}`, List{TextRun{Text: "a"}, Atom{Class: Ord, Text: "b", Variant: Italic}, TextRun{Text: "c"}}},
	} {
		if got := textOfGroup(t, c.tex); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s = %#v, want %#v", c.tex, got, c.want)
		}
	}
	// A space of math is not kept.
	if !sameTree(t, `a + b`, `a+b`) {
		t.Error("a + b and a+b differ")
	}
	// \mathrm is math, so its spaces are not kept.
	if !sameTree(t, `\mathrm{a b}`, `\mathrm{ab}`) {
		t.Error(`\mathrm keeps a space`)
	}
}

func TestANestedDollarInsideTextReturnsToMaths(t *testing.T) {
	for _, c := range [][2]string{
		{`$a+b$`, `a+b`},
		{`\(a+b\)`, `a+b`},
		{`\[a+b\]`, `a+b`},
		{`\scalebox{0.5}{$a+b$}`, `\scalebox{0.5}{a+b}`},
		{`\mathbf{a$b$c}`, `\mathbf{a}b\mathbf{c}`},
	} {
		if c[0] == `\mathbf{a$b$c}` {
			// The group keeps the three atoms: a and c bold, b italic.
			got := textOfGroup(t, c[0])
			want := List{
				Atom{Class: Ord, Text: "a", Variant: Bold},
				Atom{Class: Ord, Text: "b", Variant: Italic},
				Atom{Class: Ord, Text: "c", Variant: Bold},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s = %#v", c[0], got)
			}
			continue
		}
		if !sameTree(t, c[0], c[1]) {
			t.Errorf("%q does not parse as %q", c[0], c[1])
		}
	}
}

func TestLimitsSetsThePlaceOfTheScripts(t *testing.T) {
	for _, c := range []struct {
		tex  string
		want Limits
	}{
		{`\sum_{i}a_i`, LimitsAuto},
		{`\sum\limits_{i}a_i`, LimitsAbove},
		{`\sum\nolimits_{i}a_i`, LimitsSide},
		{`\int_0^1`, LimitsSide},
		{`\int\limits_0^1`, LimitsAbove},
		{`\lim\nolimits\limits_{x}`, LimitsAbove},
		{`\mathop{X}_a`, LimitsAuto},
	} {
		s, ok := mustParse(t, c.tex)[0].(Scripts)
		if !ok {
			t.Errorf("%s does not start with Scripts", c.tex)
			continue
		}
		if a := as[Atom](t, s.Base); a.Limits != c.want || a.Class != Op {
			t.Errorf("%s: limits %d, want %d", c.tex, a.Limits, c.want)
		}
	}
	// After an atom that is not an operator, \limits does nothing.
	if !sameTree(t, `a\limits_b`, `a_b`) {
		t.Error(`\limits after an Ord changes the tree`)
	}
	if a := as[Atom](t, mustParse(t, `\sum`)[0]); !a.Large {
		t.Error(`\sum does not grow in display style`)
	}
}

func TestAStyleCommandTakesTheRestOfItsGroup(t *testing.T) {
	l := mustParse(t, `a \displaystyle b c \scriptstyle d`)
	want := List{
		Atom{Class: Ord, Text: "a"},
		StyleChange{Style: Display, Body: List{Atom{Class: Ord, Text: "b"}, Atom{Class: Ord, Text: "c"}}},
		StyleChange{Style: Script, Body: List{Atom{Class: Ord, Text: "d"}}},
	}
	if !reflect.DeepEqual(l, want) {
		t.Errorf("got %#v, want %#v", l, want)
	}
	// A style command at the end, or followed by another, makes no node.
	if !sameTree(t, `x\displaystyle`, `x`) || !sameTree(t, `\displaystyle\textstyle x`, `\textstyle x`) {
		t.Error("an empty style makes a node")
	}
	// The style ends with its group.
	g := mustParse(t, `{\displaystyle a}b`)
	if _, ok := g[1].(Atom); !ok || len(g) != 2 {
		t.Errorf(`{\displaystyle a}b = %#v`, g)
	}
	// A style before \over sets the numerator part after it.
	f := as[Fraction](t, mustParse(t, `a \displaystyle b \over c`)[0])
	if num := as[List](t, f.Num); len(num) != 2 {
		t.Errorf("numerator %#v", num)
	}
	if _, ok := as[List](t, f.Den)[0].(Atom); !ok {
		t.Errorf("denominator %#v, want no style", f.Den)
	}
}

func TestParseErrorPositions(t *testing.T) {
	for _, c := range []struct {
		tex string
		pos int
	}{
		{`a + \foo`, 4},
		{`{a`, 2},
		{`ab}`, 2},
		{`\frac{a}`, 8},
		{`\left( x \right\zzz`, 15},
		{`x^{α\foo}`, 5},
	} {
		_, err := Parse(c.tex)
		var se *SyntaxError
		if !errors.As(err, &se) || se.Pos != c.pos {
			t.Errorf("Parse(%q): %#v, want a fault at byte %d", c.tex, err, c.pos)
		}
	}
}

func TestTheDepthLimit(t *testing.T) {
	deep := strings.Repeat("{", 1000) + strings.Repeat("}", 1000)
	for _, tex := range []string{
		deep,
		strings.Repeat(`\frac`, 1000) + "ab",
		strings.Repeat(`\not`, 1000) + "a",
		strings.Repeat(`\sqrt[`, 1000),
		strings.Repeat(`a \over `, 1000) + "b",
		strings.Repeat(`\left(`, 1000) + strings.Repeat(`\right)`, 1000),
		strings.Repeat(`\begin{matrix}`, 1000),
		strings.Repeat(`\mathchoice{`, 1000) + strings.Repeat(`}{}{}{}`, 1000),
		strings.Repeat(`x^{`, 1000),
		strings.Repeat(`\text{`, 1000),
		strings.Repeat(`\ooalign{`, 1000) + strings.Repeat(`}`, 1000),
		strings.Repeat(`\scalebox{2}{`, 1000),
	} {
		t.Run(tex[:12], func(t *testing.T) {
			_, err := Parse(tex)
			if !errors.Is(err, okage.ErrMathLimit) {
				t.Errorf("Parse(%.30q…): %v, want ErrMathLimit", tex, err)
			}
		})
	}
	// x in {{x}} is six levels deep.
	if _, err := Parse(`{{x}}`, WithMaxDepth(6)); err != nil {
		t.Errorf("WithMaxDepth(6): %v", err)
	}
	if _, err := Parse(`{{x}}`, WithMaxDepth(5)); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("WithMaxDepth(5): %v", err)
	}
	// 40 nested groups are fine, and a lower limit refuses them.
	tex := strings.Repeat("{", 40) + "x" + strings.Repeat("}", 40)
	mustParse(t, tex)
	if _, err := Parse(tex, WithMaxDepth(20)); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("WithMaxDepth(20): %v", err)
	}
	if got := err2string(Parse(deep)); !strings.HasPrefix(got, "typesetting math: limit reached: ") {
		t.Errorf("the text of a limit is %q", got)
	}
}

func err2string(_ List, err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestTheNodeLimit(t *testing.T) {
	tex := strings.Repeat("x", 2000)
	mustParse(t, tex)
	if _, err := Parse(tex, WithMaxNodes(1000)); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("WithMaxNodes(1000): %v", err)
	}
	if _, err := Parse(strings.Repeat("x", defaultMaxNodes+1)); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("%d atoms: %v", defaultMaxNodes+1, err)
	}
	if _, err := Parse(strings.Repeat("x", maxTokens+1)); !errors.Is(err, okage.ErrMathLimit) {
		t.Errorf("%d tokens: %v", maxTokens+1, err)
	}
}

// countNodes returns the number of nodes of a tree.
func countNodes(n Node) int {
	switch n := n.(type) {
	case nil:
		return 0
	case List:
		c := 1
		for _, m := range n {
			c += countNodes(m)
		}
		return c
	case Atom:
		return 1 + countNodes(n.Body)
	case Scripts:
		return 1 + countNodes(n.Base) + countNodes(n.Sup) + countNodes(n.Sub)
	case Fraction:
		return 1 + countNodes(n.Num) + countNodes(n.Den)
	case Radical:
		return 1 + countNodes(n.Body) + countNodes(n.Degree)
	case Delimited:
		return 1 + countNodes(n.Body)
	case Accent:
		return 1 + countNodes(n.Base)
	case OverUnder:
		return 1 + countNodes(n.Base) + countNodes(n.Over) + countNodes(n.Under)
	case Bar:
		return 1 + countNodes(n.Base)
	case Array:
		c := 1
		for _, r := range n.Rows {
			for _, m := range r {
				c += countNodes(m)
			}
		}
		return c
	case StyleChange:
		return 1 + countNodes(n.Body)
	case Color:
		return 1 + countNodes(n.Body)
	case SizeChange:
		return 1 + countNodes(n.Body)
	case Cancel:
		return 1 + countNodes(n.Body)
	case Phase:
		return 1 + countNodes(n.Body)
	case Tag:
		return 1 + countNodes(n.Body) + countNodes(n.Label)
	case Phantom:
		return 1 + countNodes(n.Body)
	case Raise:
		return 1 + countNodes(n.Body)
	case Scale:
		return 1 + countNodes(n.Body)
	case Boxed:
		return 1 + countNodes(n.Body)
	case Negate:
		return 1 + countNodes(n.Body)
	case Brace:
		return 1 + countNodes(n.Base)
	case OverArrow:
		return 1 + countNodes(n.Base)
	case XArrow:
		return 1 + countNodes(n.Over) + countNodes(n.Under)
	case Lap:
		return 1 + countNodes(n.Body)
	case Overlay:
		c := 1
		for _, r := range n.Rows {
			c += countNodes(r.Body)
		}
		return c
	case Choice:
		return 1 + countNodes(n.Display) + countNodes(n.Text) + countNodes(n.Script) + countNodes(n.ScriptScript)
	default: // Delimiter, Space, TextRun, Rule
		return 1
	}
}

// The parser counts each node that it makes, so the count of the limit is
// the count of the tree. The optional argument of \smash is read and dropped,
// so its nodes are counted and not in the tree.
func TestTheNodeCountIsTheTree(t *testing.T) {
	for _, tex := range gateInputs(t) {
		if tex.err != "" || strings.Contains(tex.tex, `\smash`) {
			continue
		}
		p := &parser{tex: tex.tex, cfg: newConfig(nil)}
		toks, err := p.expandMacros(resolveConditionals(tokenize(tex.tex)))
		if err != nil {
			t.Fatal(err)
		}
		l, _, err := p.parseList(toks, alphabet{}, stopEnd)
		if err != nil {
			t.Fatalf("%q: %v", tex.tex, err)
		}
		if got := countNodes(l); got != p.nodes {
			t.Errorf("%q: the tree has %d nodes, the parser counted %d", tex.tex, got, p.nodes)
		}
	}
}
