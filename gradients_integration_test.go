// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports tiny-skia tests/integration/gradients.rs.

import (
	"testing"

	"github.com/xo/okage"
)

func TestGradientsTwoStopsLinearPadLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-pad-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsLinearRepeatLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(100, 100),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeRepeat,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-repeat-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsLinearReflectLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(100, 100),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeReflect,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-reflect-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsThreeStopsEvenlySpacedLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.50, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		// No need to check other modes. "Two stops" tests will cover them.
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/three-stops-evenly-spaced-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsUnevenlySpacedLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		// No need to check other modes. "Two stops" tests will cover them.
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-unevenly-spaced-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsLinearPadHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-pad-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsLinearRepeatHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(100, 100),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeRepeat,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-repeat-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsLinearReflectHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(100, 100),
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModeReflect,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-linear-reflect-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsThreeStopsEvenlySpacedHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.50, okage.ColorFromRGBA8(220, 140, 75, 180)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(40, 180, 55, 160)),
		},
		// No need to check other modes. "Two stops" tests will cover them.
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/three-stops-evenly-spaced-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsTwoStopsUnevenlySpacedHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewLinearGradient(
		okage.PointFromXY(10, 10),
		okage.PointFromXY(190, 190),
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		// No need to check other modes. "Two stops" tests will cover them.
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/two-stops-unevenly-spaced-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// The radial gradient is only supported by the high quality pipeline.
// Therefore we do not have a lq/hq split.

func TestGradientsWellBehavedRadial(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(120, 80),
		100,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/well-behaved-radial.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsFocalOnCircleRadial(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(120, 80),
		28.29, // This radius forces the required pipeline stage.
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/focal-on-circle-radial.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsConicalGreaterRadial(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(120, 80),
		10, // This radius forces the required pipeline stage.
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(0.75, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/conical-greater-radial.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsSimpleRadialLq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(100, 100),
		100,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1.00, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/simple-radial-lq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsSimpleRadialHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(100, 100),
		100,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1.00, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/simple-radial-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsSimpleRadialWithTsHq(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.ForceHQPipeline = true
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(100, 100),
		100,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1.00, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.TransformFromRow(2, 0.3, -0.7, 1.2, 10.5, -12.3),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/simple-radial-with-ts-hq.png"))
	assertPixmapEqual(t, pixmap, expected)
}

// Gradient doesn't add the Premultiply stage when all stops are opaque.
// But it checks colors only on creation, so we have to recheck them after calling `apply_opacity`.
func TestGradientsGlobalOpacity(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		0,
		okage.PointFromXY(100, 100),
		100,
		[]okage.GradientStop{
			okage.NewGradientStop(0.25, okage.ColorFromRGBA8(50, 127, 150, 255)), // no opacity here
			okage.NewGradientStop(1.00, okage.ColorFromRGBA8(220, 140, 75, 255)), // no opacity here
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))
	paint.Shader = paint.Shader.ApplyOpacity(0.5)

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/global-opacity.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsStripGradient(t *testing.T) {
	// Equal radii, different centers creates a Strip gradient
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(50, 100),
		50,
		okage.PointFromXY(150, 100),
		50,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/strip-gradient.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsConcentricRadial(t *testing.T) {
	// Same center, non-zero start radius (concentric gradient)
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		30,
		okage.PointFromXY(100, 100),
		90,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/concentric-radial.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsConicalSmallerRadial(t *testing.T) {
	// Configuration that triggers XYTo2PtConicalSmaller stage
	// r0=60, r1=30, distance=50
	// r0_norm=1.2, r1_norm=0.6, focal_x = 1.2/0.6 = 2.0 > 1.0
	// self.r1 = 0.6 / |1.0 - 2.0| = 0.6, so is_well_behaved() = false
	// (1.0 - focal_x) < 0.0 is true, triggering XYTo2PtConicalSmaller
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewRadialGradient(
		okage.PointFromXY(100, 100),
		60,
		okage.PointFromXY(150, 100),
		30,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	expected := check(okage.LoadPNG("testdata/images/gradients/conical-smaller-radial.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsSweepGradient(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewSweepGradient(
		okage.PointFromXY(100, 100),
		135,
		225,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(
		path,
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/gradients/sweep-gradient.png"))
	assertPixmapEqual(t, pixmap, expected)
}

func TestGradientsSweepGradientFull(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	paint.Shader = must(okage.NewSweepGradient(
		okage.PointFromXY(100, 100),
		0,
		360,
		[]okage.GradientStop{
			okage.NewGradientStop(0, okage.ColorFromRGBA8(50, 127, 150, 200)),
			okage.NewGradientStop(1, okage.ColorFromRGBA8(220, 140, 75, 180)),
		},
		okage.SpreadModePad,
		okage.IdentityTransform(),
	))

	path := okage.PathFromRect(must(okage.RectFromLTRB(10, 10, 190, 190)))

	pixmap := must(okage.NewPixmap(200, 200))
	pixmap.FillPath(
		path,
		&paint,
		okage.FillRuleWinding,
		okage.IdentityTransform(),
		nil,
	)

	expected := check(okage.LoadPNG("testdata/images/gradients/sweep-gradient-full.png"))
	assertPixmapEqual(t, pixmap, expected)
}
