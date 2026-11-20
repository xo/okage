package texmath

import (
	"errors"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// This file checks that the deepest input that the limit of the depth allows
// parses in the stack of a goroutine of TinyGo for wasip1, which is 64 KiB.
// On that platform, Parse overflowed the stack on \pmod{ 24 times and on
// "a \over " 98 times, and then hung. The nightly job of TinyGo runs these
// tests for wasip1, and parsestack_tinygo_test.go measures the stack there.

// deepShape is a shape of nesting: open n times, then mid, then close n
// times.
type deepShape struct{ open, mid, close string }

// deepShapes are the shapes of the test. Each one nests through a different
// path of the parser.
var deepShapes = []deepShape{
	{"{", "x", "}"},
	{`\frac{`, "x", "}{y}"},
	{`\not `, "x", ""},
	{`a \over {`, "b", "}"},
	{`a \over `, "b", ""},
	{`x^{`, "y", "}"},
	{`\left(`, "x", `\right)`},
	{`\sqrt{`, "x", "}"},
	{`\sqrt[`, "x", "]{y}"},
	{`\begin{matrix}`, "x", `\end{matrix}`},
	{`\mathchoice{`, "x", "}{}{}{}"},
	{`\text{`, "x", "}"},
	{`\scalebox{2}{`, "x", "}"},
	{`\ooalign{`, "x", "}"},
	{`\binom{`, "x", "}{y}"},
	{`\pmod{`, "x", "}"},
	{`\ket{`, "x", "}"},
	{`\ketbra{`, "x", "}{y}"},
	{`\sum_{`, "x", "}"},
	{`\xrightarrow{`, "x", "}"},
	{`\xrightarrow[`, "x", "]{}"},
	{`\overset{`, "x", "}{y}"},
	{`\buildrel `, "x", `\over y`},
	{`\genfrac(){1pt}{0}{`, "x", "}{y}"},
	{`\raisebox{1pt}{`, "x", "}"},
	{`\substack{`, "x", "}"},
	{`\hat{`, "x", "}"},
	{`\smash[t]{`, "x", "}"},
	{`\lefteqn{`, "x", "}"},
	{`\mathbin{`, "x", "}"},
}

// nested returns the shape s nested n times, and the same without its mid
// and close, which does not end.
func (s deepShape) nested(n int) (closed, open string) {
	open = strings.Repeat(s.open, n)
	return open + s.mid + strings.Repeat(s.close, n), open
}

// TestParseDeepInputs parses the two inputs that overflowed the stack, and
// each shape at each depth up to past the limit. The deepest form of each
// shape that parses must parse, and a deeper one must reach the limit.
func TestParseDeepInputs(t *testing.T) {
	if _, err := Parse(strings.Repeat(`\pmod{`, 24)); !errors.Is(err, okage.ErrTeXMathSyntax) {
		t.Errorf(`\pmod{ 24 times: %v, want the error of a missing }`, err)
	}
	if _, err := Parse(strings.Repeat(`a \over `, 98)); err != nil {
		t.Errorf(`"a \over " 98 times: %v`, err)
	}
	for _, s := range deepShapes {
		last := 0
		for n := 1; n <= 120; n++ {
			closed, open := s.nested(n)
			_, err := Parse(closed)
			if err == nil {
				last = n
			} else if !errors.Is(err, okage.ErrMathLimit) {
				t.Errorf("%q, %d levels: %v", s.open, n, err)
			}
			if _, err := Parse(open); err == nil && s.close != "" {
				t.Errorf("%q, %d levels without an end: no error", s.open, n)
			}
		}
		if last < 15 || last >= 120 {
			t.Errorf("%q: %d levels parse", s.open, last)
		}
		closed, _ := s.nested(last + 1)
		if _, err := Parse(closed); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("%q, %d levels: %v, want ErrMathLimit", s.open, last+1, err)
		}
	}
}
