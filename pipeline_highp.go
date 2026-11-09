// Copyright 2018 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/internal/wide"
)

// This file is tiny-skia src/pipeline/highp.rs, a high precision raster
// pipeline implementation.
//
// Unlike lowp, this one implements all stages.
//
// Just like Skia, this pipeline is implemented using f32x8.
//
// For some reason, we are almost 2x slower. Maybe because Skia uses clang's
// vector extensions and we are using a manual implementation.

// highpStageWidth is STAGE_WIDTH in tiny-skia src/pipeline/highp.rs.
const highpStageWidth = 8

// highpPipeline is Pipeline in tiny-skia src/pipeline/highp.rs.
//
// Port: each stage is a method of highpPipeline (D10).
type highpPipeline struct {
	index     int
	functions []func(*highpPipeline)
	pixmapSrc PixmapRef
	pixmapDst *subPixmapMut
	ctx       *context // TODO: remove mut
	maskCtx   maskCtx
	aaMaskCtx aaMaskCtx
	r         wide.F32x8
	g         wide.F32x8
	b         wide.F32x8
	a         wide.F32x8
	dr        wide.F32x8
	dg        wide.F32x8
	db        wide.F32x8
	da        wide.F32x8
	tail      int
	dx        int
	dy        int
}

// nextStage is Pipeline::next_stage in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) nextStage() {
	next := p.functions[p.index]
	p.index++
	next(p)
}

// highpStages is STAGES in tiny-skia src/pipeline/highp.rs. It must be in
// the same order as the values of stage.
var highpStages = [stagesCount]func(*highpPipeline){
	(*highpPipeline).moveSourceToDestination,
	(*highpPipeline).moveDestinationToSource,
	(*highpPipeline).clamp0,
	(*highpPipeline).clampA,
	(*highpPipeline).premultiply,
	(*highpPipeline).uniformColor,
	(*highpPipeline).seedShader,
	(*highpPipeline).loadDst,
	(*highpPipeline).store,
	(*highpPipeline).loadDstU8,
	(*highpPipeline).storeU8,
	(*highpPipeline).gather,
	(*highpPipeline).loadMaskU8,
	(*highpPipeline).maskU8,
	(*highpPipeline).scaleU8,
	(*highpPipeline).lerpU8,
	(*highpPipeline).scale1Float,
	(*highpPipeline).lerp1Float,
	(*highpPipeline).destinationAtop,
	(*highpPipeline).destinationIn,
	(*highpPipeline).destinationOut,
	(*highpPipeline).destinationOver,
	(*highpPipeline).sourceAtop,
	(*highpPipeline).sourceIn,
	(*highpPipeline).sourceOut,
	(*highpPipeline).sourceOver,
	(*highpPipeline).clear,
	(*highpPipeline).modulate,
	(*highpPipeline).multiply,
	(*highpPipeline).plus,
	(*highpPipeline).screen,
	(*highpPipeline).xor,
	(*highpPipeline).colorBurn,
	(*highpPipeline).colorDodge,
	(*highpPipeline).darken,
	(*highpPipeline).difference,
	(*highpPipeline).exclusion,
	(*highpPipeline).hardLight,
	(*highpPipeline).lighten,
	(*highpPipeline).overlay,
	(*highpPipeline).softLight,
	(*highpPipeline).hue,
	(*highpPipeline).saturation,
	(*highpPipeline).color,
	(*highpPipeline).luminosity,
	(*highpPipeline).sourceOverRgba,
	(*highpPipeline).transform,
	(*highpPipeline).reflect,
	(*highpPipeline).repeat,
	(*highpPipeline).bilinear,
	(*highpPipeline).bicubic,
	(*highpPipeline).padX1,
	(*highpPipeline).reflectX1,
	(*highpPipeline).repeatX1,
	(*highpPipeline).gradient,
	(*highpPipeline).evenlySpaced2StopGradient,
	(*highpPipeline).xyToUnitAngle,
	(*highpPipeline).xyToRadius,
	(*highpPipeline).xyTo2PtConicalFocalOnCircle,
	(*highpPipeline).xyTo2PtConicalWellBehaved,
	(*highpPipeline).xyTo2PtConicalSmaller,
	(*highpPipeline).xyTo2PtConicalGreater,
	(*highpPipeline).xyTo2PtConicalStrip,
	(*highpPipeline).mask2PtConicalNan,
	(*highpPipeline).mask2PtConicalDegenerates,
	(*highpPipeline).applyVectorMask,
	(*highpPipeline).alter2PtConicalCompensateFocal,
	(*highpPipeline).alter2PtConicalUnswap,
	(*highpPipeline).negateX,
	(*highpPipeline).applyConcentricScaleBias,
	(*highpPipeline).gammaExpand2,
	(*highpPipeline).gammaExpandDst2,
	(*highpPipeline).gammaCompress2,
	(*highpPipeline).gammaExpand22,
	(*highpPipeline).gammaExpandDst22,
	(*highpPipeline).gammaCompress22,
	(*highpPipeline).gammaExpandSrgb,
	(*highpPipeline).gammaExpandDstSrgb,
	(*highpPipeline).gammaCompressSrgb,
}

// startHighp is start in tiny-skia src/pipeline/highp.rs.
//
// Port: upstream makes the Pipeline here. Go takes it from the caller, so
// that a run does not allocate (see rasterPipeline).
func startHighp(
	p *highpPipeline,
	functions []func(*highpPipeline),
	functionsTail []func(*highpPipeline),
	rect *screenIntRect,
	aaMaskCtx aaMaskCtx,
	maskCtx maskCtx,
	ctx *context,
	pixmapSrc PixmapRef,
	pixmapDst *subPixmapMut,
) {
	*p = highpPipeline{
		index:     0,
		functions: nil,
		pixmapSrc: pixmapSrc,
		pixmapDst: pixmapDst,
		maskCtx:   maskCtx,
		aaMaskCtx: aaMaskCtx,
		ctx:       ctx,
		tail:      0,
		dx:        0,
		dy:        0,
	}

	for y := rect.Y(); y < rect.Bottom(); y++ {
		x := int(rect.X())
		end := int(rect.Right())

		p.functions = functions
		for x+highpStageWidth <= end {
			p.index = 0
			p.dx = x
			p.dy = int(y)
			p.tail = highpStageWidth
			p.nextStage()
			x += highpStageWidth
		}

		if x != end {
			p.index = 0
			p.functions = functionsTail
			p.dx = x
			p.dy = int(y)
			p.tail = end - x
			p.nextStage()
		}
	}
}

// moveSourceToDestination is move_source_to_destination in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) moveSourceToDestination() {
	p.dr = p.r
	p.dg = p.g
	p.db = p.b
	p.da = p.a

	p.nextStage()
}

// premultiply is premultiply in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) premultiply() {
	p.r = p.r.Mul(p.a)
	p.g = p.g.Mul(p.a)
	p.b = p.b.Mul(p.a)

	p.nextStage()
}

// moveDestinationToSource is move_destination_to_source in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) moveDestinationToSource() {
	p.r = p.dr
	p.g = p.dg
	p.b = p.db
	p.a = p.da

	p.nextStage()
}

// clamp0 is clamp_0 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) clamp0() {
	p.r = p.r.Max(wide.F32x8{})
	p.g = p.g.Max(wide.F32x8{})
	p.b = p.b.Max(wide.F32x8{})
	p.a = p.a.Max(wide.F32x8{})

	p.nextStage()
}

// clampA is clamp_a in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) clampA() {
	p.r = p.r.Min(wide.SplatF32x8(1.0))
	p.g = p.g.Min(wide.SplatF32x8(1.0))
	p.b = p.b.Min(wide.SplatF32x8(1.0))
	p.a = p.a.Min(wide.SplatF32x8(1.0))

	p.nextStage()
}

// uniformColor is uniform_color in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) uniformColor() {
	ctx := &p.ctx.uniformColor
	p.r = wide.SplatF32x8(ctx.r)
	p.g = wide.SplatF32x8(ctx.g)
	p.b = wide.SplatF32x8(ctx.b)
	p.a = wide.SplatF32x8(ctx.a)

	p.nextStage()
}

// seedShader is seed_shader in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) seedShader() {
	iota := wide.F32x8{0.5, 1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5}

	p.r = wide.SplatF32x8(float32(p.dx)).Add(iota)
	p.g = wide.SplatF32x8(float32(p.dy) + 0.5)
	p.b = wide.SplatF32x8(1.0)
	p.a = wide.F32x8{}

	p.dr = wide.F32x8{}
	p.dg = wide.F32x8{}
	p.db = wide.F32x8{}
	p.da = wide.F32x8{}

	p.nextStage()
}

// loadDst is load_dst in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) loadDst() {
	load8888Highp(p.pixmapDst.slice4AtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
	p.nextStage()
}

// loadDstTail is load_dst_tail in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) loadDstTail() {
	load8888TailHighp(p.tail, p.pixmapDst.sliceAtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
	p.nextStage()
}

// store is store in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) store() {
	store8888Highp(&p.r, &p.g, &p.b, &p.a, p.pixmapDst.slice4AtXY(p.dx, p.dy))
	p.nextStage()
}

// storeTail is store_tail in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) storeTail() {
	store8888TailHighp(&p.r, &p.g, &p.b, &p.a, p.tail, p.pixmapDst.sliceAtXY(p.dx, p.dy))
	p.nextStage()
}

// loadDstU8 is load_dst_u8 in tiny-skia src/pipeline/highp.rs.
//
// Currently, all mask/A8 pixmaps are handled by lowp.
func (*highpPipeline) loadDstU8() {
	// unreachable
}

// loadDstU8Tail is load_dst_u8_tail in tiny-skia src/pipeline/highp.rs.
func (*highpPipeline) loadDstU8Tail() {
	// unreachable
}

// storeU8 is store_u8 in tiny-skia src/pipeline/highp.rs.
func (*highpPipeline) storeU8() {
	// unreachable
}

// storeU8Tail is store_u8_tail in tiny-skia src/pipeline/highp.rs.
func (*highpPipeline) storeU8Tail() {
	// unreachable
}

// gather is gather in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gather() {
	ix := gatherIx(p.pixmapSrc, p.r, p.g)
	pixels := p.pixmapSrc.gather(ix)
	load8888Highp(&pixels, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// gatherIx is gather_ix in tiny-skia src/pipeline/highp.rs.
func gatherIx(pixmap PixmapRef, x, y wide.F32x8) wide.U32x8 {
	// Exclusive -> inclusive.
	w := ulpSub(float32(pixmap.Width()))
	h := ulpSub(float32(pixmap.Height()))
	x = x.Max(wide.F32x8{}).Min(wide.SplatF32x8(w))
	y = y.Max(wide.F32x8{}).Min(wide.SplatF32x8(h))

	return y.TruncInt().Mul(wide.SplatI32x8(int32(pixmap.Width()))).Add(x.TruncInt()).ToU32x8Bitcast()
}

// ulpSub is ulp_sub in tiny-skia src/pipeline/highp.rs.
func ulpSub(v float32) float32 {
	// Somewhat similar to v - f32::EPSILON
	return math.Float32frombits(math.Float32bits(v) - 1)
}

// loadMaskU8 is load_mask_u8 in tiny-skia src/pipeline/highp.rs.
func (*highpPipeline) loadMaskU8() {
	// unreachable
}

// maskU8 is mask_u8 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) maskU8() {
	offset := p.maskCtx.offset(p.dx, p.dy)
	var c wide.F32x8
	for i := range p.tail {
		c[i] = float32(p.maskCtx.data[offset+i])
	}
	c = c.Div(wide.SplatF32x8(255.0))

	if c.Equal(wide.F32x8{}) {
		return
	}

	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)

	p.nextStage()
}

// scaleU8 is scale_u8 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) scaleU8() {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	c = c.Div(wide.SplatF32x8(255.0))

	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)

	p.nextStage()
}

// lerpU8 is lerp_u8 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) lerpU8() {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}
	c = c.Div(wide.SplatF32x8(255.0))

	p.r = lerpHighp(p.dr, p.r, c)
	p.g = lerpHighp(p.dg, p.g, c)
	p.b = lerpHighp(p.db, p.b, c)
	p.a = lerpHighp(p.da, p.a, c)

	p.nextStage()
}

// scale1Float is scale_1_float in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) scale1Float() {
	c := wide.SplatF32x8(p.ctx.currentCoverage)
	p.r = p.r.Mul(c)
	p.g = p.g.Mul(c)
	p.b = p.b.Mul(c)
	p.a = p.a.Mul(c)

	p.nextStage()
}

// lerp1Float is lerp_1_float in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) lerp1Float() {
	c := wide.SplatF32x8(p.ctx.currentCoverage)
	p.r = lerpHighp(p.dr, p.r, c)
	p.g = lerpHighp(p.dg, p.g, c)
	p.b = lerpHighp(p.db, p.b, c)
	p.a = lerpHighp(p.da, p.a, c)

	p.nextStage()
}

// The stages below are the blend_fn! macro of tiny-skia
// src/pipeline/highp.rs. Each one applies its function to each channel, with
// the source, the destination, the source alpha and the destination alpha.
//
// Port: Go has no macro, so each stage calls a function of this file with
// the name of the stage and the suffix Highp.

// clear is the stage clear of blend_fn! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) clear() {
	p.r = wide.F32x8{}
	p.g = wide.F32x8{}
	p.b = wide.F32x8{}
	p.a = wide.F32x8{}

	p.nextStage()
}

// blendFn applies f to each channel, as the macro blend_fn! of tiny-skia
// src/pipeline/highp.rs does, and calls the next stage.
func (p *highpPipeline) blendFn(f func(s, d, sa, da wide.F32x8) wide.F32x8) {
	p.r = f(p.r, p.dr, p.a, p.da)
	p.g = f(p.g, p.dg, p.a, p.da)
	p.b = f(p.b, p.db, p.a, p.da)
	p.a = f(p.a, p.da, p.a, p.da)

	p.nextStage()
}

// sourceAtop is the stage source_atop of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceAtop() { p.blendFn(sourceAtopHighp) }

// sourceAtopHighp is the function of the stage source_atop in tiny-skia
// src/pipeline/highp.rs.
func sourceAtopHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(da).Add(d.Mul(invHighp(sa)))
}

// destinationAtop is the stage destination_atop of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationAtop() { p.blendFn(destinationAtopHighp) }

// destinationAtopHighp is the function of the stage destination_atop in
// tiny-skia src/pipeline/highp.rs.
func destinationAtopHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.Mul(sa).Add(s.Mul(invHighp(da)))
}

// sourceIn is the stage source_in of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceIn() { p.blendFn(sourceInHighp) }

// sourceInHighp is the function of the stage source_in in tiny-skia
// src/pipeline/highp.rs.
func sourceInHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	return s.Mul(da)
}

// destinationIn is the stage destination_in of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationIn() { p.blendFn(destinationInHighp) }

// destinationInHighp is the function of the stage destination_in in
// tiny-skia src/pipeline/highp.rs.
func destinationInHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	return d.Mul(sa)
}

// sourceOut is the stage source_out of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOut() { p.blendFn(sourceOutHighp) }

// sourceOutHighp is the function of the stage source_out in tiny-skia
// src/pipeline/highp.rs.
func sourceOutHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	return s.Mul(invHighp(da))
}

// destinationOut is the stage destination_out of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationOut() { p.blendFn(destinationOutHighp) }

// destinationOutHighp is the function of the stage destination_out in
// tiny-skia src/pipeline/highp.rs.
func destinationOutHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	return d.Mul(invHighp(sa))
}

// sourceOver is the stage source_over of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOver() { p.blendFn(sourceOverHighp) }

// sourceOverHighp is the function of the stage source_over in tiny-skia
// src/pipeline/highp.rs.
func sourceOverHighp(s, d, sa, _ wide.F32x8) wide.F32x8 {
	return madHighp(d, invHighp(sa), s)
}

// destinationOver is the stage destination_over of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationOver() { p.blendFn(destinationOverHighp) }

// destinationOverHighp is the function of the stage destination_over in
// tiny-skia src/pipeline/highp.rs.
func destinationOverHighp(s, d, _, da wide.F32x8) wide.F32x8 {
	return madHighp(s, invHighp(da), d)
}

// modulate is the stage modulate of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) modulate() { p.blendFn(modulateHighp) }

// modulateHighp is the function of the stage modulate in tiny-skia
// src/pipeline/highp.rs.
func modulateHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Mul(d)
}

// multiply is the stage multiply of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) multiply() { p.blendFn(multiplyHighp) }

// multiplyHighp is the function of the stage multiply in tiny-skia
// src/pipeline/highp.rs.
func multiplyHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(invHighp(da)).Add(d.Mul(invHighp(sa))).Add(s.Mul(d))
}

// screen is the stage screen of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) screen() { p.blendFn(screenHighp) }

// screenHighp is the function of the stage screen in tiny-skia
// src/pipeline/highp.rs.
func screenHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(d))
}

// xor is the stage xor of blend_fn! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xor() { p.blendFn(xorHighp) }

// xorHighp is the function of the stage xor in tiny-skia
// src/pipeline/highp.rs.
func xorHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(invHighp(da)).Add(d.Mul(invHighp(sa)))
}

// plus is the stage plus of blend_fn! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) plus() { p.blendFn(plusHighp) }

// plusHighp is the function of the stage plus in tiny-skia
// src/pipeline/highp.rs.
func plusHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Min(wide.SplatF32x8(1.0))
}

// blendFn2 applies f to each color channel and source_over to the alpha, as
// the macro blend_fn2! of tiny-skia src/pipeline/highp.rs does, and calls the
// next stage.
func (p *highpPipeline) blendFn2(f func(s, d, sa, da wide.F32x8) wide.F32x8) {
	// The same logic applied to color, and source_over for alpha.
	p.r = f(p.r, p.dr, p.a, p.da)
	p.g = f(p.g, p.dg, p.a, p.da)
	p.b = f(p.b, p.db, p.a, p.da)
	p.a = madHighp(p.da, invHighp(p.a), p.a)

	p.nextStage()
}

// darken is the stage darken of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) darken() { p.blendFn2(darkenHighp) }

// darkenHighp is the function of the stage darken in tiny-skia
// src/pipeline/highp.rs.
func darkenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(da).Max(d.Mul(sa)))
}

// lighten is the stage lighten of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) lighten() { p.blendFn2(lightenHighp) }

// lightenHighp is the function of the stage lighten in tiny-skia
// src/pipeline/highp.rs.
func lightenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(s.Mul(da).Min(d.Mul(sa)))
}

// difference is the stage difference of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) difference() { p.blendFn2(differenceHighp) }

// differenceHighp is the function of the stage difference in tiny-skia
// src/pipeline/highp.rs.
func differenceHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(two(s.Mul(da).Min(d.Mul(sa))))
}

// exclusion is the stage exclusion of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) exclusion() { p.blendFn2(exclusionHighp) }

// exclusionHighp is the function of the stage exclusion in tiny-skia
// src/pipeline/highp.rs.
func exclusionHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	return s.Add(d).Sub(two(s.Mul(d)))
}

// colorBurn is the stage color_burn of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) colorBurn() { p.blendFn2(colorBurnHighp) }

// colorBurnHighp is the function of the stage color_burn in tiny-skia
// src/pipeline/highp.rs.
func colorBurnHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.CmpEq(da).Blend(
		d.Add(s.Mul(invHighp(da))),
		s.CmpEq(wide.F32x8{}).Blend(
			d.Mul(invHighp(sa)),
			sa.Mul(da.Sub(da.Min(da.Sub(d).Mul(sa).Mul(s.RecipFast())))).Add(s.Mul(invHighp(da))).Add(d.Mul(invHighp(sa))),
		),
	)
}

// colorDodge is the stage color_dodge of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) colorDodge() { p.blendFn2(colorDodgeHighp) }

// colorDodgeHighp is the function of the stage color_dodge in tiny-skia
// src/pipeline/highp.rs.
func colorDodgeHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return d.CmpEq(wide.F32x8{}).Blend(
		s.Mul(invHighp(da)),
		s.CmpEq(sa).Blend(
			s.Add(d.Mul(invHighp(sa))),
			sa.Mul(da.Min(d.Mul(sa).Mul(sa.Sub(s).RecipFast()))).Add(s.Mul(invHighp(da))).Add(d.Mul(invHighp(sa))),
		),
	)
}

// hardLight is the stage hard_light of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) hardLight() { p.blendFn2(hardLightHighp) }

// hardLightHighp is the function of the stage hard_light in tiny-skia
// src/pipeline/highp.rs.
func hardLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(invHighp(da)).Add(d.Mul(invHighp(sa))).Add(two(s).CmpLe(sa).Blend(
		two(s.Mul(d)),
		sa.Mul(da).Sub(two(da.Sub(d).Mul(sa.Sub(s)))),
	))
}

// overlay is the stage overlay of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) overlay() { p.blendFn2(overlayHighp) }

// overlayHighp is the function of the stage overlay in tiny-skia
// src/pipeline/highp.rs.
func overlayHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	return s.Mul(invHighp(da)).Add(d.Mul(invHighp(sa))).Add(two(d).CmpLe(da).Blend(
		two(s.Mul(d)),
		sa.Mul(da).Sub(two(da.Sub(d).Mul(sa.Sub(s)))),
	))
}

// softLight is the stage soft_light of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) softLight() { p.blendFn2(softLightHighp) }

// softLightHighp is the function of the stage soft_light in tiny-skia
// src/pipeline/highp.rs.
func softLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	one := wide.SplatF32x8(1.0)
	m := da.CmpGt(wide.F32x8{}).Blend(d.Div(da), wide.F32x8{})
	s2 := two(s)
	m4 := two(two(m))

	// The logic forks three ways:
	//    1. dark src?
	//    2. light src, dark dst?
	//    3. light src, light dst?
	darkSrc := d.Mul(sa.Add(s2.Sub(sa).Mul(one.Sub(m))))
	darkDst := m4.Mul(m4).Add(m4).Mul(m.Sub(one)).Add(wide.SplatF32x8(7.0).Mul(m))
	liteDst := m.Sqrt().Sub(m)
	liteSrc := d.Mul(sa).Add(da.Mul(s2.Sub(sa)).
		Mul(two(two(d)).CmpLe(da).Blend(darkDst, liteDst))) // 2 or 3?

	return s.Mul(invHighp(da)).Add(d.Mul(invHighp(sa))).Add(s2.CmpLe(sa).Blend(darkSrc, liteSrc)) // 1 or (2 or 3)?
}

// We are basing our implementation of non-separable blend modes on
//   https://www.w3.org/TR/compositing-1/#blendingnonseparable.
// and
//   https://www.khronos.org/registry/OpenGL/specs/es/3.2/es_spec_3.2.pdf
// They are equivalent, but ES' math has been better simplified.
//
// Anything extra we add beyond that is to make the math work with premul inputs.

// blendFn3 applies f to all the channels at once, as the macro blend_fn3! of
// tiny-skia src/pipeline/highp.rs does, and calls the next stage.
func (p *highpPipeline) blendFn3(f func(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8)) {
	tr, tg, tb, ta := f(p.r, p.g, p.b, p.a, p.dr, p.dg, p.db, p.da)
	p.r = tr
	p.g = tg
	p.b = tb
	p.a = ta

	p.nextStage()
}

// hue is the stage hue of blend_fn3! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) hue() { p.blendFn3(hueK) }

// hueK is hue_k in tiny-skia src/pipeline/highp.rs.
func hueK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := r.Mul(a)
	gg := g.Mul(a)
	bb := b.Mul(a)

	setSat(&rr, &gg, &bb, sat(dr, dg, db).Mul(a))
	setLum(&rr, &gg, &bb, lum(dr, dg, db).Mul(a))
	clipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(invHighp(da)).Add(dr.Mul(invHighp(a))).Add(rr)
	g = g.Mul(invHighp(da)).Add(dg.Mul(invHighp(a))).Add(gg)
	b = b.Mul(invHighp(da)).Add(db.Mul(invHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// saturation is the stage saturation of blend_fn3! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) saturation() { p.blendFn3(saturationK) }

// saturationK is saturation_k in tiny-skia src/pipeline/highp.rs.
func saturationK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := dr.Mul(a)
	gg := dg.Mul(a)
	bb := db.Mul(a)

	setSat(&rr, &gg, &bb, sat(r, g, b).Mul(da))
	setLum(&rr, &gg, &bb, lum(dr, dg, db).Mul(a)) // (This is not redundant.)
	clipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(invHighp(da)).Add(dr.Mul(invHighp(a))).Add(rr)
	g = g.Mul(invHighp(da)).Add(dg.Mul(invHighp(a))).Add(gg)
	b = b.Mul(invHighp(da)).Add(db.Mul(invHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// color is the stage color of blend_fn3! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) color() { p.blendFn3(colorK) }

// colorK is color_k in tiny-skia src/pipeline/highp.rs.
func colorK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := r.Mul(da)
	gg := g.Mul(da)
	bb := b.Mul(da)

	setLum(&rr, &gg, &bb, lum(dr, dg, db).Mul(a))
	clipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(invHighp(da)).Add(dr.Mul(invHighp(a))).Add(rr)
	g = g.Mul(invHighp(da)).Add(dg.Mul(invHighp(a))).Add(gg)
	b = b.Mul(invHighp(da)).Add(db.Mul(invHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// luminosity is the stage luminosity of blend_fn3! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) luminosity() { p.blendFn3(luminosityK) }

// luminosityK is luminosity_k in tiny-skia src/pipeline/highp.rs.
func luminosityK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	rr := dr.Mul(a)
	gg := dg.Mul(a)
	bb := db.Mul(a)

	setLum(&rr, &gg, &bb, lum(r, g, b).Mul(da))
	clipColor(&rr, &gg, &bb, a.Mul(da))

	r = r.Mul(invHighp(da)).Add(dr.Mul(invHighp(a))).Add(rr)
	g = g.Mul(invHighp(da)).Add(dg.Mul(invHighp(a))).Add(gg)
	b = b.Mul(invHighp(da)).Add(db.Mul(invHighp(a))).Add(bb)
	a = a.Add(da).Sub(a.Mul(da))

	return r, g, b, a
}

// sat is sat in tiny-skia src/pipeline/highp.rs.
func sat(r, g, b wide.F32x8) wide.F32x8 {
	return r.Max(g.Max(b)).Sub(r.Min(g.Min(b)))
}

// lum is lum in tiny-skia src/pipeline/highp.rs.
func lum(r, g, b wide.F32x8) wide.F32x8 {
	return r.Mul(wide.SplatF32x8(0.30)).Add(g.Mul(wide.SplatF32x8(0.59))).Add(b.Mul(wide.SplatF32x8(0.11)))
}

// setSat is set_sat in tiny-skia src/pipeline/highp.rs.
func setSat(r, g, b *wide.F32x8, s wide.F32x8) {
	mn := r.Min(g.Min(*b))
	mx := r.Max(g.Max(*b))
	sat := mx.Sub(mn)

	// Map min channel to 0, max channel to s, and scale the middle proportionally.
	//
	// Port: upstream has a closure scale. Go writes it out for each channel.
	isZero := sat.CmpEq(wide.F32x8{})
	*r = isZero.Blend(wide.F32x8{}, r.Sub(mn).Mul(s).Div(sat))
	*g = isZero.Blend(wide.F32x8{}, g.Sub(mn).Mul(s).Div(sat))
	*b = isZero.Blend(wide.F32x8{}, b.Sub(mn).Mul(s).Div(sat))
}

// setLum is set_lum in tiny-skia src/pipeline/highp.rs.
func setLum(r, g, b *wide.F32x8, l wide.F32x8) {
	diff := l.Sub(lum(*r, *g, *b))
	*r = r.Add(diff)
	*g = g.Add(diff)
	*b = b.Add(diff)
}

// clipColor is clip_color in tiny-skia src/pipeline/highp.rs.
func clipColor(r, g, b *wide.F32x8, a wide.F32x8) {
	mn := r.Min(g.Min(*b))
	mx := r.Max(g.Max(*b))
	l := lum(*r, *g, *b)

	*r = clipColorChannel(*r, mn, mx, l, a)
	*g = clipColorChannel(*g, mn, mx, l, a)
	*b = clipColorChannel(*b, mn, mx, l, a)
}

// clipColorChannel is the closure clip of clip_color in tiny-skia
// src/pipeline/highp.rs.
func clipColorChannel(c, mn, mx, l, a wide.F32x8) wide.F32x8 {
	c = mx.CmpGe(wide.F32x8{}).Blend(c, l.Add(c.Sub(l).Mul(l).Div(l.Sub(mn))))
	c = mx.CmpGt(a).Blend(l.Add(c.Sub(l).Mul(a.Sub(l)).Div(mx.Sub(l))), c)
	c = c.Max(wide.F32x8{}) // Sometimes without this we can dip just a little negative.
	return c
}

// sourceOverRgba is source_over_rgba in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) sourceOverRgba() {
	pixels := p.pixmapDst.slice4AtXY(p.dx, p.dy)
	load8888Highp(pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = madHighp(p.dr, invHighp(p.a), p.r)
	p.g = madHighp(p.dg, invHighp(p.a), p.g)
	p.b = madHighp(p.db, invHighp(p.a), p.b)
	p.a = madHighp(p.da, invHighp(p.a), p.a)
	store8888Highp(&p.r, &p.g, &p.b, &p.a, pixels)

	p.nextStage()
}

// sourceOverRgbaTail is source_over_rgba_tail in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOverRgbaTail() {
	pixels := p.pixmapDst.sliceAtXY(p.dx, p.dy)
	load8888TailHighp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = madHighp(p.dr, invHighp(p.a), p.r)
	p.g = madHighp(p.dg, invHighp(p.a), p.g)
	p.b = madHighp(p.db, invHighp(p.a), p.b)
	p.a = madHighp(p.da, invHighp(p.a), p.a)
	store8888TailHighp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)

	p.nextStage()
}

// transform is transform in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) transform() {
	ts := &p.ctx.transform

	tr := madHighp(p.r, wide.SplatF32x8(ts.SX), madHighp(p.g, wide.SplatF32x8(ts.KX), wide.SplatF32x8(ts.TX)))
	tg := madHighp(p.r, wide.SplatF32x8(ts.KY), madHighp(p.g, wide.SplatF32x8(ts.SY), wide.SplatF32x8(ts.TY)))
	p.r = tr
	p.g = tg

	p.nextStage()
}

// Tile x or y to [0,limit) == [0,limit - 1 ulp] (think, sampling from images).
// The gather stages hard clamp the output of these stages to [0,limit)...
// we just need to do the basic repeat or mirroring.

// reflect is reflect in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) reflect() {
	ctx := &p.ctx.limitX
	p.r = exclusiveReflect(p.r, ctx.scale, ctx.invScale)

	ctx = &p.ctx.limitY
	p.g = exclusiveReflect(p.g, ctx.scale, ctx.invScale)

	p.nextStage()
}

// exclusiveReflect is exclusive_reflect in tiny-skia src/pipeline/highp.rs.
func exclusiveReflect(v wide.F32x8, limit, invLimit float32) wide.F32x8 {
	limitV := wide.SplatF32x8(limit)
	invLimitV := wide.SplatF32x8(invLimit)
	return v.Sub(limitV).Sub(limitV.Add(limitV).
		Mul(v.Sub(limitV).Mul(invLimitV.Mul(wide.SplatF32x8(0.5))).Floor())).Sub(limitV).Abs()
}

// repeat is repeat in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) repeat() {
	ctx := &p.ctx.limitX
	p.r = exclusiveRepeat(p.r, ctx.scale, ctx.invScale)

	ctx = &p.ctx.limitY
	p.g = exclusiveRepeat(p.g, ctx.scale, ctx.invScale)

	p.nextStage()
}

// exclusiveRepeat is exclusive_repeat in tiny-skia src/pipeline/highp.rs.
func exclusiveRepeat(v wide.F32x8, limit, invLimit float32) wide.F32x8 {
	return v.Sub(v.Mul(wide.SplatF32x8(invLimit)).Floor().Mul(wide.SplatF32x8(limit)))
}

// bilinear is bilinear in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) bilinear() {
	x := p.r
	fx := x.Add(wide.SplatF32x8(0.5)).Fract()
	y := p.g
	fy := y.Add(wide.SplatF32x8(0.5)).Fract()
	one := wide.SplatF32x8(1.0)
	wx := [2]wide.F32x8{one.Sub(fx), fx}
	wy := [2]wide.F32x8{one.Sub(fy), fy}

	sampler2x2(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// bicubic is bicubic in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) bicubic() {
	x := p.r
	fx := x.Add(wide.SplatF32x8(0.5)).Fract()
	y := p.g
	fy := y.Add(wide.SplatF32x8(0.5)).Fract()
	one := wide.SplatF32x8(1.0)
	wx := [4]wide.F32x8{bicubicFar(one.Sub(fx)), bicubicNear(one.Sub(fx)), bicubicNear(fx), bicubicFar(fx)}
	wy := [4]wide.F32x8{bicubicFar(one.Sub(fy)), bicubicNear(one.Sub(fy)), bicubicNear(fy), bicubicFar(fy)}

	sampler4x4(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// In bicubic interpolation, the 16 pixels and +/- 0.5 and +/- 1.5 offsets from the sample
// pixel center are combined with a non-uniform cubic filter, with higher values near the center.
//
// We break this function into two parts, one for near 0.5 offsets and one for far 1.5 offsets.

// bicubicNear is bicubic_near in tiny-skia src/pipeline/highp.rs.
func bicubicNear(t wide.F32x8) wide.F32x8 {
	// 1/18 + 9/18t + 27/18t^2 - 21/18t^3 == t ( t ( -21/18t + 27/18) + 9/18) + 1/18
	return madHighp(
		t,
		madHighp(t,
			madHighp(
				wide.SplatF32x8(-21.0/18.0),
				t,
				wide.SplatF32x8(27.0/18.0),
			),
			wide.SplatF32x8(9.0/18.0),
		),
		wide.SplatF32x8(1.0/18.0),
	)
}

// bicubicFar is bicubic_far in tiny-skia src/pipeline/highp.rs.
func bicubicFar(t wide.F32x8) wide.F32x8 {
	// 0/18 + 0/18*t - 6/18t^2 + 7/18t^3 == t^2 (7/18t - 6/18)
	return t.Mul(t).Mul(madHighp(wide.SplatF32x8(7.0/18.0), t, wide.SplatF32x8(-6.0/18.0)))
}

// sampler2x2 is sampler_2x2 in tiny-skia src/pipeline/highp.rs.
func sampler2x2(
	pixmap PixmapRef,
	ctx *samplerCtx,
	cx, cy wide.F32x8,
	wx, wy *[2]wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	*r = wide.F32x8{}
	*g = wide.F32x8{}
	*b = wide.F32x8{}
	*a = wide.F32x8{}

	one := wide.SplatF32x8(1.0)
	const start = -0.5
	y := cy.Add(wide.SplatF32x8(start))
	for j := range 2 {
		x := cx.Add(wide.SplatF32x8(start))
		for i := range 2 {
			var rr, gg, bb, aa wide.F32x8
			sample(pixmap, ctx, x, y, &rr, &gg, &bb, &aa)

			w := wx[i].Mul(wy[j])
			*r = madHighp(w, rr, *r)
			*g = madHighp(w, gg, *g)
			*b = madHighp(w, bb, *b)
			*a = madHighp(w, aa, *a)

			x = x.Add(one)
		}

		y = y.Add(one)
	}
}

// sampler4x4 is sampler_4x4 in tiny-skia src/pipeline/highp.rs.
func sampler4x4(
	pixmap PixmapRef,
	ctx *samplerCtx,
	cx, cy wide.F32x8,
	wx, wy *[4]wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	*r = wide.F32x8{}
	*g = wide.F32x8{}
	*b = wide.F32x8{}
	*a = wide.F32x8{}

	one := wide.SplatF32x8(1.0)
	const start = -1.5
	y := cy.Add(wide.SplatF32x8(start))
	for j := range 4 {
		x := cx.Add(wide.SplatF32x8(start))
		for i := range 4 {
			var rr, gg, bb, aa wide.F32x8
			sample(pixmap, ctx, x, y, &rr, &gg, &bb, &aa)

			w := wx[i].Mul(wy[j])
			*r = madHighp(w, rr, *r)
			*g = madHighp(w, gg, *g)
			*b = madHighp(w, bb, *b)
			*a = madHighp(w, aa, *a)

			x = x.Add(one)
		}

		y = y.Add(one)
	}
}

// sample is sample in tiny-skia src/pipeline/highp.rs.
func sample(
	pixmap PixmapRef, ctx *samplerCtx, x, y wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	x = tile(x, ctx.spreadMode, float32(pixmap.Width()), ctx.invWidth)
	y = tile(y, ctx.spreadMode, float32(pixmap.Height()), ctx.invHeight)

	ix := gatherIx(pixmap, x, y)
	pixels := pixmap.gather(ix)
	load8888Highp(&pixels, r, g, b, a)
}

// tile is tile in tiny-skia src/pipeline/highp.rs.
func tile(v wide.F32x8, mode SpreadMode, limit, invLimit float32) wide.F32x8 {
	switch mode {
	case SpreadModeRepeat:
		return exclusiveRepeat(v, limit, invLimit)
	case SpreadModeReflect:
		return exclusiveReflect(v, limit, invLimit)
	}
	// SpreadModePad
	return v
}

// padX1 is pad_x1 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) padX1() {
	p.r = p.r.Normalize()

	p.nextStage()
}

// reflectX1 is reflect_x1 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) reflectX1() {
	one := wide.SplatF32x8(1.0)
	p.r = p.r.Sub(one).
		Sub(two(p.r.Sub(one).Mul(wide.SplatF32x8(0.5)).Floor())).
		Sub(one).
		Abs().Normalize()

	p.nextStage()
}

// repeatX1 is repeat_x1 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) repeatX1() {
	p.r = p.r.Sub(p.r.Floor()).Normalize()

	p.nextStage()
}

// gradient is gradient in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gradient() {
	ctx := &p.ctx.gradient

	// N.B. The loop starts at 1 because idx 0 is the color to use before the first stop.
	t := p.r
	var idx wide.U32x8
	for i := 1; i < ctx.len; i++ {
		tt := ctx.tValues[i].Get()
		n := wide.U32x8{
			b2u32(t[0] >= tt),
			b2u32(t[1] >= tt),
			b2u32(t[2] >= tt),
			b2u32(t[3] >= tt),
			b2u32(t[4] >= tt),
			b2u32(t[5] >= tt),
			b2u32(t[6] >= tt),
			b2u32(t[7] >= tt),
		}
		idx = idx.Add(n)
	}
	gradientLookupHighp(ctx, &idx, p.r, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// b2u32 is the Rust cast "b as u32" for a bool. It ports no upstream
// function.
func b2u32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// gradientLookupHighp is gradient_lookup in tiny-skia src/pipeline/highp.rs.
func gradientLookupHighp(
	ctx *gradientCtx, idx *wide.U32x8, t wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	// Surprisingly, but bound checking does not affect the performance.
	// And since `idx` can contain any number, we leave it in place.
	//
	// Port: the macro gather! of upstream reads one field for each lane. Go
	// reads all the fields of a lane at once, which gives the same values.
	var fr, fg, fb, fa, br, bg, bb, ba wide.F32x8
	for i := range highpStageWidth {
		f := &ctx.factors[idx[i]]
		fr[i] = f.r
		fg[i] = f.g
		fb[i] = f.b
		fa[i] = f.a
	}
	for i := range highpStageWidth {
		c := &ctx.biases[idx[i]]
		br[i] = c.r
		bg[i] = c.g
		bb[i] = c.b
		ba[i] = c.a
	}

	*r = madHighp(t, fr, br)
	*g = madHighp(t, fg, bg)
	*b = madHighp(t, fb, bb)
	*a = madHighp(t, fa, ba)
}

// evenlySpaced2StopGradient is evenly_spaced_2_stop_gradient in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) evenlySpaced2StopGradient() {
	ctx := &p.ctx.evenlySpaced2StopGradient

	t := p.r
	p.r = madHighp(t, wide.SplatF32x8(ctx.factor.r), wide.SplatF32x8(ctx.bias.r))
	p.g = madHighp(t, wide.SplatF32x8(ctx.factor.g), wide.SplatF32x8(ctx.bias.g))
	p.b = madHighp(t, wide.SplatF32x8(ctx.factor.b), wide.SplatF32x8(ctx.bias.b))
	p.a = madHighp(t, wide.SplatF32x8(ctx.factor.a), wide.SplatF32x8(ctx.bias.a))

	p.nextStage()
}

// xyToUnitAngle is xy_to_unit_angle in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xyToUnitAngle() {
	x := p.r
	y := p.g
	xAbs := x.Abs()
	yAbs := y.Abs()
	slope := xAbs.Min(yAbs).Div(xAbs.Max(yAbs))
	s := slope.Mul(slope)
	// Use a 7th degree polynomial to approximate atan.
	// This was generated using sollya.gforge.inria.fr.
	// A float optimized polynomial was generated using the following command.
	// P1 = fpminimax((1/(2*Pi))*atan(x),[|1,3,5,7|],[|24...|],[2^(-40),1],relative);
	phi := slope.Mul(
		wide.SplatF32x8(0.15912117063999176025390625).Add(
			s.Mul(wide.SplatF32x8(-5.185396969318389892578125e-2).Add(
				s.Mul(wide.SplatF32x8(2.476101927459239959716796875e-2).Add(
					s.Mul(wide.SplatF32x8(-7.0547382347285747528076171875e-3))))))))
	phi = xAbs.CmpLt(yAbs).Blend(wide.SplatF32x8(0.25).Sub(phi), phi)
	phi = x.
		CmpLt(wide.SplatF32x8(0.0)).
		Blend(wide.SplatF32x8(0.5).Sub(phi), phi)
	phi = y.
		CmpLt(wide.SplatF32x8(0.0)).
		Blend(wide.SplatF32x8(1.0).Sub(phi), phi)
	phi = phi.CmpNe(phi).Blend(wide.SplatF32x8(0.0), phi)
	p.r = phi
	p.nextStage()
}

// xyToRadius is xy_to_radius in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xyToRadius() {
	x2 := p.r.Mul(p.r)
	y2 := p.g.Mul(p.g)
	p.r = x2.Add(y2).Sqrt()

	p.nextStage()
}

// xyTo2PtConicalFocalOnCircle is xy_to_2pt_conical_focal_on_circle in
// tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalFocalOnCircle() {
	x := p.r
	y := p.g
	p.r = x.Add(y.Mul(y).Div(x))

	p.nextStage()
}

// xyTo2PtConicalWellBehaved is xy_to_2pt_conical_well_behaved in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalWellBehaved() {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Add(y.Mul(y)).Sqrt().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))

	p.nextStage()
}

// xyTo2PtConicalGreater is xy_to_2pt_conical_greater in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalGreater() {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Sub(y.Mul(y)).Sqrt().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))

	p.nextStage()
}

// xyTo2PtConicalSmaller is xy_to_2pt_conical_smaller in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalSmaller() {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Mul(x).Sub(y.Mul(y)).Sqrt().Neg().Sub(x.Mul(wide.SplatF32x8(ctx.p0)))

	p.nextStage()
}

// xyTo2PtConicalStrip is xy_to_2pt_conical_strip in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalStrip() {
	ctx := &p.ctx.twoPointConicalGradient

	x := p.r
	y := p.g
	p.r = x.Add(wide.SplatF32x8(ctx.p0).Sub(y.Mul(y)).Sqrt())

	p.nextStage()
}

// mask2PtConicalNan is mask_2pt_conical_nan in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) mask2PtConicalNan() {
	ctx := &p.ctx.twoPointConicalGradient

	t := p.r
	isDegenerate := t.CmpNe(t)
	p.r = isDegenerate.Blend(wide.F32x8{}, t)
	ctx.mask = condToMask(isDegenerate.ToU32x8Bitcast().Not())

	p.nextStage()
}

// mask2PtConicalDegenerates is mask_2pt_conical_degenerates in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) mask2PtConicalDegenerates() {
	ctx := &p.ctx.twoPointConicalGradient

	t := p.r
	isDegenerate := t.CmpLe(wide.F32x8{}).Or(t.CmpNe(t))
	p.r = isDegenerate.Blend(wide.F32x8{}, t)
	ctx.mask = condToMask(isDegenerate.ToU32x8Bitcast().Not())

	p.nextStage()
}

// applyVectorMask is apply_vector_mask in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) applyVectorMask() {
	ctx := &p.ctx.twoPointConicalGradient

	p.r = p.r.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.g = p.g.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.b = p.b.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()
	p.a = p.a.ToU32x8Bitcast().And(ctx.mask).ToF32x8Bitcast()

	p.nextStage()
}

// alter2PtConicalCompensateFocal is alter_2pt_conical_compensate_focal in
// tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) alter2PtConicalCompensateFocal() {
	ctx := &p.ctx.twoPointConicalGradient

	p.r = p.r.Add(wide.SplatF32x8(ctx.p1))

	p.nextStage()
}

// alter2PtConicalUnswap is alter_2pt_conical_unswap in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) alter2PtConicalUnswap() {
	p.r = wide.SplatF32x8(1.0).Sub(p.r)

	p.nextStage()
}

// negateX is negate_x in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) negateX() {
	p.r = p.r.Neg()

	p.nextStage()
}

// applyConcentricScaleBias is apply_concentric_scale_bias in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) applyConcentricScaleBias() {
	ctx := &p.ctx.twoPointConicalGradient

	// Apply t = t * scale + bias for concentric gradients
	x := p.r
	p.r = x.Mul(wide.SplatF32x8(ctx.p0)).Add(wide.SplatF32x8(ctx.p1))

	p.nextStage()
}

// gammaExpand2 is gamma_expand_2 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpand2() {
	p.r = p.r.Mul(p.r)
	p.g = p.g.Mul(p.g)
	p.b = p.b.Mul(p.b)

	p.nextStage()
}

// gammaExpandDst2 is gamma_expand_dst_2 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpandDst2() {
	p.dr = p.dr.Mul(p.dr)
	p.dg = p.dg.Mul(p.dg)
	p.db = p.db.Mul(p.db)

	p.nextStage()
}

// gammaCompress2 is gamma_compress_2 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaCompress2() {
	p.r = p.r.Sqrt()
	p.g = p.g.Sqrt()
	p.b = p.b.Sqrt()

	p.nextStage()
}

// gammaExpand22 is gamma_expand_22 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpand22() {
	p.r = p.r.Powf(2.2)
	p.g = p.g.Powf(2.2)
	p.b = p.b.Powf(2.2)

	p.nextStage()
}

// gammaExpandDst22 is gamma_expand_dst_22 in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpandDst22() {
	p.dr = p.dr.Powf(2.2)
	p.dg = p.dg.Powf(2.2)
	p.db = p.db.Powf(2.2)

	p.nextStage()
}

// gammaCompress22 is gamma_compress_22 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaCompress22() {
	p.r = p.r.Powf(0.45454545)
	p.g = p.g.Powf(0.45454545)
	p.b = p.b.Powf(0.45454545)

	p.nextStage()
}

// srgbExpand is srgb_expand in tiny-skia src/pipeline/highp.rs.
func srgbExpand(x wide.F32x8) wide.F32x8 {
	small := x.CmpLe(wide.SplatF32x8(0.04045))
	linear := x.Div(wide.SplatF32x8(12.92))
	exp := x.Add(wide.SplatF32x8(0.055)).Div(wide.SplatF32x8(1.055)).Powf(2.4)
	return small.Blend(linear, exp)
}

// srgbCompress is srgb_compress in tiny-skia src/pipeline/highp.rs.
func srgbCompress(x wide.F32x8) wide.F32x8 {
	small := x.CmpLe(wide.SplatF32x8(0.0031308))
	linear := x.Mul(wide.SplatF32x8(12.92))
	exp := x.Powf(0.416666666).Mul(wide.SplatF32x8(1.055)).Sub(wide.SplatF32x8(0.055))
	return small.Blend(linear, exp)
}

// gammaExpandSrgb is gamma_expand_srgb in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpandSrgb() {
	p.r = srgbExpand(p.r)
	p.g = srgbExpand(p.g)
	p.b = srgbExpand(p.b)

	p.nextStage()
}

// gammaExpandDstSrgb is gamma_expand_dst_srgb in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) gammaExpandDstSrgb() {
	p.dr = srgbExpand(p.dr)
	p.dg = srgbExpand(p.dg)
	p.db = srgbExpand(p.db)

	p.nextStage()
}

// gammaCompressSrgb is gamma_compress_srgb in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) gammaCompressSrgb() {
	p.r = srgbCompress(p.r)
	p.g = srgbCompress(p.g)
	p.b = srgbCompress(p.b)

	p.nextStage()
}

// justReturn is just_return in tiny-skia src/pipeline/highp.rs.
func (*highpPipeline) justReturn() {
	// Ends the loop.
}

// condToMask is cond_to_mask in tiny-skia src/pipeline/highp.rs.
func condToMask(cond wide.U32x8) wide.U32x8 {
	var mask wide.U32x8
	for i := range highpStageWidth {
		if cond[i] != 0 {
			mask[i] = ^uint32(0)
		}
	}
	return mask
}

// load8888Highp is load_8888 in tiny-skia src/pipeline/highp.rs.
//
// Port: data holds the bytes of STAGE_WIDTH pixels.
func load8888Highp(
	data *[highpStageWidth * BytesPerPixel]byte,
	r, g, b, a *wide.F32x8,
) {
	// Surprisingly, `f32 * FACTOR` is way faster than `f32x8 * f32x8::splat(FACTOR)`.

	const factor float32 = 1.0 / 255.0

	for i := range highpStageWidth {
		r[i] = float32(data[i*BytesPerPixel]) * factor
		g[i] = float32(data[i*BytesPerPixel+1]) * factor
		b[i] = float32(data[i*BytesPerPixel+2]) * factor
		a[i] = float32(data[i*BytesPerPixel+3]) * factor
	}
}

// load8888TailHighp is load_8888_tail in tiny-skia src/pipeline/highp.rs.
func load8888TailHighp(
	tail int, data PixelSlice,
	r, g, b, a *wide.F32x8,
) {
	// Fill a dummy array with `tail` values. `tail` is always in a 1..STAGE_WIDTH-1 range.
	// This way we can reuse the `load_8888_` method and remove any branches.
	var tmp [highpStageWidth * BytesPerPixel]byte
	copy(tmp[0:tail*BytesPerPixel], data[0:tail*BytesPerPixel])
	load8888Highp(&tmp, r, g, b, a)
}

// store8888Highp is store_8888 in tiny-skia src/pipeline/highp.rs.
//
// Port: data holds the bytes of STAGE_WIDTH pixels.
func store8888Highp(
	r, g, b, a *wide.F32x8,
	data *[highpStageWidth * BytesPerPixel]byte,
) {
	ri := unnorm(r)
	gi := unnorm(g)
	bi := unnorm(b)
	ai := unnorm(a)

	for i := range highpStageWidth {
		data[i*BytesPerPixel] = uint8(ri[i])
		data[i*BytesPerPixel+1] = uint8(gi[i])
		data[i*BytesPerPixel+2] = uint8(bi[i])
		data[i*BytesPerPixel+3] = uint8(ai[i])
	}
}

// store8888TailHighp is store_8888_tail in tiny-skia src/pipeline/highp.rs.
func store8888TailHighp(
	r, g, b, a *wide.F32x8,
	tail int, data PixelSlice,
) {
	ri := unnorm(r)
	gi := unnorm(g)
	bi := unnorm(b)
	ai := unnorm(a)

	// This is better than `for i in 0..tail`, because this way the compiler
	// knows that we have only 4 steps and slices access is guarantee to be valid.
	// This removes bounds checking and a possible panic call.
	for i := range highpStageWidth {
		data.Set(i, premultipliedColorU8FromRGBAUnchecked(
			uint8(ri[i]), uint8(gi[i]), uint8(bi[i]), uint8(ai[i]),
		))

		if i+1 == tail {
			break
		}
	}
}

// unnorm is unnorm in tiny-skia src/pipeline/highp.rs.
func unnorm(v *wide.F32x8) wide.I32x8 {
	return v.Max(wide.F32x8{}).Min(wide.SplatF32x8(1.0)).Mul(wide.SplatF32x8(255.0)).RoundInt()
}

// invHighp is inv in tiny-skia src/pipeline/highp.rs.
func invHighp(v wide.F32x8) wide.F32x8 {
	return wide.SplatF32x8(1.0).Sub(v)
}

// two is two in tiny-skia src/pipeline/highp.rs.
func two(v wide.F32x8) wide.F32x8 {
	return v.Add(v)
}

// madHighp is mad in tiny-skia src/pipeline/highp.rs.
func madHighp(f, m, a wide.F32x8) wide.F32x8 {
	return f.Mul(m).Add(a)
}

// lerpHighp is lerp in tiny-skia src/pipeline/highp.rs.
func lerpHighp(from, to, t wide.F32x8) wide.F32x8 {
	return madHighp(to.Sub(from), t, from)
}
