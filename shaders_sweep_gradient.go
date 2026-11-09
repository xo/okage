// Copyright 2006 The Android Open Source Project
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"github.com/xo/okage/path"
)

// SweepGradient is SweepGradient in tiny-skia src/shaders/sweep_gradient.rs.
// It is a sweep gradient shader.
type SweepGradient struct {
	base gradient
	t0   float32
	t1   float32
}

// NewSweepGradient is SweepGradient::new in tiny-skia
// src/shaders/sweep_gradient.rs. It creates a new sweep gradient shader.
//
// It returns false when an angle is not finite, when startAngle is larger
// than endAngle, when stops is empty, when transform is not invertible, and
// for most degenerate gradients.
func NewSweepGradient(
	center Point,
	startAngle float32,
	endAngle float32,
	stops []GradientStop,
	mode SpreadMode,
	transform Transform,
) (Shader, bool) {
	if !isFiniteF32(startAngle) || !isFiniteF32(endAngle) || startAngle > endAngle {
		return nil, false
	}

	switch len(stops) {
	case 0:
		return nil, false
	case 1:
		return SolidColor(stops[0].color), true
	}
	if _, ok := transform.Invert(); !ok {
		return nil, false
	}
	if path.IsNearlyEqualWithinTolerance(startAngle, endAngle, degenerateThreshold) {
		if mode == SpreadModePad && endAngle > degenerateThreshold {
			// In this case, the first color is repeated from 0 to the angle, then a hardstop
			// switches to the last color (all other colors are compressed to the infinitely
			// thin interpolation region).
			frontColor := stops[0].color
			backColor := stops[len(stops)-1].color
			newStops := []GradientStop{
				NewGradientStop(0.0, frontColor),
				NewGradientStop(1.0, frontColor),
				NewGradientStop(1.0, backColor),
			}
			return NewSweepGradient(center, 0.0, endAngle, newStops, mode, transform)
		}
		// TODO: Consider making a degenerate fallback shader similar to Skia. Tiny Skia
		// currently opts to return `None` in some places.
		return nil, false
	}
	if startAngle <= 0.0 && endAngle >= 360.0 {
		mode = SpreadModePad
	}
	t0 := startAngle / 360.0
	t1 := endAngle / 360.0
	return SweepGradient{
		base: newGradient(
			stops,
			mode,
			transform,
			path.TransformFromTranslate(-center.X, -center.Y),
		),
		t0: t0,
		t1: t1,
	}, true
}

// IsOpaque is SweepGradient::is_opaque in tiny-skia
// src/shaders/sweep_gradient.rs, and the arm Shader::SweepGradient of
// Shader::is_opaque in tiny-skia src/shaders/mod.rs.
func (g SweepGradient) IsOpaque() bool {
	return g.base.colorsAreOpaque
}

// pushStages is SweepGradient::push_stages in tiny-skia
// src/shaders/sweep_gradient.rs.
func (g SweepGradient) pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool {
	scale := 1.0 / (g.t1 - g.t0)
	bias := -scale * g.t0
	p.ctx.twoPointConicalGradient.p0 = scale
	p.ctx.twoPointConicalGradient.p1 = bias
	return g.base.pushStages(
		p,
		cs,
		func(p *rasterPipelineBuilder) {
			p.push(stageXYToUnitAngle)
			if scale != 1.0 && bias != 0.0 {
				p.push(stageApplyConcentricScaleBias)
			}
		},
		func(*rasterPipelineBuilder) {},
	)
}
