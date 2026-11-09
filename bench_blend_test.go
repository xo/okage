// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage_test

// This file ports the benchmarks of tiny-skia in benches/src/blend.rs. The
// benchmarks of upstream for Skia, cairo and raqote are not ported, because
// okage compares only with itself here (D22).

import (
	"testing"

	"github.com/xo/okage"
)

// fillBlend is fill_tiny_skia in tiny-skia benches/src/blend.rs. It fills a
// path with blendMode over another path.
func fillBlend(b *testing.B, blendMode okage.BlendMode) {
	b.Helper()

	paint1 := okage.DefaultPaint()
	paint1.SetColorRGBA8(50, 127, 150, 200)
	paint1.BlendMode = okage.BlendModeSourceOver
	paint1.AntiAlias = false

	paint2 := okage.DefaultPaint()
	paint2.SetColorRGBA8(220, 140, 75, 180)
	paint2.BlendMode = blendMode // <-- variable
	paint2.AntiAlias = false

	pb := okage.NewPathBuilder()
	pb.MoveTo(60, 60)
	pb.LineTo(160, 940)
	pb.CubicTo(380, 840, 660, 800, 940, 800)
	pb.CubicTo(740, 460, 440, 160, 60, 60)
	pb.Close()
	path1 := must(pb.Finish())

	pb = okage.NewPathBuilder()
	pb.MoveTo(940, 60)
	pb.LineTo(840, 940)
	pb.CubicTo(620, 840, 340, 800, 60, 800)
	pb.CubicTo(260, 460, 560, 160, 940, 60)
	pb.Close()
	path2 := must(pb.Finish())

	pixmap := must(okage.NewPixmap(1000, 1000))
	pixmap.FillPath(path1, &paint1, okage.FillRuleWinding, okage.IdentityTransform(), nil)

	for b.Loop() {
		pixmap.FillPath(path2, &paint2, okage.FillRuleWinding, okage.IdentityTransform(), nil)
	}
}

func BenchmarkBlendClear(b *testing.B)           { fillBlend(b, okage.BlendModeClear) }
func BenchmarkBlendSource(b *testing.B)          { fillBlend(b, okage.BlendModeSource) }
func BenchmarkBlendDestination(b *testing.B)     { fillBlend(b, okage.BlendModeDestination) }
func BenchmarkBlendSourceOver(b *testing.B)      { fillBlend(b, okage.BlendModeSourceOver) }
func BenchmarkBlendDestinationOver(b *testing.B) { fillBlend(b, okage.BlendModeDestinationOver) }
func BenchmarkBlendSourceIn(b *testing.B)        { fillBlend(b, okage.BlendModeSourceIn) }
func BenchmarkBlendDestinationIn(b *testing.B)   { fillBlend(b, okage.BlendModeDestinationIn) }
func BenchmarkBlendSourceOut(b *testing.B)       { fillBlend(b, okage.BlendModeSourceOut) }
func BenchmarkBlendDestinationOut(b *testing.B)  { fillBlend(b, okage.BlendModeDestinationOut) }
func BenchmarkBlendSourceAtop(b *testing.B)      { fillBlend(b, okage.BlendModeSourceAtop) }
func BenchmarkBlendDestinationAtop(b *testing.B) { fillBlend(b, okage.BlendModeDestinationAtop) }
func BenchmarkBlendXor(b *testing.B)             { fillBlend(b, okage.BlendModeXor) }
func BenchmarkBlendPlus(b *testing.B)            { fillBlend(b, okage.BlendModePlus) }
func BenchmarkBlendModulate(b *testing.B)        { fillBlend(b, okage.BlendModeModulate) }
func BenchmarkBlendScreen(b *testing.B)          { fillBlend(b, okage.BlendModeScreen) }
func BenchmarkBlendOverlay(b *testing.B)         { fillBlend(b, okage.BlendModeOverlay) }
func BenchmarkBlendDarken(b *testing.B)          { fillBlend(b, okage.BlendModeDarken) }
func BenchmarkBlendLighten(b *testing.B)         { fillBlend(b, okage.BlendModeLighten) }
func BenchmarkBlendColorDodge(b *testing.B)      { fillBlend(b, okage.BlendModeColorDodge) }
func BenchmarkBlendColorBurn(b *testing.B)       { fillBlend(b, okage.BlendModeColorBurn) }
func BenchmarkBlendHardLight(b *testing.B)       { fillBlend(b, okage.BlendModeHardLight) }
func BenchmarkBlendSoftLight(b *testing.B)       { fillBlend(b, okage.BlendModeSoftLight) }
func BenchmarkBlendDifference(b *testing.B)      { fillBlend(b, okage.BlendModeDifference) }
func BenchmarkBlendExclusion(b *testing.B)       { fillBlend(b, okage.BlendModeExclusion) }
func BenchmarkBlendMultiply(b *testing.B)        { fillBlend(b, okage.BlendModeMultiply) }
func BenchmarkBlendHue(b *testing.B)             { fillBlend(b, okage.BlendModeHue) }
func BenchmarkBlendSaturation(b *testing.B)      { fillBlend(b, okage.BlendModeSaturation) }
func BenchmarkBlendColor(b *testing.B)           { fillBlend(b, okage.BlendModeColor) }
func BenchmarkBlendLuminosity(b *testing.B)      { fillBlend(b, okage.BlendModeLuminosity) }
