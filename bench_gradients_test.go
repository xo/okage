// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/gradients.rs.
// The benchmarks of upstream for other libraries are not ported.

import (
	"testing"

	"github.com/xo/okage"
)

// gradientPath is the path that each benchmark of this file fills.
func gradientPath() *okage.Path {
	pb := okage.NewPathBuilder()
	pb.MoveTo(60, 60)
	pb.LineTo(160, 940)
	pb.CubicTo(380, 840, 660, 800, 940, 800)
	pb.CubicTo(740, 460, 440, 160, 60, 60)
	pb.Close()
	return must(pb.Finish())
}

// twoStopsLinear is two_stops_linear_tiny_skia in tiny-skia
// benches/src/gradients.rs. It fills a path with a linear gradient.
func twoStopsLinear(b *testing.B, hq bool, points []okage.GradientStop, mode okage.SpreadMode) {
	b.Helper()

	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = hq
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(100, 100),
		okage.PointFromXY(900, 900),
		points,
		mode,
		okage.IdentityTransform(),
	))

	path := gradientPath()

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}

func BenchmarkGradientsTwoStopsLinearPad(b *testing.B) {
	twoStopsLinear(
		b,
		false,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
	)
}

func BenchmarkGradientsTwoStopsLinearReflect(b *testing.B) {
	twoStopsLinear(
		b,
		false,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeReflect,
	)
}

func BenchmarkGradientsTwoStopsLinearRepeat(b *testing.B) {
	twoStopsLinear(
		b,
		false,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeRepeat,
	)
}

func BenchmarkGradientsThreeStopsLinearUneven(b *testing.B) {
	twoStopsLinear(
		b,
		false,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.45, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.66, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		okage.SpreadModePad,
	)
}

func BenchmarkGradientsThreeStopsLinearEven(b *testing.B) {
	twoStopsLinear(
		b,
		false,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.50, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		okage.SpreadModePad,
	)
}

func BenchmarkGradientsTwoStopsLinearPadHQ(b *testing.B) {
	twoStopsLinear(
		b,
		true,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
	)
}

func BenchmarkGradientsTwoStopsLinearReflectHQ(b *testing.B) {
	twoStopsLinear(
		b,
		true,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeReflect,
	)
}

func BenchmarkGradientsTwoStopsLinearRepeatHQ(b *testing.B) {
	twoStopsLinear(
		b,
		true,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeRepeat,
	)
}

func BenchmarkGradientsThreeStopsLinearUnevenHQ(b *testing.B) {
	twoStopsLinear(
		b,
		true,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.45, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.66, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		okage.SpreadModePad,
	)
}

func BenchmarkGradientsThreeStopsLinearEvenHQ(b *testing.B) {
	twoStopsLinear(
		b,
		true,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.50, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		okage.SpreadModePad,
	)
}

// radial fills a path with a radial gradient from start to a circle of
// radius 500 around the point 500, 500.
func radial(b *testing.B, start okage.Point) {
	b.Helper()

	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		start,
		0,
		okage.PointFromXY(500, 500),
		500,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1.00, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := gradientPath()

	pixmap := must(okage.NewPixmap(1000, 1000))

	for b.Loop() {
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}

func BenchmarkGradientsSimpleRadial(b *testing.B) {
	radial(b, okage.PointFromXY(500, 500))
}

func BenchmarkGradientsTwoPointRadial(b *testing.B) {
	radial(b, okage.PointFromXY(400, 400))
}
