// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"slices"

	"github.com/xo/okage/path"
)

// degenerateThreshold is DEGENERATE_THRESHOLD in tiny-skia
// src/shaders/gradient.rs.
//
// The default SCALAR_NEARLY_ZERO threshold of .0024 is too big and causes
// regressions for svg gradients defined in the wild.
const degenerateThreshold float32 = 1.0 / (1 << 15)

// GradientStop is GradientStop in tiny-skia src/shaders/gradient.rs. It is a
// gradient point.
type GradientStop struct {
	position path.NormalizedF32
	color    Color
}

// NewGradientStop is GradientStop::new in tiny-skia src/shaders/gradient.rs.
// It creates a new gradient point.
//
// position is clamped to a 0..=1 range.
func NewGradientStop(position float32, color Color) GradientStop {
	return GradientStop{
		position: path.NormalizedF32Clamped(position),
		color:    color,
	}
}

// gradient is Gradient in tiny-skia src/shaders/gradient.rs.
type gradient struct {
	stops           []GradientStop
	tileMode        SpreadMode
	transform       Transform
	pointsToUnit    Transform
	colorsAreOpaque bool
	hasUniformStops bool
}

// newGradient is Gradient::new in tiny-skia src/shaders/gradient.rs.
//
// Port: Rust takes ownership of stops and changes it. Go copies stops first,
// so that the slice of the caller does not change.
func newGradient(
	stops []GradientStop,
	tileMode SpreadMode,
	transform Transform,
	pointsToUnit Transform,
) gradient {
	stops = slices.Clone(stops)

	// Note: we let the caller skip the first and/or last position.
	// i.e. pos[0] = 0.3, pos[1] = 0.7
	// In these cases, we insert dummy entries to ensure that the final data
	// is bracketed by [0, 1].
	// i.e. our_pos[0] = 0, our_pos[1] = 0.3, our_pos[2] = 0.7, our_pos[3] = 1
	dummyFirst := stops[0].position.Get() != 0.0
	dummyLast := stops[len(stops)-1].position.Get() != 1.0

	// Now copy over the colors, adding the dummies as needed.
	if dummyFirst {
		stops = slices.Insert(stops, 0, NewGradientStop(0.0, stops[0].color))
	}

	if dummyLast {
		stops = append(stops, NewGradientStop(1.0, stops[len(stops)-1].color))
	}

	colorsAreOpaque := true
	for _, p := range stops {
		if !p.color.IsOpaque() {
			colorsAreOpaque = false
			break
		}
	}

	// Pin the last value to 1.0, and make sure positions are monotonic.
	startIndex := 1
	if dummyFirst {
		startIndex = 0
	}
	var prev float32
	hasUniformStops := true
	uniformStep := stops[startIndex].position.Get() - prev
	for i := startIndex; i < len(stops); i++ {
		var curr float32
		if i+1 == len(stops) {
			// The last one must be zero.
			curr = 1.0
		} else {
			curr = path.Bound(stops[i].position.Get(), prev, 1.0)
		}

		hasUniformStops = hasUniformStops && path.IsNearlyEqual(uniformStep, curr-prev)
		stops[i].position = path.NormalizedF32Clamped(curr)
		prev = curr
	}

	return gradient{
		stops:           stops,
		tileMode:        tileMode,
		transform:       transform,
		pointsToUnit:    pointsToUnit,
		colorsAreOpaque: colorsAreOpaque,
		hasUniformStops: hasUniformStops,
	}
}

// pushStages is Gradient::push_stages in tiny-skia src/shaders/gradient.rs.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// p, as D20 says.
func (g *gradient) pushStages(
	p *rasterPipelineBuilder,
	cs ColorSpace,
	pushStagesPre func(*rasterPipelineBuilder),
	pushStagesPost func(*rasterPipelineBuilder),
) bool {
	p.push(stageSeedShader)

	ts, ok := g.transform.Invert()
	if !ok {
		warn(p.logger, "failed to invert a gradient transform. Nothing will be rendered")
		return false
	}
	ts = ts.PostConcat(g.pointsToUnit)
	p.pushTransform(ts)

	pushStagesPre(p)

	switch g.tileMode {
	case SpreadModeReflect:
		p.push(stageReflectX1)
	case SpreadModeRepeat:
		p.push(stageRepeatX1)
	case SpreadModePad:
		if g.hasUniformStops {
			// We clamp only when the stops are evenly spaced.
			// If not, there can be hard stops, and clamping ruins hard stops at 0 and/or 1.
			// In that case, we must make sure we are using the general "gradient" stage,
			// which is the only stage that correctly handles unclamped t.
			p.push(stagePadX1)
		}
	}

	// The two-stop case with stops at 0 and 1.
	if len(g.stops) == 2 {
		c0 := cs.expandColor(g.stops[0].color)
		c1 := cs.expandColor(g.stops[1].color)

		p.ctx.evenlySpaced2StopGradient = evenlySpaced2StopGradientCtx{
			factor: newGradientColor(
				c1.Red()-c0.Red(),
				c1.Green()-c0.Green(),
				c1.Blue()-c0.Blue(),
				c1.Alpha()-c0.Alpha(),
			),
			bias: gradientColorFromColor(c0),
		}

		p.push(stageEvenlySpaced2StopGradient)
	} else {
		// Unlike Skia, we do not support the `evenly_spaced_gradient` stage.
		// In our case, there is no performance difference.

		var ctx gradientCtx

		// Note: In order to handle clamps in search, the search assumes
		// a stop conceptually placed at -inf.
		// Therefore, the max number of stops is `self.points.len()+1`.
		//
		// We also need at least 16 values for lowp pipeline.
		ctx.factors = make([]gradientColor, 0, max(len(g.stops)+1, 16))
		ctx.biases = make([]gradientColor, 0, max(len(g.stops)+1, 16))

		ctx.tValues = make([]path.NormalizedF32, 0, len(g.stops)+1)

		// Remove the dummy stops inserted by Gradient::new
		// because they are naturally handled by the search method.
		var firstStop, lastStop int
		if len(g.stops) > 2 {
			first := 1
			if g.stops[0].color != g.stops[1].color {
				first = 0
			}

			n := len(g.stops)
			last := n - 2
			if g.stops[n-2].color != g.stops[n-1].color {
				last = n - 1
			}
			firstStop, lastStop = first, last
		} else {
			firstStop, lastStop = 0, 1
		}

		tL := g.stops[firstStop].position.Get()
		cL := gradientColorFromColor(cs.expandColor(g.stops[firstStop].color))
		ctx.pushConstColor(cL)
		ctx.tValues = append(ctx.tValues, path.NormalizedF32Zero)
		// N.B. lastStop is the index of the last stop, not one after.
		for i := firstStop; i < lastStop; i++ {
			tR := g.stops[i+1].position.Get()
			cR := gradientColorFromColor(cs.expandColor(g.stops[i+1].color))
			if tL < tR {
				// For each stop we calculate a bias B and a scale factor F, such that
				// for any t between stops n and n+1, the color we want is B[n] + F[n]*t.
				f := newGradientColor(
					(cR.r-cL.r)/(tR-tL),
					(cR.g-cL.g)/(tR-tL),
					(cR.b-cL.b)/(tR-tL),
					(cR.a-cL.a)/(tR-tL),
				)
				ctx.factors = append(ctx.factors, f)

				ctx.biases = append(ctx.biases, newGradientColor(
					cL.r-float32(f.r*tL),
					cL.g-float32(f.g*tL),
					cL.b-float32(f.b*tL),
					cL.a-float32(f.a*tL),
				))

				ctx.tValues = append(ctx.tValues, path.NormalizedF32Clamped(tL))
			}

			tL = tR
			cL = cR
		}

		ctx.pushConstColor(cL)
		ctx.tValues = append(ctx.tValues, path.NormalizedF32Clamped(tL))

		ctx.len = len(ctx.factors)

		// Fill with zeros until we have enough data to fit into F32x16.
		for len(ctx.factors) < 16 {
			ctx.factors = append(ctx.factors, gradientColor{})
			ctx.biases = append(ctx.biases, gradientColor{})
		}

		p.push(stageGradient)
		p.ctx.gradient = ctx
	}

	if !g.colorsAreOpaque {
		p.push(stagePremultiply)
	}

	pushStagesPost(p)

	return true
}

// applyOpacity is Gradient::apply_opacity in tiny-skia
// src/shaders/gradient.rs.
//
// Port: a copy of a Go gradient shares its stops, so applyOpacity changes a
// copy of the stops. A clone of a Rust gradient has its own stops.
func (g *gradient) applyOpacity(opacity float32) {
	g.stops = slices.Clone(g.stops)
	for i := range g.stops {
		g.stops[i].color.ApplyOpacity(opacity)
	}

	g.colorsAreOpaque = true
	for _, p := range g.stops {
		if !p.color.IsOpaque() {
			g.colorsAreOpaque = false
			break
		}
	}
}
