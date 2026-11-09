// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/fill.rs. The
// benchmarks of upstream for other libraries are not ported.

import (
	"testing"

	"github.com/xo/okage"
)

func BenchmarkFillRect(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = false

	rect := must(okage.RectFromXYWH(50.7, 20.1, 812.4, 777.3))

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
	}
}

func BenchmarkFillRectAA(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(50.7, 20.1, 812.4, 777.3))

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillRect(rect, &paint, okage.IdentityTransform(), nil)
	}
}

func BenchmarkFillRectAATs(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

	rect := must(okage.RectFromXYWH(200.3, 100.4, 500.5, 300.2))

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillRect(rect, &paint, okage.TransformFromRow(1.8, 0.3, -0.7, 0.8, 12, 15.3), nil)
	}
}

func BenchmarkFillAll(b *testing.B) {
	pixmap := must(okage.NewPixmap(1000, 1000))
	c := okage.ColorFromRGBA8(50, 100, 150, 200)
	for b.Loop() {
		pixmap.Fill(c)
	}
}

func BenchmarkFillPathAA(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.AntiAlias = true

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

// Filling a semi-transparent rectangle path with a Source blending mode.
// By using this blending mode we're forcing a simple pixels overwrite.
func BenchmarkFillSource(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.BlendMode = okage.BlendModeSource
	paint.AntiAlias = false

	path := okage.PathFromRect(must(okage.RectFromLTRB(100, 100, 900, 900)))

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}

// Filling an opaque rectangle path.
// A chosen blending mode doesn't really matter in this case,
// since we are simply overwriting the pixels.
func BenchmarkFillOpaque(b *testing.B) {
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 255)
	paint.AntiAlias = false

	path := okage.PathFromRect(must(okage.RectFromLTRB(100, 100, 900, 900)))

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}
