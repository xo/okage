// Copyright 2016 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "testing"

// TestBlend is the module blend_tests in tiny-skia src/pipeline/mod.rs. It
// tests the blending modes.
//
// Skia has two kinds of a raster pipeline: high and low precision. "High"
// uses f32 and "low" uses u16. And for basic operations we do not need f32,
// and u16 is simply faster. But those modes are not identical. They can
// produce slightly different results due to rounding.
//
// Port: each call of the macros test_blend_lowp! and test_blend_highp! is a
// row of the table. ColorDodge, ColorBurn, SoftLight, Hue, Saturation, Color
// and Luminosity are not available for lowp. Source is a no-op.
func TestBlend(t *testing.T) {
	tests := []struct {
		name       string
		mode       BlendMode
		isHighp    bool
		r, g, b, a uint8
	}{
		{"clear_lowp", BlendModeClear, false, 0, 0, 0, 0},
		{"destination_lowp", BlendModeDestination, false, 39, 100, 118, 200},
		{"source_over_lowp", BlendModeSourceOver, false, 167, 129, 88, 239},
		{"destination_over_lowp", BlendModeDestinationOver, false, 73, 122, 130, 239},
		{"source_in_lowp", BlendModeSourceIn, false, 122, 78, 42, 141},
		{"destination_in_lowp", BlendModeDestinationIn, false, 28, 71, 83, 141},
		{"source_out_lowp", BlendModeSourceOut, false, 34, 22, 12, 39},
		{"destination_out_lowp", BlendModeDestinationOut, false, 12, 30, 35, 59},
		{"source_atop_lowp", BlendModeSourceAtop, false, 133, 107, 76, 200},
		{"destination_atop_lowp", BlendModeDestinationAtop, false, 61, 92, 95, 180},
		{"xor_lowp", BlendModeXor, false, 45, 51, 46, 98},
		{"plus_lowp", BlendModePlus, false, 194, 199, 171, 255},
		{"modulate_lowp", BlendModeModulate, false, 24, 39, 25, 141},
		{"screen_lowp", BlendModeScreen, false, 170, 160, 146, 239},
		{"overlay_lowp", BlendModeOverlay, false, 92, 128, 106, 239},
		{"darken_lowp", BlendModeDarken, false, 72, 121, 88, 239},
		{"lighten_lowp", BlendModeLighten, false, 166, 128, 129, 239},
		{"hard_light_lowp", BlendModeHardLight, false, 154, 128, 95, 239},
		{"difference_lowp", BlendModeDifference, false, 138, 57, 87, 239},
		{"exclusion_lowp", BlendModeExclusion, false, 146, 121, 121, 239},
		{"multiply_lowp", BlendModeMultiply, false, 69, 90, 71, 238},
		{"clear_highp", BlendModeClear, true, 0, 0, 0, 0},
		{"destination_highp", BlendModeDestination, true, 39, 100, 118, 200},
		{"source_over_highp", BlendModeSourceOver, true, 167, 128, 88, 239},
		{"destination_over_highp", BlendModeDestinationOver, true, 72, 121, 129, 239},
		{"source_in_highp", BlendModeSourceIn, true, 122, 78, 42, 141},
		{"destination_in_highp", BlendModeDestinationIn, true, 28, 71, 83, 141},
		{"source_out_highp", BlendModeSourceOut, true, 33, 21, 11, 39},
		{"destination_out_highp", BlendModeDestinationOut, true, 11, 29, 35, 59},
		{"source_atop_highp", BlendModeSourceAtop, true, 133, 107, 76, 200},
		{"destination_atop_highp", BlendModeDestinationAtop, true, 61, 92, 95, 180},
		{"xor_highp", BlendModeXor, true, 45, 51, 46, 98},
		{"plus_highp", BlendModePlus, true, 194, 199, 171, 255},
		{"modulate_highp", BlendModeModulate, true, 24, 39, 24, 141},
		{"screen_highp", BlendModeScreen, true, 171, 160, 146, 239},
		{"overlay_highp", BlendModeOverlay, true, 92, 128, 106, 239},
		{"darken_highp", BlendModeDarken, true, 72, 121, 88, 239},
		{"lighten_highp", BlendModeLighten, true, 167, 128, 129, 239},
		{"color_dodge_highp", BlendModeColorDodge, true, 186, 192, 164, 239},
		{"color_burn_highp", BlendModeColorBurn, true, 54, 63, 46, 239},
		{"hard_light_highp", BlendModeHardLight, true, 155, 128, 95, 239},
		{"soft_light_highp", BlendModeSoftLight, true, 98, 124, 115, 239},
		{"difference_highp", BlendModeDifference, true, 139, 58, 88, 239},
		{"exclusion_highp", BlendModeExclusion, true, 147, 121, 122, 239},
		{"multiply_highp", BlendModeMultiply, true, 69, 89, 71, 239},
		{"hue_highp", BlendModeHue, true, 128, 103, 74, 239},
		{"saturation_highp", BlendModeSaturation, true, 59, 126, 140, 239},
		{"color_highp", BlendModeColor, true, 139, 100, 60, 239},
		{"luminosity_highp", BlendModeLuminosity, true, 100, 149, 157, 239},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixmap, ok := NewPixmap(1, 1)
			if !ok {
				t.Fatal("NewPixmap failed")
			}
			pixmap.Fill(ColorFromRGBA8(50, 127, 150, 200))

			pixmapSrc, ok := PixmapRefFromBytes([]byte{0, 0, 0, 0}, 1, 1)
			if !ok {
				t.Fatal("PixmapRefFromBytes failed")
			}

			p := newRasterPipelineBuilder()
			p.setForceHqPipeline(tt.isHighp)
			p.pushUniformColor(ColorFromRGBA8(220, 140, 75, 180).Premultiply())
			p.push(stageLoadDestination)
			s, ok := tt.mode.toStage()
			if !ok {
				t.Fatal("toStage failed")
			}
			p.push(s)
			p.push(stageStore)
			rp := p.compile()
			rect := intSizeToScreenIntRect(pixmap.Size(), 0, 0)
			m := pixmap.AsMut()
			sub := m.asSubpixmap()
			rp.run(&rect, aaMaskCtx{}, maskCtx{}, pixmapSrc, &sub)

			got, ok := pixmap.AsRef().Pixel(0, 0)
			if !ok {
				t.Fatal("Pixel failed")
			}
			want, ok := PremultipliedColorU8FromRGBA(tt.r, tt.g, tt.b, tt.a)
			if !ok {
				t.Fatal("PremultipliedColorU8FromRGBA failed")
			}
			if got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}
