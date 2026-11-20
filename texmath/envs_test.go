// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"slices"
	"testing"
)

// The tests of the source that render the environments read the SVG text.
// Here they read the items of the Box. rowgrid_test.go measures the grids.

func TestEnvironments(t *testing.T) {
	for _, c := range []struct {
		tex         string
		kind        ArrayKind
		rows        []int // the cells of each row
		open, close rune
		stretch     float64
	}{
		{`\begin{matrix} a & b \\ c & d \end{matrix}`, ArrayMatrix, []int{2, 2}, 0, 0, 0},
		{`\begin{pmatrix} a \end{pmatrix}`, ArrayMatrix, []int{1}, '(', ')', 0},
		{`\begin{Vmatrix} a \end{Vmatrix}`, ArrayMatrix, []int{1}, '‖', '‖', 0},
		{`\begin{cases} 1 & x>0 \\ 0 & x\leq 0 \end{cases}`, ArrayMatrix, []int{2, 2}, '{', 0, 1.2},
		{`\begin{matrix} a \\ b & c \end{matrix}`, ArrayMatrix, []int{1, 2}, 0, 0, 0},
		{`\begin{aligned} x &= y + z \\ &= w \end{aligned}`, ArrayAligned, []int{2, 2}, 0, 0, 0},
		{`\begin{split} a \end{split}`, ArrayAligned, []int{1}, 0, 0, 0},
		{`\begin{gathered} a \\ b \end{gathered}`, ArrayGathered, []int{1, 1}, 0, 0, 0},
		{`\begin{smallmatrix} a & b \end{smallmatrix}`, ArraySmallMatrix, []int{2}, 0, 0, 0},
		{`\begin{array}{cc} a & b \\ c & d \end{array}`, ArrayArray, []int{2, 2}, 0, 0, 0},
		{`\substack{i<n \\ i \ne j}`, ArraySubstack, []int{1, 1}, 0, 0, 0},
	} {
		a, ok := mustParse(t, c.tex)[0].(Array)
		if !ok {
			t.Errorf("%s is not an Array", c.tex)
			continue
		}
		var rows []int
		for _, r := range a.Rows {
			rows = append(rows, len(r))
		}
		if a.Kind != c.kind || !slices.Equal(rows, c.rows) || a.Open != c.open || a.Close != c.close || a.Stretch != c.stretch {
			t.Errorf("%s = %#v", c.tex, a)
		}
	}
	a := as[Array](t, mustParse(t, `\begin{array}{|l|c|r|} 1 & 2 & 3 \end{array}`)[0])
	if !slices.Equal(a.Align, []Align{AlignLeft, AlignCenter, AlignRight}) || !slices.Equal(a.VRules, []int{0, 1, 2, 3}) {
		t.Errorf("array {|l|c|r|} = %v %v", a.Align, a.VRules)
	}
}

func TestEnvironmentErrors(t *testing.T) {
	for _, tex := range []string{
		`\begin{array}{cc} a & b`,
		`\begin{array}{cc a \end{array}`,
		`\begin{aligned} x &= y`,
		`\begin{gathered} a`,
		`\begin{smallmatrix} a`,
		`\begin{array}{c}\nope\end{array}`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("Parse(%q) should fail", tex)
		}
	}
	for _, tex := range []string{
		`\begin{array} a & b \end{array}`,
		`\begin{array}{} a \end{array}`,
		`\begin{array}{@c} a & b \end{array}`,
		`\begin{array}{p{2cm}c} a & b \end{array}`,
		`\begin{array}{z} a \end{array}`,
	} {
		mustParse(t, tex)
	}
}

func TestReadColSpec(t *testing.T) {
	for _, c := range []struct {
		in      string
		aligns  []Align
		vrules  []int
		restLen int
	}{
		{`{|c|c|}a`, []Align{AlignCenter, AlignCenter}, []int{0, 1, 2}, 1},
		{`{lcr}`, []Align{AlignLeft, AlignCenter, AlignRight}, nil, 0},
		{`{c|c}`, []Align{AlignCenter, AlignCenter}, []int{1}, 0},
		{`{p{2cm}r}x`, []Align{AlignLeft, AlignRight}, nil, 1},
		{`a & b`, nil, nil, 3},
		{`{z}`, []Align{AlignCenter}, nil, 0},
	} {
		a, v, rest := readColSpec(tokenize(c.in))
		if !slices.Equal(a, c.aligns) || !slices.Equal(v, c.vrules) || len(rest) != c.restLen {
			t.Errorf("readColSpec(%q) = %v %v %d, want %v %v %d", c.in, a, v, len(rest), c.aligns, c.vrules, c.restLen)
		}
	}
}

func TestEnvironmentsRender(t *testing.T) {
	face := testFace(t)
	for _, tex := range []string{
		`\begin{array}{cc} a & b \\ c & d \end{array}`,
		`\begin{array}{|c|c|} a & b \\ c & d \end{array}`,
		`\begin{array}{lcr} a & bb & ccc \\ d & e & f \end{array}`,
		`\begin{array}{|l|c|r|} 1 & 2 & 3 \end{array}`,
		`\begin{array}{c|c} a & b \end{array}`,
		`\begin{array}{p{2cm}c} a & b \end{array}`, // p{} is flush left
		`\left(\begin{array}{cc} a & b \\ c & d \end{array}\right)`,
		`\begin{aligned} x &= y + z \\ &= w \end{aligned}`,
		`\begin{split} f(x) &= a + b \\ &= c \end{split}`,
		`\begin{aligned} a \\ b \end{aligned}`,          // one column, with no &
		`\begin{aligned} a &= b & c &= d \end{aligned}`, // two pairs, with the gap between pairs
		`\begin{gathered} a + b \\ c \\ d + e + f \end{gathered}`,
		`\begin{smallmatrix} a & b \\ c & d \end{smallmatrix}`,
		`\left(\begin{smallmatrix} 1 & 0 \\ 0 & 1 \end{smallmatrix}\right)`,
		`\begin{aligned} A &= \frac{1}{2} \\ B &= \begin{smallmatrix} a \\ b \end{smallmatrix} \end{aligned}`,
	} {
		b, err := LayoutTeX(tex, face, 40, Text)
		if err != nil {
			t.Errorf("%s: %v", tex, err)
			continue
		}
		if len(itemsOf(b, ItemGlyph)) == 0 || !(b.Width > 0) {
			t.Errorf("%s: %d glyphs, a width of %v", tex, len(itemsOf(b, ItemGlyph)), b.Width)
		}
	}
}

// A | in the column specification draws a rule, and a rule at an edge
// widens the array by the gap between columns, half the size.
func TestArrayVerticalRules(t *testing.T) {
	plain := mustLayout(t, `\begin{array}{cc} a & b \\ c & d \end{array}`, 40, Text)
	ruled := mustLayout(t, `\begin{array}{|c|c|} a & b \\ c & d \end{array}`, 40, Text)
	mid := mustLayout(t, `\begin{array}{c|c} a & b \end{array}`, 40, Text)
	for _, c := range []struct {
		name string
		b    *Box
		want int
	}{{"cc", plain, 0}, {"|c|c|", ruled, 3}, {"c|c", mid, 1}} {
		if got := len(itemsOf(c.b, ItemRule)); got != c.want {
			t.Errorf("%s: %d rules, want %d", c.name, got, c.want)
		}
	}
	if want := plain.Width + 2*20.0; !near(ruled.Width, want) {
		t.Errorf("the ruled array is %v wide, want %v", ruled.Width, want)
	}
}

// l, c and r place a narrow cell at the left, in the middle and at the right
// of its column: the glyph i is at 0, at half the difference of the advances
// of i and W, and at the whole difference.
func TestArrayColumnAlignment(t *testing.T) {
	face := testFace(t)
	wi, _, _ := glyphMetrics(t, face, mathItalic('i'), 60)
	ww, _, _ := glyphMetrics(t, face, mathItalic('W'), 60)
	wi += italicOf(t, face, mathItalic('i'), 60)
	ww += italicOf(t, face, mathItalic('W'), 60)
	for _, c := range []struct {
		spec string
		want float64
	}{{"l", 0}, {"c", (ww - wi) / 2}, {"r", ww - wi}} {
		b := mustLayout(t, `\begin{array}{`+c.spec+`} i \\ W \end{array}`, 60, Text)
		if got := b.Items[0].X; !near(got, c.want) {
			t.Errorf("%s: the i is at %v, want %v", c.spec, got, c.want)
		}
	}
}

// smallmatrix is smaller than matrix.
func TestSmallMatrixSmaller(t *testing.T) {
	small := mustLayout(t, `\begin{smallmatrix} a & b \\ c & d \end{smallmatrix}`, 40, Text)
	big := mustLayout(t, `\begin{matrix} a & b \\ c & d \end{matrix}`, 40, Text)
	if !(small.Height+small.Depth < big.Height+big.Depth) {
		t.Errorf("smallmatrix is %v tall, matrix %v", small.Height+small.Depth, big.Height+big.Depth)
	}
}

// aligned sets its columns right and left in turn, so the = of each row
// starts at the same x.
func TestAlignedColumns(t *testing.T) {
	b := mustLayout(t, `\begin{aligned} x &= y \\ zz &= w \end{aligned}`, 40, Text)
	glyphs := itemsOf(b, ItemGlyph)
	if len(glyphs) != 7 {
		t.Fatalf("%d glyphs, want 7", len(glyphs))
	}
	if eq1, eq2 := glyphs[1], glyphs[5]; eq1.Text != "=" || eq2.Text != "=" || !near(eq1.X, eq2.X) {
		t.Errorf("the = are at %v (%q) and %v (%q), want the same x", eq1.X, eq1.Text, eq2.X, eq2.Text)
	}
	mustLayout(t, `\begin{split} x &= y \end{split}`, 40, Text)
}
