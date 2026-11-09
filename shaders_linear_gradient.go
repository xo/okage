// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// LinearGradient is LinearGradient in tiny-skia
// src/shaders/linear_gradient.rs. It is a linear gradient shader.
type LinearGradient struct {
	base gradient
}

// NewLinearGradient is LinearGradient::new in tiny-skia
// src/shaders/linear_gradient.rs. It creates a new linear gradient shader.
//
// It returns a SolidColor when:
//   - len(stops) == 1
//   - start and end are very close
//
// It returns false when:
//   - stops is empty
//   - start == end
//   - transform is not invertible
func NewLinearGradient(
	start Point,
	end Point,
	stops []GradientStop,
	mode SpreadMode,
	transform Transform,
) (Shader, bool) {
	if len(stops) == 0 {
		return nil, false
	}

	if len(stops) == 1 {
		return SolidColor(stops[0].color), true
	}

	length := end.Sub(start).Length()
	if !isFiniteF32(length) {
		return nil, false
	}

	if path.IsNearlyZeroWithinTolerance(length, degenerateThreshold) {
		// Degenerate gradient, the only tricky complication is when in clamp mode,
		// the limit of the gradient approaches two half planes of solid color
		// (first and last). However, they are divided by the line perpendicular
		// to the start and end point, which becomes undefined once start and end
		// are exactly the same, so just use the end color for a stable solution.

		// Except for special circumstances of clamped gradients,
		// every gradient shape (when degenerate) can be mapped to the same fallbacks.
		// The specific shape factories must account for special clamped conditions separately,
		// this always returns the last color for clamped gradients.
		switch mode {
		case SpreadModePad:
			// Depending on how the gradient shape degenerates,
			// there can be a more specialized fallback representation
			// for the factories to use, but this is a reasonable default.
			return SolidColor(stops[len(stops)-1].color), true
		case SpreadModeReflect, SpreadModeRepeat:
			// repeat and mirror are treated the same: the border colors are never visible,
			// but approximate the final color as infinite repetitions of the colors, so
			// it can be represented as the average color of the gradient.
			return SolidColor(averageGradientColor(stops)), true
		}
	}

	if _, ok := transform.Invert(); !ok {
		return nil, false
	}

	unitTs, ok := pointsToUnitTs(start, end)
	if !ok {
		return nil, false
	}
	return LinearGradient{
		base: newGradient(stops, mode, transform, unitTs),
	}, true
}

// isFiniteF32 is f32::is_finite. It ports no upstream function.
func isFiniteF32(x float32) bool {
	return !math.IsInf(float64(x), 0) && !math.IsNaN(float64(x))
}

// IsOpaque is LinearGradient::is_opaque in tiny-skia
// src/shaders/linear_gradient.rs, and the arm Shader::LinearGradient of
// Shader::is_opaque in tiny-skia src/shaders/mod.rs.
func (g LinearGradient) IsOpaque() bool {
	return g.base.colorsAreOpaque
}

// pushStages is LinearGradient::push_stages in tiny-skia
// src/shaders/linear_gradient.rs.
func (g LinearGradient) pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool {
	return g.base.pushStages(p, cs, func(*rasterPipelineBuilder) {}, func(*rasterPipelineBuilder) {})
}

// pointsToUnitTs is points_to_unit_ts in tiny-skia
// src/shaders/linear_gradient.rs.
func pointsToUnitTs(start, end Point) (Transform, bool) {
	vec := end.Sub(start)
	mag := vec.Length()
	var inv float32
	if mag != 0.0 {
		inv = path.Invert(mag)
	}

	vec.Scale(inv)

	ts := tsFromSinCosAt(-vec.Y, vec.X, start.X, start.Y)
	ts = ts.PostTranslate(-start.X, -start.Y)
	ts = ts.PostScale(inv, inv)
	return ts, true
}

// averageGradientColor is average_gradient_color in tiny-skia
// src/shaders/linear_gradient.rs.
func averageGradientColor(points []GradientStop) Color {
	loadColor := func(c Color) wide.F32x4 {
		return wide.F32x4{c.Red(), c.Green(), c.Blue(), c.Alpha()}
	}

	storeColor := func(c wide.F32x4) Color {
		color, ok := ColorFromRGBA(c[0], c[1], c[2], c[3])
		if !ok {
			panic("called `Option::unwrap()` on a `None` value")
		}
		return color
	}

	if len(points) == 0 {
		panic("assertion failed: !points.is_empty()")
	}

	// The gradient is a piecewise linear interpolation between colors. For a given interval,
	// the integral between the two endpoints is 0.5 * (ci + cj) * (pj - pi), which provides that
	// intervals average color. The overall average color is thus the sum of each piece. The thing
	// to keep in mind is that the provided gradient definition can implicitly use p=0 and p=1.
	blend := wide.SplatF32x4(0.0)

	// Bake 1/(colorCount - 1) uniform stop difference into this scale factor
	wScale := wide.SplatF32x4(0.5)

	for i := range len(points) - 1 {
		// Calculate the average color for the interval between pos(i) and pos(i+1)
		c0 := loadColor(points[i].color)
		c1 := loadColor(points[i+1].color)
		// when pos == null, there are colorCount uniformly distributed stops, going from 0 to 1,
		// so pos[i + 1] - pos[i] = 1/(colorCount-1)
		w := points[i+1].position.Get() - points[i].position.Get()
		blend = blend.Add(wScale.Mul(wide.SplatF32x4(w)).Mul(c1.Add(c0)))
	}

	// Now account for any implicit intervals at the start or end of the stop definitions
	if points[0].position.Get() > 0.0 {
		// The first color is fixed between p = 0 to pos[0], so 0.5 * (ci + cj) * (pj - pi)
		// becomes 0.5 * (c + c) * (pj - 0) = c * pj
		c := loadColor(points[0].color)
		blend = blend.Add(wide.SplatF32x4(points[0].position.Get()).Mul(c))
	}

	lastIdx := len(points) - 1
	if points[lastIdx].position.Get() < 1.0 {
		// The last color is fixed between pos[n-1] to p = 1, so 0.5 * (ci + cj) * (pj - pi)
		// becomes 0.5 * (c + c) * (1 - pi) = c * (1 - pi)
		c := loadColor(points[lastIdx].color)
		blend = blend.Add(wide.SplatF32x4(1.0).Sub(wide.SplatF32x4(points[lastIdx].position.Get())).Mul(c))
	}

	return storeColor(blend)
}

// tsFromSinCosAt is ts_from_sin_cos_at in tiny-skia
// src/shaders/linear_gradient.rs.
func tsFromSinCosAt(sin, cos, px, py float32) Transform {
	cosInv := 1.0 - cos
	return path.TransformFromRow(
		cos,
		sin,
		-sin,
		cos,
		sdot(sin, py, cosInv, px),
		sdot(-sin, px, cosInv, py),
	)
}

// sdot is sdot in tiny-skia src/shaders/linear_gradient.rs.
func sdot(a, b, c, d float32) float32 {
	return float32(a*b) + float32(c*d)
}
