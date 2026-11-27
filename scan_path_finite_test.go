package okage_test

// This file ports no upstream file. It holds the tests of the paths with a
// point that is not finite, which panicked before D107.

import (
	"bytes"
	"math"
	"testing"

	"github.com/xo/okage"
)

// TestFillPathNotFinite fills the path of the fuzz test FuzzEncode of ivy,
// whose second point holds a NaN, and a finite path under a transform that
// makes its first point NaN. Before D107 the antialiased fill of the first
// panicked in the alpha runs. Neither one draws a pixel.
func TestFillPathNotFinite(t *testing.T) {
	nan := float32(math.NaN())
	for _, tc := range []struct {
		name  string
		build func(pb *okage.PathBuilder)
		ts    okage.Transform
	}{
		{"ivy", func(pb *okage.PathBuilder) {
			pb.MoveTo(109, 19)
			pb.QuadTo(-15, nan, -6, 125)
			pb.CubicTo(18, 3, -9, 36, -5, 228)
			pb.Close()
		}, okage.TransformFromRow(0.5, 0, 0, 0.5, 2.125, 13.375)},
		{"transform", func(pb *okage.PathBuilder) {
			pb.MoveTo(1e30, 1e30)
			pb.LineTo(0, 0)
			pb.LineTo(10, 0)
			pb.Close()
		}, okage.TransformFromRow(1e10, 0, -1e10, 1, 0, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pb := okage.NewPathBuilder()
			tc.build(pb)
			p, ok := pb.Finish()
			if !ok {
				// Finish rejects the path, so no fill can reach it.
				return
			}
			pixmap := must(okage.NewPixmap(60, 45))
			paint := okage.DefaultPaint()
			paint.AntiAlias = true
			pixmap.FillPath(p, &paint, okage.FillRuleWinding, tc.ts, nil)
			if !bytes.Equal(pixmap.Data(), make([]byte, len(pixmap.Data()))) {
				t.Error("expected no pixel")
			}
		})
	}
}
