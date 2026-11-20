// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"math"
	"testing"

	"github.com/xo/okage"
)

// The first tests read the lengths of the tree. The tests after them measure
// the box of \rule, as the tests of the source do. The source compares within
// 0.02 pixels, and these compare the exact lengths: one point is one unit of
// the size, so at the size 10 the box of \rule{40pt}{5pt} is 40 by 5.

func rule(t *testing.T, tex string) Rule {
	t.Helper()
	r, ok := mustParse(t, tex)[0].(Rule)
	if !ok {
		t.Fatalf("%s is not a Rule", tex)
	}
	return r
}

func TestRuleInMath(t *testing.T) {
	pt := func(v float64) Length { return Length{Value: v, Unit: Pt} }
	for _, c := range []struct {
		src  string
		want Rule
	}{
		{`\rule{40pt}{5pt}`, Rule{Width: pt(40), Height: pt(5), Lift: Length{}}},
		{`\rule[2pt]{10pt}{4pt}`, Rule{Width: pt(10), Height: pt(4), Lift: pt(2)}},
		{`\rule[-3pt]{10pt}{4pt}`, Rule{Width: pt(10), Height: pt(4), Lift: pt(-3)}},
		{`\rule[\baselineskip]{10pt}{4pt}`, Rule{Width: pt(10), Height: pt(4), Lift: pt(0)}},
		{`\rule{bogus}{5pt}`, Rule{Width: pt(0), Height: pt(5)}},
	} {
		got := rule(t, c.src)
		if got.Lift.Value == 0 {
			got.Lift = Length{} // 0pt and the zero Length are the same lift
		}
		want := c.want
		if want.Lift.Value == 0 {
			want.Lift = Length{}
		}
		if got != want {
			t.Errorf("%s = %#v, want %#v", c.src, got, want)
		}
	}
}

func TestRuleLengthUnits(t *testing.T) {
	for _, c := range []struct {
		src  string
		want Length
	}{
		{`\rule{1pt}{1pt}`, Length{1, Pt}},
		{`\rule{1pt}{1in}`, Length{7227.0 / 100, Pt}},
		{`\rule{1pt}{1pc}`, Length{12, Pt}},
		{`\rule{1pt}{1cm}`, Length{7227.0 / 254, Pt}},
		{`\rule{1pt}{1mm}`, Length{7227.0 / 2540, Pt}},
		{`\rule{1pt}{1bp}`, Length{7227.0 / 7200, Pt}},
		{`\rule{1pt}{1dd}`, Length{1238.0 / 1157, Pt}},
		{`\rule{1pt}{1cc}`, Length{14856.0 / 1157, Pt}},
		{`\rule{1pt}{65536sp}`, Length{1, Pt}},
		{`\rule{1pt}{1em}`, Length{1, Em}},
		{`\rule{1pt}{2ex}`, Length{2, Ex}},
		{`\rule{1pt}{5}`, Length{5, Pt}},
		{`\rule{1pt}{1zz}`, Length{0, Pt}},
	} {
		if got := rule(t, c.src).Height; got != c.want {
			t.Errorf("%s: height %#v, want %#v", c.src, got, c.want)
		}
	}
}

func TestRuleMalformed(t *testing.T) {
	for _, src := range []string{`\rule[2pt{10pt}{4pt}`, `\rule`, `\rule{10pt}`} {
		if _, err := Parse(src); err == nil {
			t.Errorf("%s: want an error", src)
		}
	}
}

// okage: a length past the limit is ErrMathLimit. The source sets it.
func TestRuleLengthLimit(t *testing.T) {
	for _, src := range []string{`\rule{20000pt}{1pt}`, `\rule{1pt}{1000000000000em}`, `\rule[-17000pt]{1pt}{1pt}`} {
		if _, err := Parse(src); !errors.Is(err, okage.ErrMathLimit) {
			t.Errorf("%s: %v, want ErrMathLimit", src, err)
		}
	}
	rule(t, `\rule{16384pt}{1pt}`)
}

// TestRuleBoxInMath is TestRuleInMath of the source: the box of the rule, and
// whether it draws a rectangle.
func TestRuleBoxInMath(t *testing.T) {
	face := testFace(t)
	for _, c := range []struct {
		src                 string
		wantRect            bool
		wantW, wantH, wantD float64
	}{
		{`\rule{40pt}{5pt}`, true, 40, 5, 0},
		{`\rule{0pt}{10pt}`, false, 0, 10, 0},      // an invisible strut: no ink, the full height
		{`\rule{10pt}{0pt}`, false, 10, 0, 0},      // no height: no ink
		{`\rule[2pt]{10pt}{4pt}`, true, 10, 6, 0},  // lifted: the box is lift+height high
		{`\rule[-3pt]{10pt}{4pt}`, true, 10, 1, 3}, // lowered: the rest is depth
		{`\rule{1pc}{1in}`, true, 12, 7227.0 / 100, 0},
	} {
		b := onFace(t, face, c.src, 10, Text)
		rules := itemsOf(b, ItemRule)
		if got := len(rules) == 1; got != c.wantRect {
			t.Errorf("%s: %d rules, want a rectangle %v", c.src, len(rules), c.wantRect)
		}
		if !near(b.Width, c.wantW) || !near(b.Height, c.wantH) || !near(b.Depth, c.wantD) {
			t.Errorf("%s: w=%v h=%v d=%v, want %v/%v/%v", c.src, b.Width, b.Height, b.Depth, c.wantW, c.wantH, c.wantD)
		}
	}
	// The rectangle has its bottom edge lift above the baseline, and y grows
	// down: \rule[-3pt]{10pt}{4pt} spans y from -1 to 3.
	r := itemsOf(onFace(t, face, `\rule[-3pt]{10pt}{4pt}`, 10, Text), ItemRule)[0]
	if r.X != 0 || r.Y != -1 || r.W != 10 || r.H != 4 {
		t.Errorf(`\rule[-3pt]{10pt}{4pt} draws %v,%v %vx%v, want 0,-1 10x4`, r.X, r.Y, r.W, r.H)
	}
}

// An unreadable length is zero and not an error.
func TestRuleWithAnUnreadableLength(t *testing.T) {
	b := mustLayout(t, `\rule{bogus}{5pt}`, 10, Text)
	if b.Width != 0 || b.Height != 5 || len(b.Items) != 0 {
		t.Errorf(`\rule{bogus}{5pt} = %v×%v with %d items, want 0×5 and no ink`, b.Width, b.Height, len(b.Items))
	}
}

// TestRuleHeightInEachUnit is the layout half of TestRuleLengthUnits of the
// source. An em is the size, and an ex is the height of the upright glyph x
// of the face, computed again from the font.
func TestRuleHeightInEachUnit(t *testing.T) {
	face := testFace(t)
	_, ex, _ := glyphMetrics(t, face, 'x', 10)
	for _, c := range []struct {
		src  string
		want float64
	}{
		{`\rule{1pt}{1pt}`, 1},
		{`\rule{1pt}{1in}`, 7227.0 / 100},
		{`\rule{1pt}{1pc}`, 12},
		{`\rule{1pt}{1cm}`, 7227.0 / 254},
		{`\rule{1pt}{1mm}`, 7227.0 / 2540},
		{`\rule{1pt}{1bp}`, 7227.0 / 7200},
		{`\rule{1pt}{1dd}`, 1238.0 / 1157},
		{`\rule{1pt}{1cc}`, 14856.0 / 1157},
		{`\rule{1pt}{65536sp}`, 1},
		{`\rule{1pt}{1em}`, 10},
		{`\rule{1pt}{1ex}`, ex},
		{`\rule{1pt}{5}`, 5},
		{`\rule{1pt}{1zz}`, 0},
	} {
		if b := onFace(t, face, c.src, 10, Text); !near(b.Height, c.want) {
			t.Errorf("%s: height %v, want %v", c.src, b.Height, c.want)
		}
	}
}

// 1ex is measured: the height of the glyph x at the size of the style, so it
// is above zero, below the em, and it grows with the size. In script style
// it is the height of x at the script size.
func TestRuleExIsMeasured(t *testing.T) {
	face := testFace(t)
	h := func(src string, size float64, st Style) float64 {
		t.Helper()
		return onFace(t, face, src, size, st).Height
	}
	one := h(`\rule{1pt}{1ex}`, 10, Text)
	if _, ex, _ := glyphMetrics(t, face, 'x', 10); one != ex || one <= 0 || one >= 10 {
		t.Errorf("1ex at 10 = %v, want the height %v of x, between 0 and the em", one, ex)
	}
	if two := h(`\rule{1pt}{2ex}`, 10, Text); !near(two, 2*one) {
		t.Errorf("2ex = %v, want twice 1ex (%v)", two, 2*one)
	}
	if big := h(`\rule{1pt}{1ex}`, 20, Text); !near(big, 2*one) {
		t.Errorf("1ex at 20 = %v, want %v", big, 2*one)
	}
	_, ex, _ := glyphMetrics(t, face, 'x', scriptSize(face, 10))
	if got := h(`\rule{1pt}{1ex}`, 10, Script); !near(got, ex) {
		t.Errorf("1ex in script style = %v, want %v", got, ex)
	}
	if got := h(`\rule{1pt}{1em}`, 10, Script); !near(got, scriptSize(face, 10)) {
		t.Errorf("1em in script style = %v, want the script size %v", got, scriptSize(face, 10))
	}
}

// A lift in a control sequence reads zero.
func TestRuleLiftWithControlSequence(t *testing.T) {
	if b := mustLayout(t, `\rule[\baselineskip]{10pt}{4pt}`, 10, Text); b.Height != 4 {
		t.Errorf("height %v, want 4: an unreadable lift is zero", b.Height)
	}
}

// A lift below the height of the rule leaves no height, and never a negative
// one.
func TestRuleLiftBelowItsOwnHeight(t *testing.T) {
	b := mustLayout(t, `\rule[-9pt]{10pt}{4pt}`, 10, Text)
	if b.Height != 0 || math.Abs(b.Depth-9) > 1e-12 {
		t.Errorf("h=%v d=%v, want h 0 and d 9", b.Height, b.Depth)
	}
}
