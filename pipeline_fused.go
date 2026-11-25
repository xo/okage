package okage

import "github.com/xo/okage/internal/wide"

// This file ports no upstream file. It runs the fused programs of D98: a
// copy of a common program of stages that keeps its values in registers,
// which the tool fusegen writes into internal/wide. A pipeline whose stages
// match a fused program runs the copy in place of its stages, and the copy
// gives the same bytes.

// useFused reports whether a pipeline runs the fused program that matches
// its stages. A test sets it to false to run the stages.
var useFused = true

// matchFused returns the fused program of the stages of a pipeline of the
// kind kind, with its values from ctx, or wide.FusedNone if no fused program
// matches them.
func matchFused(kind rasterPipelineKind, stages []stage, ctx *context) (wide.FusedProgram, wide.FusedCtx) {
	var fc wide.FusedCtx
	is := func(want ...stage) bool {
		if len(stages) != len(want) {
			return false
		}
		for i, s := range want {
			if stages[i] != s {
				return false
			}
		}
		return true
	}

	if kind == rasterPipelineKindLow && len(stages) > 0 && stages[0] == stageUniformColor {
		// The fused programs hold a channel of the color in 8 bits.
		rgba := ctx.uniformColor.rgba
		if rgba[0] > 255 || rgba[1] > 255 || rgba[2] > 255 || rgba[3] > 255 {
			return wide.FusedNone, fc
		}
		fc.RGBA = rgba
		switch {
		case is(stageUniformColor, stageSourceOverRgba),
			is(stageUniformColor, stageLoadDestination, stageSourceOver, stageStore):
			return wide.FusedSolidOverLowp, fc
		case is(stageUniformColor, stageScale1Float, stageLoadDestination, stageSourceOver, stageStore):
			return wide.FusedSolidCoverageOverLowp, fc
		case is(stageUniformColor, stageMaskU8, stageLoadDestination, stageSourceOver, stageStore):
			return wide.FusedSolidMaskOverLowp, fc
		case is(stageUniformColor, stageMaskU8, stageScale1Float, stageLoadDestination, stageSourceOver, stageStore):
			return wide.FusedSolidMaskCoverageOverLowp, fc
		}
		return wide.FusedNone, fc
	}

	// A linear gradient of two stops with the spread mode pad. The builder
	// leaves out the stage transform for the identity, and the identity
	// gives the same x: x * 1 + (y * 0 + 0) is x for each x of seed_shader.
	ts := ctx.transform
	switch {
	case is(stageSeedShader, stageTransform, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba):
	case is(stageSeedShader, stagePadX1, stageEvenlySpaced2StopGradient, stagePremultiply, stageSourceOverRgba):
		ts = Transform{SX: 1, SY: 1}
	default:
		return wide.FusedNone, fc
	}
	fc.Transform = [3]float32{ts.SX, ts.KX, ts.TX}
	g := &ctx.evenlySpaced2StopGradient
	fc.Factor = [4]float32{g.factor.r, g.factor.g, g.factor.b, g.factor.a}
	fc.Bias = [4]float32{g.bias.r, g.bias.g, g.bias.b, g.bias.a}
	if kind == rasterPipelineKindLow {
		return wide.FusedLinearPad2Lowp, fc
	}
	return wide.FusedLinearPad2Highp, fc
}

// runFused draws rect with the fused program of rp, and reports whether it
// did. It does not draw when the coverage does not fit the fused program.
func (rp *rasterPipeline) runFused(rect *screenIntRect, maskCtx *maskCtx, pixmapDst *subPixmapMut) bool {
	switch rp.fused {
	case wide.FusedSolidCoverageOverLowp, wide.FusedSolidMaskCoverageOverLowp:
		c := fromFloat(rp.ctx.currentCoverage)[0]
		if c > 255 {
			return false
		}
		rp.fusedCtx.Coverage = c
	}
	readsMask := rp.fused == wide.FusedSolidMaskOverLowp || rp.fused == wide.FusedSolidMaskCoverageOverLowp

	x := int(rect.X())
	n := int(rect.Width())
	for y := int(rect.Y()); y < int(rect.Bottom()); y++ {
		// The code before can leave the upper bits set (D35).
		wide.ClearUpperBits()
		var mask []byte
		if readsMask {
			mask = maskCtx.data[maskCtx.offset(x, y):]
		}
		wide.FusedRow(rp.fused, &rp.fusedCtx, pixmapDst.sliceAtXY(x, y), mask, x, y, n)
	}
	return true
}
