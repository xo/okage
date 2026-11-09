// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/patterns.rs. The
// benchmarks of upstream for other libraries are not ported.

import (
	"testing"

	"github.com/xo/okage"
)

// fillPattern is pattern_tiny_skia in tiny-skia benches/src/patterns.rs. It
// fills a path with a pattern of a small triangle. Its inner function
// crate_triangle is the same as the one of tests/integration/pattern.rs, so
// it calls crateTriangle of pattern_integration_test.go.
func fillPattern(b *testing.B, quality okage.FilterQuality, ts okage.Transform) {
	b.Helper()

	pixmap := must(okage.NewPixmap(1000, 1000))
	triangle := crateTriangle()

	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	paint.Shader = okage.NewPattern(
		triangle.AsRef(),
		okage.SpreadModeRepeat,
		quality,
		1,
		ts,
	)

	pb := okage.NewPathBuilder()
	pb.MoveTo(60, 60)
	pb.LineTo(160, 940)
	pb.CubicTo(380, 840, 660, 800, 940, 800)
	pb.CubicTo(740, 460, 440, 160, 60, 60)
	pb.Close()
	path := must(pb.Finish())

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}

func BenchmarkPatternsPlain(b *testing.B) {
	fillPattern(
		b,
		okage.FilterQualityNearest,
		okage.IdentityTransform(),
	)
}

func BenchmarkPatternsLQ(b *testing.B) {
	fillPattern(
		b,
		okage.FilterQualityBilinear,
		okage.TransformFromRow(1.5, -0.4, 0, -0.8, 5, 1),
	)
}

func BenchmarkPatternsHQ(b *testing.B) {
	fillPattern(
		b,
		okage.FilterQualityBicubic,
		okage.TransformFromRow(1.5, -0.4, 0, -0.8, 5, 1),
	)
}
