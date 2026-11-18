package okage_test

// This file ports no upstream test. It checks that a hairline draws on a
// pixmap that is 2 pixels wide or high, or less, where tiny-skia draws
// nothing (D54).

import (
	"bytes"
	"testing"

	"github.com/xo/okage"
)

// TestHairlineOnANarrowPixmap checks that a hairline on a pixmap of 1 or 2
// rows, or of 1 or 2 columns, gives the pixels that the same hairline gives
// on a larger pixmap.
func TestHairlineOnANarrowPixmap(t *testing.T) {
	type line struct {
		name  string
		build func(pb *okage.PathBuilder)
	}
	lines := []line{
		{"horizontal", func(pb *okage.PathBuilder) { pb.MoveTo(-5, 0.6); pb.LineTo(15, 0.6) }},
		{"diagonal", func(pb *okage.PathBuilder) { pb.MoveTo(-5, 0.2); pb.LineTo(15, 1.6) }},
		{"quad", func(pb *okage.PathBuilder) { pb.MoveTo(-5, 0.5); pb.QuadTo(5, 1.5, 15, 0.5) }},
		{"cubic", func(pb *okage.PathBuilder) { pb.MoveTo(-5, 0.5); pb.CubicTo(0, 2, 10, -1, 15, 1.5) }},
	}
	for _, l := range lines {
		for _, aa := range []bool{true, false} {
			for _, width := range []float32{0, 0.5} {
				if !aa && width != 0 {
					// Without antialiasing, only a width of 0 is a hairline.
					continue
				}
				for _, rows := range []uint32{1, 2} {
					for _, vertical := range []bool{false, true} {
						pb := okage.NewPathBuilder()
						l.build(pb)
						p := must(pb.Finish())
						if vertical {
							// Swap x and y, so the line runs down.
							p = must(p.Transform(okage.TransformFromRow(0, 1, 1, 0, 0, 0)))
						}
						paint := okage.DefaultPaint()
						paint.AntiAlias = aa
						stroke := okage.DefaultStroke()
						stroke.Width = width
						w, h := uint32(10), rows
						bw, bh := w, uint32(10)
						if vertical {
							w, h, bw, bh = h, w, bh, bw
						}
						small := must(okage.NewPixmap(w, h))
						small.StrokePath(p, &paint, &stroke, okage.IdentityTransform(), nil)
						big := must(okage.NewPixmap(bw, bh))
						big.StrokePath(p, &paint, &stroke, okage.IdentityTransform(), nil)
						var want []byte
						for y := range h {
							want = append(want, big.Data()[4*y*bw:4*(y*bw+w)]...)
						}
						if bytes.Count(want, []byte{0}) == len(want) {
							t.Fatalf("%s aa=%v width=%v rows=%d vertical=%v: the larger pixmap is empty", l.name, aa, width, rows, vertical)
						}
						if !bytes.Equal(small.Data(), want) {
							t.Errorf("%s aa=%v width=%v rows=%d vertical=%v: expected the pixels of the larger pixmap\n got: %v\nwant: %v",
								l.name, aa, width, rows, vertical, small.Data(), want)
						}
					}
				}
			}
		}
	}
}
