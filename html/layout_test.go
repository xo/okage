package html

import (
	"errors"
	"math"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage"
)

// mono measures text as Courier in dot of Graphviz 16.1.0 does: each
// character is 0.6 of the size wide, a line is 1.275 of the size high, and
// the ascent is 0.9 of the size. The expected values of these tests come
// from dot -Tjson with fontname="Courier" and fontsize=10.
func mono(text string, style TextStyle) (Metrics, error) {
	return Metrics{
		Width:  0.6 * style.Size * float64(utf8.RuneCountInString(text)),
		Height: 1.275 * style.Size,
		Ascent: 0.9 * style.Size,
	}, nil
}

func layout(t *testing.T, src string) *Layout {
	t.Helper()
	l, issues, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	lay, err := l.Layout(WithMeasure(mono), WithTextStyle(TextStyle{Size: 10}))
	if err != nil {
		t.Fatal(err)
	}
	return lay
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 0.006
}

func checkRect(t *testing.T, name string, got, want Rect) {
	t.Helper()
	if !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.Width, want.Width) || !near(got.Height, want.Height) {
		t.Errorf("%s: got %+v, want %+v", name, got, want)
	}
}

func checkBoxes(t *testing.T, lay *Layout, want ...Rect) {
	t.Helper()
	if len(lay.Boxes) != len(want) {
		t.Fatalf("got %d boxes, want %d: %+v", len(lay.Boxes), len(want), lay.Boxes)
	}
	for i, b := range lay.Boxes {
		checkRect(t, "box", b.Rect, want[i])
	}
}

// pt is the left end and the baseline of a run.
type pt struct {
	text string
	x, y float64
}

func checkRuns(t *testing.T, lay *Layout, want ...pt) {
	t.Helper()
	if len(lay.Runs) != len(want) {
		t.Fatalf("got %d runs, want %d: %+v", len(lay.Runs), len(want), lay.Runs)
	}
	for i, r := range lay.Runs {
		w := want[i]
		if r.Text != w.text || !near(r.X, w.x) || !near(r.Y, w.y) {
			t.Errorf("run %d: got %q at %v,%v, want %q at %v,%v", i, r.Text, r.X, r.Y, w.text, w.x, w.y)
		}
	}
}

func TestLayoutCell(t *testing.T) {
	lay := layout(t, `<table border="1" cellborder="1" cellspacing="2" cellpadding="2" port="t"><tr><td port="c">x</td></tr></table>`)
	checkRect(t, "size", Rect{Width: lay.Width, Height: lay.Height}, Rect{Width: 18, Height: 24.75})
	checkBoxes(t, lay, Rect{0, 0, 18, 24.75}, Rect{3, 3, 12, 18.75})
	checkRuns(t, lay, pt{"x", 6, 15})
	checkRect(t, "port t", lay.Ports["t"], Rect{0, 0, 18, 24.75})
	checkRect(t, "port c", lay.Ports["c"], Rect{3, 3, 12, 18.75})
	r := lay.Runs[0]
	if !near(r.Top, 6) || !near(r.Height, 12.75) || r.Style.Size != 10 || r.Under != "" {
		t.Errorf("run: %+v", r)
	}
}

func TestLayoutBorders(t *testing.T) {
	lay := layout(t, `<table border="3" cellborder="2" cellspacing="4"><tr><td>x</td></tr></table>`)
	checkBoxes(t, lay, Rect{0, 0, 28, 34.75}, Rect{7, 7, 14, 20.75})
	if lay.Boxes[0].StrokeWidth != 3 || lay.Boxes[1].StrokeWidth != 2 {
		t.Errorf("stroke widths: %+v", lay.Boxes)
	}
	checkRuns(t, lay, pt{"x", 11, 20})
}

func TestLayoutSpans(t *testing.T) {
	lay := layout(t, `<table><tr><td colspan="2">WWWWWWWWWWWW</td><td>c</td></tr><tr><td>a</td><td colspan="2">bbbbbbbbbbbbbbbbbbbbbb</td></tr></table>`)
	checkBoxes(t, lay,
		Rect{0, 0, 190, 45.5},
		Rect{3, 3, 124, 18.75},
		Rect{129, 3, 58, 18.75},
		Rect{3, 23.75, 44, 18.75},
		Rect{49, 23.75, 138, 18.75},
	)
	checkRuns(t, lay, pt{"WWWWWWWWWWWW", 29, 15}, pt{"c", 155, 15}, pt{"a", 22, 35.75}, pt{"bbbbbbbbbbbbbbbbbbbbbb", 52, 35.75})
}

func TestLayoutFixedSize(t *testing.T) {
	lay := layout(t, `<table><tr><td fixedsize="true" width="40" height="30">xxxxxxxxxxxx</td><td width="40" height="30">y</td></tr></table>`)
	checkBoxes(t, lay, Rect{0, 0, 88, 36}, Rect{3, 3, 40, 30}, Rect{45, 3, 40, 30})
	checkRuns(t, lay, pt{"xxxxxxxxxxxx", -13, 20.625}, pt{"y", 62, 20.625})
}

func TestLayoutTableSize(t *testing.T) {
	// A width and a height of a table leave the cells at the top left.
	lay := layout(t, `<table width="100" height="60"><tr><td>a</td><td>b</td></tr></table>`)
	checkBoxes(t, lay, Rect{0, 0, 100, 60}, Rect{3, 3, 12, 18.75}, Rect{17, 3, 12, 18.75})
}

func TestLayoutStretch(t *testing.T) {
	// A table in a wider cell fills it, and its columns share the extra
	// width in equal parts.
	lay := layout(t, `<table border="0"><tr><td>WWWWWWWWWWWWW</td></tr><tr><td><table><tr><td>a</td><td>b</td><td>c</td></tr></table></td></tr></table>`)
	// A table with border="0" also gives its cells no border.
	checkBoxes(t, lay,
		Rect{4, 22.75, 78, 24.75},
		Rect{7, 25.75, 22.667, 18.75},
		Rect{31.667, 25.75, 22.667, 18.75},
		Rect{56.333, 25.75, 22.667, 18.75},
	)
	checkRuns(t, lay, pt{"WWWWWWWWWWWWW", 4, 13}, pt{"a", 15.333, 37.75}, pt{"b", 40, 37.75}, pt{"c", 64.667, 37.75})
}

func TestLayoutNestedFixed(t *testing.T) {
	lay := layout(t, `<table border="0"><tr><td>WWWWWWWWWWWWWWWWWWWW</td></tr><tr><td><table width="40" height="20" fixedsize="true" valign="top"><tr><td>a</td></tr></table></td></tr></table>`)
	// The fixed table keeps its size, in the center of the cell.
	checkRect(t, "fixed", lay.Boxes[0].Rect, Rect{44, 22.75, 40, 20})
}

func TestLayoutAlign(t *testing.T) {
	lay := layout(t, `<table><tr><td width="100">WW<br align="left"/>x<br align="right"/>yyyy</td></tr><tr><td width="100" balign="right">WW<br/>x</td></tr><tr><td width="100" align="left">WW<br align="right"/>xxxx</td></tr></table>`)
	checkRuns(t, lay,
		pt{"WW", 41, 15}, pt{"x", 59, 27.75}, pt{"yyyy", 41, 40.5},
		pt{"WW", 47, 61.25}, pt{"x", 53, 74},
		pt{"WW", 18, 94.75}, pt{"xxxx", 6, 107.5},
	)
}

func TestLayoutAlignCell(t *testing.T) {
	lay := layout(t, `<table><tr><td>WWWWWWWWWWWW</td></tr><tr><td align="left">l</td></tr><tr><td align="right">r</td></tr><tr><td valign="top" height="40">t</td></tr><tr><td align="text">WWWW<br align="left"/>x<br/></td></tr></table>`)
	checkRuns(t, lay,
		pt{"WWWWWWWWWWWW", 6, 15}, pt{"l", 6, 35.75}, pt{"r", 72, 56.5}, pt{"t", 39, 77.25},
		pt{"WWWW", 6, 119.25}, pt{"x", 39, 132},
	)
}

func TestLayoutRules(t *testing.T) {
	for _, src := range []string{
		`<table rows="*" columns="*"><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>`,
		`<table><tr><td>a</td><vr/><td>b</td></tr><hr/><tr><td>c</td><vr/><td>d</td></tr></table>`,
	} {
		lay := layout(t, src)
		want := [][4]float64{
			{16, 0, 16, 22.75},
			{0, 22.75, 16, 22.75},
			{16, 22.75, 32, 22.75},
			{16, 22.75, 16, 45.5},
		}
		if len(lay.Rules) != len(want) {
			t.Fatalf("rules: %+v", lay.Rules)
		}
		for i, r := range lay.Rules {
			w := want[i]
			if !near(r.X1, w[0]) || !near(r.Y1, w[1]) || !near(r.X2, w[2]) || !near(r.Y2, w[3]) || r.Width != 1 {
				t.Errorf("rule %d: got %+v, want %v", i, r, w)
			}
		}
	}
}

func TestLayoutSides(t *testing.T) {
	lay := layout(t, `<table border="0" cellborder="1" cellspacing="4" color="blue"><tr><td sides="LB" border="3" bgcolor="red">a</td><td bgcolor="yellow" color="green"><font color="white">b</font></td></tr></table>`)
	checkBoxes(t, lay, Rect{4, 4, 16, 22.75}, Rect{24, 4, 12, 22.75})
	b := lay.Boxes
	if b[0].Fill != "red" || b[0].StrokeWidth != 0 || b[1].Stroke != "green" || b[1].StrokeWidth != 1 {
		t.Errorf("boxes: %+v", b)
	}
	want := []Rule{
		{X1: 5.5, Y1: 5.5, X2: 5.5, Y2: 25.25, Stroke: "blue", Width: 3, Cell: b[0].Cell},
		{X1: 5.5, Y1: 25.25, X2: 18.5, Y2: 25.25, Stroke: "blue", Width: 3, Cell: b[0].Cell},
	}
	if len(lay.Rules) != 2 || lay.Rules[0] != want[0] || lay.Rules[1] != want[1] {
		t.Errorf("rules: got %+v, want %+v", lay.Rules, want)
	}
	if lay.Runs[0].Under != "red" || lay.Runs[1].Under != "yellow" || lay.Runs[1].Color != "white" {
		t.Errorf("runs: %+v", lay.Runs)
	}
}

func TestLayoutText(t *testing.T) {
	lay := layout(t, `ab<br align="left"/>cccc`)
	checkRect(t, "size", Rect{Width: lay.Width, Height: lay.Height}, Rect{Width: 24, Height: 25.5})
	checkRuns(t, lay, pt{"ab", 0, 9}, pt{"cccc", 0, 21.75})
	if len(lay.Boxes) != 0 {
		t.Errorf("boxes: %+v", lay.Boxes)
	}
}

func TestLayoutFonts(t *testing.T) {
	lay := layout(t, `<table><tr><td>a<font point-size="20" face="Mono">B</font>c<sup>2</sup><sub>3</sub></td></tr></table>`)
	r := lay.Runs
	if len(r) != 5 {
		t.Fatalf("runs: %+v", r)
	}
	// The runs share a baseline below the top by the largest ascent.
	if !near(r[0].Y, r[1].Y) || !near(r[1].Y-r[1].Top, 18) || r[1].Style.Family != "Mono" || r[1].Style.Size != 20 {
		t.Errorf("runs: %+v", r)
	}
	if !near(r[3].Style.Size, 10*subSupScale) || !near(r[3].Shift, 10.0/3) || !near(r[0].Y-r[3].Y, 10.0/3) || !r[3].Sup {
		t.Errorf("sup: %+v", r[3])
	}
	if !near(r[4].Shift, -2) || !near(r[4].Y-r[0].Y, 2) || !r[4].Sub {
		t.Errorf("sub: %+v", r[4])
	}
}

func TestLayoutMeasureError(t *testing.T) {
	l, _, err := Parse("x")
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	_, err = l.Layout(WithMeasure(func(string, TextStyle) (Metrics, error) { return Metrics{}, boom }))
	if !errors.Is(err, okage.ErrHTMLMeasure) || !errors.Is(err, boom) {
		t.Errorf("got %v", err)
	}
}
