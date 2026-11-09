// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/spiral.rs. The
// benchmarks of upstream for other libraries are not ported. The values of
// CURVES are the same as in benches/src/hairline.rs, so the benchmark uses
// curves of bench_hairline_test.go.

import (
	"testing"

	"github.com/xo/okage"
)

func BenchmarkSpiral(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	path := spiralPath()

	stroke := okage.DefaultStroke()
	pixmap := must(okage.NewPixmap(1000, 1000))
	for b.Loop() {
		pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)
	}
}
