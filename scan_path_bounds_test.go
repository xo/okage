package okage_test

// This file ports no upstream file. It holds the tests of the paths with huge
// coordinates, which panicked before D53.

import (
	"bytes"
	"math"
	"testing"

	"github.com/xo/okage"
)

// polygon returns the closed path through the points xy.
func polygon(t *testing.T, xy ...float32) *okage.Path {
	t.Helper()
	pb := okage.NewPathBuilder()
	pb.MoveTo(xy[0], xy[1])
	for i := 2; i < len(xy); i += 2 {
		pb.LineTo(xy[i], xy[i+1])
	}
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		t.Fatalf("polygon %v has no path", xy)
	}
	return p
}

// TestFillPathHugeQuad fills a quadrilateral with points near 1.5e13 under a
// scale of 0.09. The quadrilateral covers the whole pixmap, so each pixel
// gets the color of the paint. Before D53 the antialiased fill panicked with
// "slice bounds out of range".
func TestFillPathHugeQuad(t *testing.T) {
	p := polygon(t, -1.5e13, -1.5e13, 1.5e13, -1.4e13, 1.6e13, 1.5e13, -1.4e13, 1.6e13)
	// The grid of 8 by 8 samples of D54 shifts the bounds by one more bit.
	for _, tt := range []struct {
		aa       bool
		coverage okage.Coverage
	}{{true, okage.Coverage4x4}, {true, okage.Coverage8x8}, {false, okage.Coverage4x4}} {
		paint := okage.DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 255)
		paint.AntiAlias = tt.aa
		paint.Coverage = tt.coverage
		pixmap := must(okage.NewPixmap(32, 32))
		pixmap.FillPath(p, &paint, okage.FillRuleWinding, okage.TransformFromScale(0.09, 0.09), nil)
		want := must(okage.PremultipliedColorU8FromRGBA(50, 127, 150, 255))
		pixels := pixmap.Pixels()
		for i := range pixels.Len() {
			if got := pixels.At(i); got != want {
				t.Fatalf("anti-alias %v, %v: pixel %d is %v, want %v", tt.aa, tt.coverage, i, got, want)
			}
		}
	}
}

// TestFillPathHugeSameAsSmall compares a fill whose far sides are huge with
// the fill of the same shape whose far sides are just out of the pixmap. The
// far sides are out of the clip in both, so the bytes must be the same.
// Before D53 a far side past 2^29 pixels panicked or left out rows.
func TestFillPathHugeSameAsSmall(t *testing.T) {
	for _, far := range []float32{3e8, 1e10, 1e12, 1.5e13, 1e30} {
		for _, size := range [][2]uint32{{1, 1}, {7, 300}, {300, 7}, {100, 100}, {1000, 200}} {
			w, h := float32(size[0]), float32(size[1])
			// the left part, a horizontal band, and the part under the
			// diagonal y = x
			shapes := [][2][]float32{
				{
					{-far, -far, w*0.4 + 0.5, -far, w*0.4 + 0.5, far, -far, far},
					{-10, -10, w*0.4 + 0.5, -10, w*0.4 + 0.5, h + 10, -10, h + 10},
				},
				{
					{-far, h*0.3 + 0.25, w + far, h*0.3 + 0.25, w + far, h*0.7 + 0.5, -far, h*0.7 + 0.5},
					{-10, h*0.3 + 0.25, w + 10, h*0.3 + 0.25, w + 10, h*0.7 + 0.5, -10, h*0.7 + 0.5},
				},
				{
					{-far, -far, far, far, -far, far},
					{-2048, -2048, 2048, 2048, -2048, 2048},
				},
			}
			if far > 1e12 {
				// The edge clipper finds the point where the diagonal
				// enters the clip in float32, and past 1e12 that point
				// is off by more than a pixel.
				shapes = shapes[:2]
			}
			for i, s := range shapes {
				for _, aa := range []bool{true, false} {
					paint := okage.DefaultPaint()
					paint.SetColorRGBA8(200, 10, 90, 180)
					paint.AntiAlias = aa
					got := must(okage.NewPixmap(size[0], size[1]))
					got.FillPath(polygon(t, s[0]...), &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
					want := must(okage.NewPixmap(size[0], size[1]))
					want.FillPath(polygon(t, s[1]...), &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
					if !bytes.Equal(got.Data(), want.Data()) {
						t.Errorf("far %g, size %v, shape %d, anti-alias %v: the pixmaps differ", far, size, i, aa)
					}
				}
			}
		}
	}
}

// TestStrokePathHuge strokes a horizontal line with ends near 1e13. The
// stroke covers rows 11 to 20 of the pixmap, and no other row. Before D53 the
// antialiased stroke panicked.
func TestStrokePathHuge(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(-1e13, 16/0.09)
	pb.LineTo(1e13, 16/0.09)
	p := must(pb.Finish())
	for _, aa := range []bool{true, false} {
		paint := okage.DefaultPaint()
		paint.AntiAlias = aa
		stroke := okage.DefaultStroke()
		stroke.Width = 10 / 0.09
		pixmap := must(okage.NewPixmap(32, 32))
		pixmap.StrokePath(p, &paint, &stroke, okage.TransformFromScale(0.09, 0.09), nil)
		for y := range 32 {
			for x := range 32 {
				px, _ := pixmap.Pixel(uint32(x), uint32(y))
				got := px.Alpha()
				var want uint8
				if y >= 11 && y < 21 {
					want = 255
				}
				if got != want {
					t.Fatalf("anti-alias %v: pixel (%d, %d) has alpha %d, want %d", aa, x, y, got, want)
				}
			}
		}
	}
}

// FuzzFillPathHuge makes sure that no path makes FillPath or StrokePath
// panic. The coordinates are 10 to the power of an exponent of up to 38, so
// that the fuzzer reaches huge values often. A coordinate can also be NaN
// or an infinity, which Finish rejects (D107).
func FuzzFillPathHuge(f *testing.F) {
	f.Add(-1.5e13, -1.5e13, 1.5e13, -1.4e13, 1.6e13, 1.5e13, -1.4e13, 1.6e13, float32(0.09), float32(1), uint8(0))
	f.Add(-1.98e11, 8.66e10, -8.95e10, -1.47e11, 1.85e10, 1.99e10, -9.99e10, -3.47e10, float32(0.09), float32(3), uint8(1))
	f.Add(-3e12, -6.2e12, 2.3e12, 3.6e11, 0.0, 0.0, 1e3, 1e3, float32(3.6), float32(0), uint8(2))
	f.Add(109.0, 19.0, -15.0, math.NaN(), -6.0, 125.0, -5.0, 228.0, float32(0.5), float32(1), uint8(1))
	f.Fuzz(func(t *testing.T, x0, y0, x1, y1, x2, y2, x3, y3 float64, scale, width float32, mode uint8) {
		xy := []float32{float32(x0), float32(y0), float32(x1), float32(y1), float32(x2), float32(y2), float32(x3), float32(y3)}
		pb := okage.NewPathBuilder()
		pb.MoveTo(xy[0], xy[1])
		switch mode % 3 {
		case 0:
			pb.LineTo(xy[2], xy[3])
			pb.LineTo(xy[4], xy[5])
			pb.LineTo(xy[6], xy[7])
		case 1:
			pb.QuadTo(xy[2], xy[3], xy[4], xy[5])
			pb.LineTo(xy[6], xy[7])
		default:
			pb.CubicTo(xy[2], xy[3], xy[4], xy[5], xy[6], xy[7])
		}
		pb.Close()
		p, ok := pb.Finish()
		if !ok {
			return
		}
		paint := okage.DefaultPaint()
		paint.AntiAlias = mode&4 == 0
		ts := okage.TransformFromScale(scale, scale)
		pixmap := must(okage.NewPixmap(uint32(mode>>3)%64+1, uint32(mode>>3)%37+1))
		pixmap.FillPath(p, &paint, okage.FillRuleEvenOdd, ts, nil)
		stroke := okage.DefaultStroke()
		if !(width >= 0 && width < 1e30) {
			width = 1
		}
		stroke.Width = width
		pixmap.StrokePath(p, &paint, &stroke, ts, nil)
	})
}
