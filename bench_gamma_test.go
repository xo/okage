package okage_test

// This file holds benchmarks of the stages of the color spaces, which no
// benchmark of tiny-skia covers (D96). It ports no upstream file.

import (
	"testing"

	"github.com/xo/okage"
)

// BenchmarkGamma fills a rect with a color that is not opaque in each color
// space that has stages: the stages expand the source and the destination,
// and compress the result.
func BenchmarkGamma(b *testing.B) {
	for _, cs := range []okage.ColorSpace{
		okage.ColorSpaceGamma2,
		okage.ColorSpaceSimpleSRGB,
		okage.ColorSpaceFullSRGBGamma,
	} {
		b.Run(cs.String(), func(b *testing.B) {
			paint := okage.DefaultPaint()
			paint.SetColorRGBA8(50, 127, 150, 200)
			paint.AntiAlias = false
			paint.Colorspace = cs

			rect := must(okage.RectFromXYWH(50.7, 20.1, 812.4, 777.3))

			pixmap := must(okage.NewPixmap(1000, 1000))
			pixmap.Fill(okage.ColorFromRGBA8(220, 140, 75, 180))

			for b.Loop() {
				pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
			}
		})
	}
}
