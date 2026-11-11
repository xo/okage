package html

import (
	"os"
	"testing"

	"github.com/xo/okage/svg"
)

// testRenderer returns a renderer with the fonts of the tests of resvg, and
// Noto Sans as the default family.
func testRenderer() *svg.Renderer {
	return svg.New(
		svg.WithFontFS(os.DirFS("../svg/testdata/fonts")),
		svg.WithFontFamily("Noto Sans"),
		svg.WithSansSerifFamily("Noto Sans"),
	)
}

// TestRendererMeasure checks that the default measure gives the width of
// svg.ShapeText, the height and the ascent of its box, and the fallback of
// svg for CJK text.
func TestRendererMeasure(t *testing.T) {
	r := testRenderer()
	l, _, err := Parse("<table><tr><td><b>id</b>: \u6f22\u5b57</td></tr></table>")
	if err != nil {
		t.Fatal(err)
	}
	lay, err := l.Layout(WithRenderer(r), WithTextStyle(TextStyle{Size: 11}))
	if err != nil {
		t.Fatal(err)
	}
	if len(lay.Runs) != 2 {
		t.Fatalf("runs: %+v", lay.Runs)
	}
	for i, st := range []svg.TextStyle{
		{Size: 11, Weight: 700},
		{Size: 11},
	} {
		run, err := r.ShapeText(lay.Runs[i].Text, st)
		if err != nil {
			t.Fatal(err)
		}
		b := run.Bounds()
		got := lay.Runs[i]
		if got.Width != float64(run.Width()) || got.Width <= 0 {
			t.Errorf("run %d: width %v, want %v", i, got.Width, run.Width())
		}
		if got.Y-got.Top < float64(-b.Y())-1e-9 || got.Height < float64(b.Height())-1e-9 {
			t.Errorf("run %d: ascent %v and height %v, want at least %v and %v", i, got.Y-got.Top, got.Height, -b.Y(), b.Height())
		}
	}
	if lay.Runs[1].X != lay.Runs[0].X+lay.Runs[0].Width {
		t.Errorf("runs: %+v", lay.Runs)
	}
}

func TestCSSFamily(t *testing.T) {
	for face, want := range map[string]string{
		"":                 "",
		"Helvetica":        `"Helvetica"`,
		"IBM Plex Sans":    `"IBM Plex Sans"`,
		"Noto Sans, serif": "Noto Sans, serif",
	} {
		if got := cssFamily(face); got != want {
			t.Errorf("cssFamily(%q) = %q, want %q", face, got, want)
		}
	}
}
