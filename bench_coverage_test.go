package okage_test

// This file ports no upstream benchmark. It measures Paint.Coverage and
// Paint.StrokeAdjust (D54).

import (
	"testing"

	"github.com/xo/okage"
)

// BenchmarkFillPathAA8x8 is BenchmarkFillPathAA with Coverage8x8.
func BenchmarkFillPathAA8x8(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.Coverage = okage.Coverage8x8

	pb := okage.NewPathBuilder()
	pb.MoveTo(500, 20)
	pb.CubicTo(650, 320, 770, 650, 800, 980)
	pb.LineTo(20, 380)
	pb.LineTo(200, 980)
	pb.CubicTo(230, 650, 350, 320, 500, 20)
	pb.Close()
	path := must(pb.Finish())

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)
	}
}

// fillGlyphs fills 400 small shapes, about the size of the glyphs of text
// at 96 dpi, with the coverage c.
func fillGlyphs(b *testing.B, c okage.Coverage) {
	b.Helper()
	paint := okage.DefaultPaint()
	paint.Coverage = c

	pb := okage.NewPathBuilder()
	pb.MoveTo(1.2, 9.7)
	pb.CubicTo(1.2, 4.1, 3.3, 0.6, 5.1, 0.6)
	pb.CubicTo(6.9, 0.6, 8.4, 3.0, 8.4, 6.2)
	pb.LineTo(6.8, 6.2)
	pb.CubicTo(6.8, 3.9, 6.1, 2.1, 5.0, 2.1)
	pb.CubicTo(3.6, 2.1, 2.8, 5.3, 2.8, 9.7)
	pb.Close()
	pb.PushCircle(5, 5.5, 1.3)
	glyph := must(pb.Finish())

	pixmap := must(okage.NewPixmap(400, 240))

	for b.Loop() {
		for y := range 20 {
			for x := range 40 {
				ts := okage.TransformFromTranslate(float32(x)*9.7+0.3, float32(y)*11.9+0.6)
				pixmap.FillPath(glyph, &paint, okage.FillRuleWinding, ts, nil)
			}
		}
	}
}

func BenchmarkFillGlyphs4x4(b *testing.B) { fillGlyphs(b, okage.Coverage4x4) }
func BenchmarkFillGlyphs8x8(b *testing.B) { fillGlyphs(b, okage.Coverage8x8) }

// BenchmarkStrokeAdjust strokes the grid of thin lines of a plan, with
// stroke adjustment.
func BenchmarkStrokeAdjust(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.StrokeAdjust = true

	pb := okage.NewPathBuilder()
	for i := range 100 {
		v := float32(i)*9.7 + 0.3
		pb.MoveTo(v, 0)
		pb.LineTo(v, 1000)
		pb.MoveTo(0, v)
		pb.LineTo(1000, v)
	}
	path := must(pb.Finish())
	stroke := okage.DefaultStroke()
	stroke.Width = 0.4

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)
	}
}
