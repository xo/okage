package okage

// This file ports no upstream file. It tests that a hairline makes no
// allocation for each segment, each curve or each clipped line (D35).

import (
	"runtime"
	"testing"

	"github.com/xo/okage/path"
)

// TestHairlineAllocs strokes a path with many lines and curves, some of them
// horizontal and some of them across the edge of the pixmap. It must make as
// many allocations as a path with one line.
func TestHairlineAllocs(t *testing.T) {
	if runtime.Compiler == "tinygo" {
		t.Skip("skipping: TinyGo keeps less on the stack")
	}

	one := path.NewPathBuilder()
	one.MoveTo(10, 10)
	one.LineTo(20, 30)
	short, ok := one.Finish()
	if !ok {
		t.Fatal("cannot make the short path")
	}

	pb := path.NewPathBuilder()
	for i := range 20 {
		f := float32(i)
		pb.MoveTo(-20, 5+f*4)
		pb.LineTo(120, 5+f*4)
		pb.MoveTo(-10+f, -10)
		pb.CubicTo(200, 30+f, -100, 60, 50+f, 130)
		pb.QuadTo(130, 50, 20+f, 40)
		pb.LineTo(80, 41+f)
		pb.Close()
	}
	long, ok := pb.Finish()
	if !ok {
		t.Fatal("cannot make the long path")
	}

	pixmap, ok := NewPixmap(100, 100)
	if !ok {
		t.Fatal("cannot make the pixmap")
	}
	// A width of 0 is a hairline with and without anti-alias.
	stroke := DefaultStroke()
	stroke.Width = 0

	for _, aa := range []bool{false, true} {
		paint := DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 200)
		paint.AntiAlias = aa
		allocs := func(p *Path) float64 {
			return testing.AllocsPerRun(5, func() {
				pixmap.StrokePath(p, &paint, &stroke, IdentityTransform(), nil)
			})
		}
		if got, want := allocs(long), allocs(short); got > want {
			t.Errorf("anti-alias %v: got %v allocations, want at most %v", aa, got, want)
		}
	}
}
