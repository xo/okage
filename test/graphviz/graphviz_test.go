package graphviz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage/html"
)

// tolerance is the largest difference of a coordinate. dot writes two
// decimals.
const tolerance = 0.015

// labels are the labels that the tests compare. The text is Courier at 10
// points unless a font tag sets another size.
var labels = []struct {
	name, src string
}{
	{"Cell", `<table border="1" cellborder="1" cellspacing="2" cellpadding="2"><tr><td>x</td></tr></table>`},
	{"Borders", `<table border="3" cellborder="2" cellspacing="4"><tr><td>x</td></tr></table>`},
	{"Span", `<table><tr><td colspan="2">WWWWWWWWWWWW</td></tr><tr><td>a</td><td>b</td></tr></table>`},
	{"Spans", `<table><tr><td colspan="2">WWWWWWWWWWWW</td><td>c</td></tr><tr><td>a</td><td colspan="2">bbbbbbbbbbbbbbbbbbbbbb</td></tr></table>`},
	{"RowSpan", `<table><tr><td rowspan="3">a<br/>b<br/>c<br/>d<br/>e</td><td>x</td></tr><tr><td>y</td></tr><tr><td>z</td></tr></table>`},
	{"FixedSize", `<table><tr><td fixedsize="true" width="40" height="30">xxxxxxxxxxxx</td><td width="40" height="30">y</td></tr></table>`},
	{"TableSize", `<table width="100" height="60"><tr><td>a</td><td>b</td></tr></table>`},
	{"TableWidth", `<table width="100"><tr><td>a</td><td>bbbbb</td></tr><tr><td colspan="2">c</td></tr></table>`},
	{"AlignCell", `<table><tr><td>WWWWWWWWWWWW</td></tr><tr><td align="left">l</td></tr><tr><td align="right">r</td></tr><tr><td valign="top" height="40">t</td></tr><tr><td valign="bottom" height="40">b</td></tr></table>`},
	{"Lines", `<table><tr><td>WWWWWWWWWWWW<br/>c<br align="left"/>dd<br align="right"/>ee</td></tr><tr><td balign="left">WWWWWWWWWWWW<br/>x</td></tr><tr><td align="text">WWWW<br align="left"/>x<br/></td></tr></table>`},
	{"AlignLines", `<table><tr><td width="100">WW<br align="left"/>x<br align="right"/>yyyy</td></tr><tr><td width="100" balign="right">WW<br/>x</td></tr><tr><td width="100" align="left">WW<br align="right"/>xxxx</td></tr></table>`},
	{"Stretch", `<table border="0"><tr><td>WWWWWWWWWWWWW</td></tr><tr><td><table><tr><td>a</td><td>b</td><td>c</td></tr></table></td></tr></table>`},
	{"StretchCell", `<table border="0"><tr><td>WWWWWWWWWWWW</td></tr><tr><td><table><tr><td>a</td></tr></table></td></tr><tr><td align="left"><table><tr><td>a</td></tr></table></td></tr></table>`},
	{"StretchRows", `<table border="0"><tr><td height="60">x</td><td valign="bottom"><table><tr><td>a</td></tr><tr><td>b</td></tr></table></td></tr></table>`},
	{"NestedFixed", `<table border="0"><tr><td>WWWWWWWWWWWWWWWWWWWW</td></tr><tr><td><table width="40" height="30" fixedsize="true"><tr><td>a</td></tr></table></td></tr></table>`},
	{"Rules", `<table rows="*" columns="*"><tr><td>a</td><td>b</td><td>c</td></tr><tr><td>d</td><td>e</td><td>f</td></tr></table>`},
	{"RuleTags", `<table><tr><td>a</td><vr/><td>b</td></tr><hr/><tr><td>c</td><td>d</td></tr></table>`},
	{"Sides", `<table border="0" cellborder="1" cellspacing="4"><tr><td sides="LB" border="3" port="a">a</td><td sides="TR" port="b">b</td><td sides="LTB" border="2" port="c">c</td></tr></table>`},
	{"Fill", `<table bgcolor="yellow"><tr><td bgcolor="red">a</td><td>b</td></tr></table>`},
	{"Empty", `<table><tr><td></td><td> </td></tr></table>`},
	{"EmptyCell", `<table><tr><td></td></tr></table>`},
	{"Text", `ab<br align="left"/>cccc`},
	{"Spaces", `  a   b  `},
	{"Entities", `x&eacute;&nbsp;&#65;&#x42;&Alpha;&hellip;y`},
	{"FontTable", `<font point-size="14"><table><tr><td>a</td><td><font color="blue">b</font></td></tr></table></font>`},
	{"FontSize", `<table><tr><td><font point-size="14">WW</font></td></tr><tr><td>x</td></tr></table>`},
	{"Schema", `
		<table border="0" cellborder="1" cellspacing="0" cellpadding="4">
		<tr><td bgcolor="lightblue">"books"</td></tr>
		<tr><td align="left" PORT="book_id">book_id: integer</td></tr>
		<tr><td align="left" PORT="author_id">author_id: integer</td></tr>
		<tr><td align="left" PORT="isbn">isbn: varchar</td></tr>
		<tr><td align="left" PORT="available">available: timestamp</td></tr>
		</table>`},
}

// metrics are the height and the ascent of a line of Courier at a size, as
// dot lays it out.
type metrics struct {
	height, ascent float64
}

// dotPath returns the path of dot, or skips the test.
func dotPath(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("dot is not installed")
	}
	return p
}

// op is a drawing operation of dot -Tjson.
type op struct {
	Op     string       `json:"op"`
	Points [][2]float64 `json:"points"`
	Pt     [2]float64   `json:"pt"`
	Text   string       `json:"text"`
	Width  float64      `json:"width"`
	Size   float64      `json:"size"`
}

// object is a node of dot -Tjson.
type object struct {
	Name  string `json:"name"`
	Pos   string `json:"pos"`
	LDraw []op   `json:"_ldraw_"` //nolint:tagliatelle // the key of dot -Tjson

}

// runDot lays out one label as the label of a node, and returns the center
// of the node and the drawing operations of its label.
func runDot(t *testing.T, dot, src string) (float64, float64, []op) {
	t.Helper()
	graph := "digraph { node [shape=plaintext fontname=\"Courier\" fontsize=10]\nn [label=<" + src + ">]\n}\n"
	cmd := exec.CommandContext(t.Context(), dot, "-Tjson")
	cmd.Stdin = strings.NewReader(graph)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("dot: %v: %s", err, stderr.String())
	}
	var g struct {
		Objects []object `json:"objects"`
	}
	if err := json.Unmarshal(out, &g); err != nil {
		t.Fatalf("dot -Tjson: %v", err)
	}
	for _, o := range g.Objects {
		if o.Name != "n" {
			continue
		}
		x, y, ok := strings.Cut(o.Pos, ",")
		if !ok {
			t.Fatalf("dot: pos %q", o.Pos)
		}
		px, err1 := strconv.ParseFloat(x, 64)
		py, err2 := strconv.ParseFloat(y, 64)
		if err1 != nil || err2 != nil {
			t.Fatalf("dot: pos %q", o.Pos)
		}
		return px, py, o.LDraw
	}
	t.Fatal("dot: no node")
	return 0, 0, nil
}

// probe returns the metrics of a line of Courier at a size. The probe is two
// lines with no border and no padding, so its height is two lines, and the
// top of the label is one line above the center of the node.
func probe(t *testing.T, dot string, size float64) metrics {
	t.Helper()
	s := strconv.FormatFloat(size, 'f', -1, 64)
	src := `<table border="0" cellborder="0" cellpadding="0" cellspacing="0"><tr><td><font point-size="` + s + `">x<br/>x</font></td></tr></table>`
	_, py, ops := runDot(t, dot, src)
	var base []float64
	for _, o := range ops {
		if o.Op == "T" {
			base = append(base, o.Pt[1])
		}
	}
	if len(base) != 2 {
		t.Fatalf("probe: %d runs", len(base))
	}
	h := base[0] - base[1]
	return metrics{height: h, ascent: py + h - base[0]}
}

// segment is a line from one point to another, with the points in order.
type segment struct {
	x1, y1, x2, y2 float64
}

// newSegment returns a segment with its points in order.
func newSegment(x1, y1, x2, y2 float64) segment {
	if x2 < x1 || x2 == x1 && y2 < y1 {
		x1, y1, x2, y2 = x2, y2, x1, y1
	}
	return segment{x1, y1, x2, y2}
}

func cmpSegment(a, b segment) int {
	for _, d := range []float64{a.x1 - b.x1, a.y1 - b.y1, a.x2 - b.x2, a.y2 - b.y2} {
		if math.Abs(d) > tolerance {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	return 0
}

// drawing is what dot draws for a label, in the coordinates of html: from the
// top left of the label, with y down.
type drawing struct {
	boxes []html.Rect
	rules []segment
	runs  []html.Run
}

// convert converts the operations of dot. A polygon with an area is a box,
// a polygon with no area is a rule, a polyline is the sides of a cell, and a
// text is a run.
func convert(ops []op, ox, oy float64) drawing {
	var d drawing
	pt := func(p [2]float64) (float64, float64) {
		return p[0] - ox, oy - p[1]
	}
	for _, o := range ops {
		switch o.Op {
		case "p", "P":
			x0, y0 := math.Inf(1), math.Inf(1)
			x1, y1 := math.Inf(-1), math.Inf(-1)
			for _, p := range o.Points {
				x, y := pt(p)
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
			}
			if x1-x0 > tolerance && y1-y0 > tolerance {
				d.boxes = append(d.boxes, html.Rect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0})
				continue
			}
			d.rules = append(d.rules, newSegment(x0, y0, x1, y1))
		case "L":
			for i := 1; i < len(o.Points); i++ {
				x1, y1 := pt(o.Points[i-1])
				x2, y2 := pt(o.Points[i])
				d.rules = append(d.rules, newSegment(x1, y1, x2, y2))
			}
		case "T":
			x, y := pt(o.Pt)
			d.runs = append(d.runs, html.Run{X: x, Y: y, Width: o.Width, Text: o.Text})
		}
	}
	return d
}

// expected returns what dot draws for a layout. dot draws the fill and the
// border of a box as two polygons, and insets a border, or a side of a cell,
// by half its width only when it is wider than 1.
func expected(lay *html.Layout) drawing {
	var d drawing
	cells := map[*html.Cell]html.Rect{}
	for _, b := range lay.Boxes {
		if b.Cell != nil {
			cells[b.Cell] = b.Rect
		}
		if b.Fill != "" && b.StrokeWidth > 0 {
			d.boxes = append(d.boxes, b.Rect)
		}
		r := b.Rect
		if b.StrokeWidth > 1 {
			in := b.StrokeWidth / 2
			r = html.Rect{X: r.X + in, Y: r.Y + in, Width: r.Width - 2*in, Height: r.Height - 2*in}
		}
		d.boxes = append(d.boxes, r)
	}
	for _, r := range lay.Rules {
		x1, y1, x2, y2 := r.X1, r.Y1, r.X2, r.Y2
		box, ok := cells[r.Cell]
		if !ok && r.Cell != nil && r.Cell.Port != "" {
			// A cell with sides and no fill has no box: take the box of
			// its port.
			box, ok = lay.Ports[r.Cell.Port]
		}
		if ok && r.Width <= 1 {
			// Undo the inset of html: move each end that is half the
			// width inside the box onto the edge of the box.
			edge := func(v, lo, hi float64) float64 {
				switch {
				case near(v, lo+r.Width/2):
					return lo
				case near(v, hi-r.Width/2):
					return hi
				}
				return v
			}
			x1, x2 = edge(x1, box.X, box.X+box.Width), edge(x2, box.X, box.X+box.Width)
			y1, y2 = edge(y1, box.Y, box.Y+box.Height), edge(y2, box.Y, box.Y+box.Height)
		}
		d.rules = append(d.rules, newSegment(x1, y1, x2, y2))
	}
	d.runs = lay.Runs
	return d
}

func near(a, b float64) bool {
	return math.Abs(a-b) <= tolerance
}

// runKey is a text at a size.
type runKey struct {
	text string
	size float64
}

// widths returns the width that dot gives each text at each size. Pango
// hints the advances of Courier, so a size other than 10 is not exactly 0.6
// of the size for each character.
func widths(ops []op) map[runKey]float64 {
	w := map[runKey]float64{}
	var size float64
	for _, o := range ops {
		switch o.Op {
		case "F":
			size = o.Size
		case "T":
			w[runKey{o.Text, size}] = o.Width
		}
	}
	return w
}

func TestDot(t *testing.T) {
	dot := dotPath(t)
	sizes := map[float64]metrics{}
	for _, tt := range labels {
		t.Run(tt.name, func(t *testing.T) {
			l, issues, err := html.Parse(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if len(issues) != 0 {
				t.Fatalf("issues: %v", issues)
			}
			px, py, ops := runDot(t, dot, tt.src)
			w := widths(ops)
			measure := func(text string, style html.TextStyle) (html.Metrics, error) {
				m, ok := sizes[style.Size]
				if !ok {
					m = probe(t, dot, style.Size)
					sizes[style.Size] = m
				}
				width, ok := w[runKey{text, style.Size}]
				if !ok {
					width = 0.6 * style.Size * float64(utf8.RuneCountInString(text))
				}
				return html.Metrics{Width: width, Height: m.height, Ascent: m.ascent}, nil
			}
			lay, err := l.Layout(html.WithMeasure(measure), html.WithTextStyle(html.TextStyle{Family: "Courier", Size: 10}))
			if err != nil {
				t.Fatal(err)
			}
			got := expected(lay)
			want := convert(ops, px-lay.Width/2, py+lay.Height/2)
			compare(t, got, want)
		})
	}
}

// compare compares the drawing of html with the drawing of dot.
func compare(t *testing.T, got, want drawing) {
	t.Helper()
	if len(got.boxes) != len(want.boxes) {
		t.Errorf("boxes: got %d, want %d\n got %v\nwant %v", len(got.boxes), len(want.boxes), got.boxes, want.boxes)
	} else {
		for i, g := range got.boxes {
			w := want.boxes[i]
			if !near(g.X, w.X) || !near(g.Y, w.Y) || !near(g.Width, w.Width) || !near(g.Height, w.Height) {
				t.Errorf("box %d: got %+v, want %+v", i, g, w)
			}
		}
	}
	slices.SortFunc(got.rules, cmpSegment)
	slices.SortFunc(want.rules, cmpSegment)
	if !slices.EqualFunc(got.rules, want.rules, func(a, b segment) bool { return cmpSegment(a, b) == 0 }) {
		t.Errorf("rules:\n got %v\nwant %v", got.rules, want.rules)
	}
	if len(got.runs) != len(want.runs) {
		t.Errorf("runs: got %d, want %d\n got %v\nwant %v", len(got.runs), len(want.runs), runs(got.runs), runs(want.runs))
		return
	}
	for i, g := range got.runs {
		w := want.runs[i]
		if g.Text != w.Text || !near(g.X, w.X) || !near(g.Y, w.Y) || !near(g.Width, w.Width) {
			t.Errorf("run %d: got %q at %.3f,%.3f width %.3f, want %q at %.3f,%.3f width %.3f", i, g.Text, g.X, g.Y, g.Width, w.Text, w.X, w.Y, w.Width)
		}
	}
}

// runs returns the runs as text, for a message.
func runs(rs []html.Run) string {
	var b strings.Builder
	for _, r := range rs {
		fmt.Fprintf(&b, "%q@%.2f,%.2f ", r.Text, r.X, r.Y)
	}
	return b.String()
}
