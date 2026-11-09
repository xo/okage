// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/clip.rs. The
// benchmarks of upstream for other libraries are not ported.

import (
	"testing"

	"github.com/xo/okage"
)

// doClip is do_clip_tiny_skia in tiny-skia benches/src/clip.rs. It makes a
// mask and fills the pixmap through it.
func doClip(b *testing.B, aa bool) {
	b.Helper()

	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	path := okage.PathFromRect(must(okage.RectFromXYWH(0, 0, 1000, 1000)))

	pixmap := must(okage.NewPixmap(1000, 1000))
	for b.Loop() {
		pb := okage.NewPathBuilder()
		pb.PushRect(must(okage.RectFromXYWH(100, 100, 800, 800)))
		pb.PushRect(must(okage.RectFromXYWH(300, 300, 400, 400)))
		clipPath := must(pb.Finish())

		clipPath = must(clipPath.Transform(okage.TransformFromRow(1, -0.5, 0, 1, 0, 300)))

		mask := must(okage.NewMask(1000, 1000))
		mask.FillPath(clipPath, okage.FillRuleEvenOdd, aa, okage.IdentityTransform())

		// Do not use fill_rect, because it is very slow by itself.
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), mask)
	}
}

func BenchmarkClip(b *testing.B) {
	doClip(b, false)
}

func BenchmarkClipAA(b *testing.B) {
	doClip(b, true)
}
