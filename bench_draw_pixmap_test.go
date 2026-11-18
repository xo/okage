package okage_test

// This file ports no upstream file. It holds the benchmarks of DrawPixmap
// for an image that is aligned to the pixels, which D53 makes faster.

import (
	"testing"

	"github.com/xo/okage"
)

// The size of the page of the benchmarks, the size of a page of a PDF file
// that ivy renders.
const (
	benchPageWidth  = 1600
	benchPageHeight = 893
)

// benchLayer returns a pixmap of the size of the page with circles of
// colors that are not opaque, as a transparency group of a PDF file has.
func benchLayer(b *testing.B) *okage.Pixmap {
	b.Helper()
	layer := must(okage.NewPixmap(benchPageWidth, benchPageHeight))
	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	for i := range 40 {
		paint.SetColorRGBA8(uint8(i*37), uint8(255-i*11), uint8(i*91), uint8(80+i*4))
		circle := must(okage.PathFromCircle(float32(i*41%benchPageWidth), float32(i*67%benchPageHeight), float32(60+i*7)))
		layer.FillPath(circle, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
	return layer
}

// benchOpaque returns an opaque pixmap of the size of the page, as a page of
// a DjVu file is.
func benchOpaque(b *testing.B) *okage.Pixmap {
	b.Helper()
	pixmap := must(okage.NewPixmap(benchPageWidth, benchPageHeight))
	data := pixmap.DataMut()
	for i := 0; i < len(data); i += okage.BytesPerPixel {
		v := uint8(i / okage.BytesPerPixel * 7)
		data[i], data[i+1], data[i+2], data[i+3] = v, v/2, 255-v, 255
	}
	return pixmap
}

// benchMask returns a soft mask of the size of the page: an antialiased
// circle inside a rectangle.
func benchMask(b *testing.B) *okage.Mask {
	b.Helper()
	mask := must(okage.NewMask(benchPageWidth, benchPageHeight))
	circle := must(okage.PathFromCircle(benchPageWidth/2, benchPageHeight/2, benchPageHeight/2-10))
	mask.FillPath(circle, okage.FillRuleWinding, true, okage.IdentityTransform())
	return mask
}

// drawPixmap draws src onto a page with paint, ts and mask, once for each
// iteration of the benchmark.
func drawPixmap(b *testing.B, src *okage.Pixmap, paint okage.PixmapPaint, ts okage.Transform, mask *okage.Mask) {
	b.Helper()
	dst := benchOpaque(b)
	b.ResetTimer()
	for b.Loop() {
		dst.DrawPixmap(0, 0, src.AsRef(), &paint, ts, mask)
	}
}

// BenchmarkDrawPixmapLayer draws a transparency group onto a page through a
// soft mask, with an opacity of 0.5, as ivy does at the end of a group of a
// PDF file.
func BenchmarkDrawPixmapLayer(b *testing.B) {
	paint := okage.DefaultPixmapPaint()
	paint.Opacity = 0.5
	drawPixmap(b, benchLayer(b), paint, okage.IdentityTransform(), benchMask(b))
}

// BenchmarkDrawPixmapLayerMultiply is BenchmarkDrawPixmapLayer with the
// blend mode Multiply.
func BenchmarkDrawPixmapLayerMultiply(b *testing.B) {
	paint := okage.DefaultPixmapPaint()
	paint.Opacity = 0.5
	paint.BlendMode = okage.BlendModeMultiply
	drawPixmap(b, benchLayer(b), paint, okage.IdentityTransform(), benchMask(b))
}

// BenchmarkDrawPixmapLayerNoMask draws a transparency group onto a page with
// the default paint.
func BenchmarkDrawPixmapLayerNoMask(b *testing.B) {
	drawPixmap(b, benchLayer(b), okage.DefaultPixmapPaint(), okage.IdentityTransform(), nil)
}

// BenchmarkDrawPixmapOpaque draws an opaque image onto a page, moved by
// whole pixels, as ivy draws a page of a DjVu file.
func BenchmarkDrawPixmapOpaque(b *testing.B) {
	drawPixmap(b, benchOpaque(b), okage.DefaultPixmapPaint(), okage.TransformFromTranslate(3, -5), nil)
}

// BenchmarkDrawPixmapScaled draws an opaque image onto a page with a scale.
// The image is not aligned to the pixels, so it takes the pipeline.
func BenchmarkDrawPixmapScaled(b *testing.B) {
	drawPixmap(b, benchOpaque(b), okage.DefaultPixmapPaint(), okage.TransformFromScale(1.01, 0.99), nil)
}
