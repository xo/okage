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
	// Port (D33): one loop on the lanes. Upstream: r * a, g * a, b * a.
	for i := range p.r {
		a := p.a[i]
		p.r[i] = float32(p.r[i] * a)
		p.g[i] = float32(p.g[i] * a)
		p.b[i] = float32(p.b[i] * a)
	}

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
	// Port (D33): one loop on the lanes. Upstream: r.max(f32x8::default()),
	// and the same for g, b and a.
	for i := range p.r {
		p.r[i] = wide.LaneMax(p.r[i], 0)
		p.g[i] = wide.LaneMax(p.g[i], 0)
		p.b[i] = wide.LaneMax(p.b[i], 0)
		p.a[i] = wide.LaneMax(p.a[i], 0)
	}

	p.nextStage()
}

// clampA is clamp_a in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) clampA() {
	// Port (D33): one loop on the lanes. Upstream: r.min(f32x8::splat(1.0)),
	// and the same for g, b and a.
	for i := range p.r {
		p.r[i] = wide.LaneMin(p.r[i], 1.0)
		p.g[i] = wide.LaneMin(p.g[i], 1.0)
		p.b[i] = wide.LaneMin(p.b[i], 1.0)
		p.a[i] = wide.LaneMin(p.a[i], 1.0)
	}

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

	// Port (D33): one loop on the lanes. Upstream: f32x8::splat(dx as f32) +
	// iota.
	dx := float32(p.dx)
	for i := range p.r {
		p.r[i] = dx + iota[i]
	}
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

	// Port (D33): one loop on the lanes, with gatherIxLane for each lane.
	width := int32(pixmap.Width())
	var ix wide.U32x8
	for i := range ix {
		ix[i] = gatherIxLane(x[i], y[i], w, h, width)
	}
	return ix
}

// gatherIxLane is gatherIx on one lane (D33), with w and h already made
// inclusive. Upstream: x.max(0).min(w), y.max(0).min(h), and then
// (y.trunc_int() * width + x.trunc_int()).to_u32x8_bitcast().
func gatherIxLane(x, y, w, h float32, width int32) uint32 {
	x = wide.LaneMin(wide.LaneMax(x, 0), w)
	y = wide.LaneMin(wide.LaneMax(y, 0), h)
	return uint32(wide.TruncI32(y)*width + wide.TruncI32(x))
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

	// Port (D33): one loop on the lanes. Upstream: r * c, and the same for
	// g, b and a.
	for i := range c {
		p.r[i] = float32(p.r[i] * c[i])
		p.g[i] = float32(p.g[i] * c[i])
		p.b[i] = float32(p.b[i] * c[i])
		p.a[i] = float32(p.a[i] * c[i])
	}

	p.nextStage()
}

// scaleU8 is scale_u8 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) scaleU8() {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	// Port (D33): one loop on the lanes. Upstream: c / 255, then r * c, and
	// the same for g, b and a.
	for i := range c {
		ci := c[i] / 255.0
		p.r[i] = float32(p.r[i] * ci)
		p.g[i] = float32(p.g[i] * ci)
		p.b[i] = float32(p.b[i] * ci)
		p.a[i] = float32(p.a[i] * ci)
	}

	p.nextStage()
}

// lerpU8 is lerp_u8 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) lerpU8() {
	// Load u8xTail and cast it to f32x8.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.F32x8{float32(data[0]), float32(data[1]), 0.0, 0.0, 0.0, 0.0, 0.0, 0.0}

	// Port (D33): one loop on the lanes. Upstream: c / 255, then
	// lerp(dr, r, c), and the same for g, b and a.
	for i := range c {
		ci := c[i] / 255.0
		p.r[i] = lerpHighp(p.dr[i], p.r[i], ci)
		p.g[i] = lerpHighp(p.dg[i], p.g[i], ci)
		p.b[i] = lerpHighp(p.db[i], p.b[i], ci)
		p.a[i] = lerpHighp(p.da[i], p.a[i], ci)
	}

	p.nextStage()
}

// scale1Float is scale_1_float in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) scale1Float() {
	c := p.ctx.currentCoverage

	// Port (D33): one loop on the lanes. Upstream: r * c, and the same for
	// g, b and a.
	for i := range p.r {
		p.r[i] = float32(p.r[i] * c)
		p.g[i] = float32(p.g[i] * c)
		p.b[i] = float32(p.b[i] * c)
		p.a[i] = float32(p.a[i] * c)
	}

	p.nextStage()
}

// lerp1Float is lerp_1_float in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) lerp1Float() {
	c := p.ctx.currentCoverage

	// Port (D33): one loop on the lanes. Upstream: lerp(dr, r, c), and the
	// same for g, b and a.
	for i := range p.r {
		p.r[i] = lerpHighp(p.dr[i], p.r[i], c)
		p.g[i] = lerpHighp(p.dg[i], p.g[i], c)
		p.b[i] = lerpHighp(p.db[i], p.b[i], c)
		p.a[i] = lerpHighp(p.da[i], p.a[i], c)
	}

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
	// Port (D33): one loop on the lanes. Upstream: s * da + d * inv(sa).
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i]*da[i]) + float32(d[i]*invHighp(sa[i]))
	}
	return r
}

// destinationAtop is the stage destination_atop of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationAtop() { p.blendFn(destinationAtopHighp) }

// destinationAtopHighp is the function of the stage destination_atop in
// tiny-skia src/pipeline/highp.rs.
func destinationAtopHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: d * sa + s * inv(da).
	var r wide.F32x8
	for i := range r {
		r[i] = float32(d[i]*sa[i]) + float32(s[i]*invHighp(da[i]))
	}
	return r
}

// sourceIn is the stage source_in of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceIn() { p.blendFn(sourceInHighp) }

// sourceInHighp is the function of the stage source_in in tiny-skia
// src/pipeline/highp.rs.
func sourceInHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s * da.
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i] * da[i])
	}
	return r
}

// destinationIn is the stage destination_in of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationIn() { p.blendFn(destinationInHighp) }

// destinationInHighp is the function of the stage destination_in in
// tiny-skia src/pipeline/highp.rs.
func destinationInHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: d * sa.
	var r wide.F32x8
	for i := range r {
		r[i] = float32(d[i] * sa[i])
	}
	return r
}

// sourceOut is the stage source_out of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOut() { p.blendFn(sourceOutHighp) }

// sourceOutHighp is the function of the stage source_out in tiny-skia
// src/pipeline/highp.rs.
func sourceOutHighp(s, _, _, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s * inv(da).
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i] * invHighp(da[i]))
	}
	return r
}

// destinationOut is the stage destination_out of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationOut() { p.blendFn(destinationOutHighp) }

// destinationOutHighp is the function of the stage destination_out in
// tiny-skia src/pipeline/highp.rs.
func destinationOutHighp(_, d, sa, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: d * inv(sa).
	var r wide.F32x8
	for i := range r {
		r[i] = float32(d[i] * invHighp(sa[i]))
	}
	return r
}

// sourceOver is the stage source_over of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOver() { p.blendFn(sourceOverHighp) }

// sourceOverHighp is the function of the stage source_over in tiny-skia
// src/pipeline/highp.rs.
func sourceOverHighp(s, d, sa, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: mad(d, inv(sa), s).
	var r wide.F32x8
	for i := range r {
		r[i] = madHighp(d[i], invHighp(sa[i]), s[i])
	}
	return r
}

// destinationOver is the stage destination_over of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) destinationOver() { p.blendFn(destinationOverHighp) }

// destinationOverHighp is the function of the stage destination_over in
// tiny-skia src/pipeline/highp.rs.
func destinationOverHighp(s, d, _, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: mad(s, inv(da), d).
	var r wide.F32x8
	for i := range r {
		r[i] = madHighp(s[i], invHighp(da[i]), d[i])
	}
	return r
}

// modulate is the stage modulate of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) modulate() { p.blendFn(modulateHighp) }

// modulateHighp is the function of the stage modulate in tiny-skia
// src/pipeline/highp.rs.
func modulateHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s * d.
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i] * d[i])
	}
	return r
}

// multiply is the stage multiply of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) multiply() { p.blendFn(multiplyHighp) }

// multiplyHighp is the function of the stage multiply in tiny-skia
// src/pipeline/highp.rs.
func multiplyHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s * inv(da) + d * inv(sa) + s * d.
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i]*invHighp(da[i])) + float32(d[i]*invHighp(sa[i])) + float32(s[i]*d[i])
	}
	return r
}

// screen is the stage screen of blend_fn! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) screen() { p.blendFn(screenHighp) }

// screenHighp is the function of the stage screen in tiny-skia
// src/pipeline/highp.rs.
func screenHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s + d - s * d.
	var r wide.F32x8
	for i := range r {
		r[i] = s[i] + d[i] - float32(s[i]*d[i])
	}
	return r
}

// xor is the stage xor of blend_fn! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xor() { p.blendFn(xorHighp) }

// xorHighp is the function of the stage xor in tiny-skia
// src/pipeline/highp.rs.
func xorHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s * inv(da) + d * inv(sa).
	var r wide.F32x8
	for i := range r {
		r[i] = float32(s[i]*invHighp(da[i])) + float32(d[i]*invHighp(sa[i]))
	}
	return r
}

// plus is the stage plus of blend_fn! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) plus() { p.blendFn(plusHighp) }

// plusHighp is the function of the stage plus in tiny-skia
// src/pipeline/highp.rs.
func plusHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: (s + d).min(f32x8::splat(1.0)).
	var r wide.F32x8
	for i := range r {
		r[i] = wide.LaneMin(s[i]+d[i], 1.0)
	}
	return r
}

// blendFn2 applies f to each color channel and source_over to the alpha, as
// the macro blend_fn2! of tiny-skia src/pipeline/highp.rs does, and calls the
// next stage.
func (p *highpPipeline) blendFn2(f func(s, d, sa, da wide.F32x8) wide.F32x8) {
	// The same logic applied to color, and source_over for alpha.
	p.r = f(p.r, p.dr, p.a, p.da)
	p.g = f(p.g, p.dg, p.a, p.da)
	p.b = f(p.b, p.db, p.a, p.da)
	// Port (D33): one loop on the lanes.
	for i := range p.a {
		p.a[i] = madHighp(p.da[i], invHighp(p.a[i]), p.a[i])
	}

	p.nextStage()
}

// darken is the stage darken of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) darken() { p.blendFn2(darkenHighp) }

// darkenHighp is the function of the stage darken in tiny-skia
// src/pipeline/highp.rs.
func darkenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s + d - (s * da).max(d * sa).
	var r wide.F32x8
	for i := range r {
		r[i] = s[i] + d[i] - wide.LaneMax(float32(s[i]*da[i]), float32(d[i]*sa[i]))
	}
	return r
}

// lighten is the stage lighten of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) lighten() { p.blendFn2(lightenHighp) }

// lightenHighp is the function of the stage lighten in tiny-skia
// src/pipeline/highp.rs.
func lightenHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s + d - (s * da).min(d * sa).
	var r wide.F32x8
	for i := range r {
		r[i] = s[i] + d[i] - wide.LaneMin(float32(s[i]*da[i]), float32(d[i]*sa[i]))
	}
	return r
}

// difference is the stage difference of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) difference() { p.blendFn2(differenceHighp) }

// differenceHighp is the function of the stage difference in tiny-skia
// src/pipeline/highp.rs.
func differenceHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s + d - two((s * da).min(d * sa)).
	var r wide.F32x8
	for i := range r {
		r[i] = s[i] + d[i] - two(wide.LaneMin(float32(s[i]*da[i]), float32(d[i]*sa[i])))
	}
	return r
}

// exclusion is the stage exclusion of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) exclusion() { p.blendFn2(exclusionHighp) }

// exclusionHighp is the function of the stage exclusion in tiny-skia
// src/pipeline/highp.rs.
func exclusionHighp(s, d, _, _ wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream: s + d - two(s * d).
	var r wide.F32x8
	for i := range r {
		r[i] = s[i] + d[i] - two(float32(s[i]*d[i]))
	}
	return r
}

// colorBurn is the stage color_burn of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) colorBurn() { p.blendFn2(colorBurnHighp) }

// colorBurnHighp is the function of the stage color_burn in tiny-skia
// src/pipeline/highp.rs.
func colorBurnHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes, with colorBurnHighpLane for each
	// lane.
	var r wide.F32x8
	for i := range r {
		r[i] = colorBurnHighpLane(s[i], d[i], sa[i], da[i])
	}
	return r
}

// colorBurnHighpLane is colorBurnHighp on one lane (D33). Upstream:
//
//	d.cmp_eq(da).blend(
//	    d + s * inv(da),
//	    s.cmp_eq(f32x8::default()).blend(
//	        d * inv(sa),
//	        sa * (da - da.min((da - d) * sa * s.recip_fast())) + s * inv(da) + d * inv(sa),
//	    ),
//	)
func colorBurnHighpLane(s, d, sa, da float32) float32 {
	if d == da {
		return d + float32(s*invHighp(da))
	}
	if s == 0 {
		return float32(d * invHighp(sa))
	}
	m := wide.LaneMin(da, float32(float32((da-d)*sa)*(1/s)))
	return float32(sa*(da-m)) + float32(s*invHighp(da)) + float32(d*invHighp(sa))
}

// colorDodge is the stage color_dodge of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) colorDodge() { p.blendFn2(colorDodgeHighp) }

// colorDodgeHighp is the function of the stage color_dodge in tiny-skia
// src/pipeline/highp.rs.
func colorDodgeHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes, with colorDodgeHighpLane for each
	// lane.
	var r wide.F32x8
	for i := range r {
		r[i] = colorDodgeHighpLane(s[i], d[i], sa[i], da[i])
	}
	return r
}

// colorDodgeHighpLane is colorDodgeHighp on one lane (D33). Upstream:
//
//	d.cmp_eq(f32x8::default()).blend(
//	    s * inv(da),
//	    s.cmp_eq(sa).blend(
//	        s + d * inv(sa),
//	        sa * da.min((d * sa) * (sa - s).recip_fast()) + s * inv(da) + d * inv(sa),
//	    ),
//	)
func colorDodgeHighpLane(s, d, sa, da float32) float32 {
	if d == 0 {
		return float32(s * invHighp(da))
	}
	if s == sa {
		return s + float32(d*invHighp(sa))
	}
	m := wide.LaneMin(da, float32(float32(d*sa)*(1/(sa-s))))
	return float32(sa*m) + float32(s*invHighp(da)) + float32(d*invHighp(sa))
}

// hardLight is the stage hard_light of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) hardLight() { p.blendFn2(hardLightHighp) }

// hardLightHighp is the function of the stage hard_light in tiny-skia
// src/pipeline/highp.rs.
func hardLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream:
	//
	//	s * inv(da) + d * inv(sa) + two(s).cmp_le(sa).blend(
	//	    two(s * d),
	//	    sa * da - two((da - d) * (sa - s)),
	//	)
	var r wide.F32x8
	for i := range r {
		si, di, sai, dai := s[i], d[i], sa[i], da[i]
		var v float32
		if two(si) <= sai {
			v = two(float32(si * di))
		} else {
			v = float32(sai*dai) - two(float32((dai-di)*(sai-si)))
		}
		r[i] = float32(si*invHighp(dai)) + float32(di*invHighp(sai)) + v
	}
	return r
}

// overlay is the stage overlay of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) overlay() { p.blendFn2(overlayHighp) }

// overlayHighp is the function of the stage overlay in tiny-skia
// src/pipeline/highp.rs.
func overlayHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes. Upstream:
	//
	//	s * inv(da) + d * inv(sa) + two(d).cmp_le(da).blend(
	//	    two(s * d),
	//	    sa * da - two((da - d) * (sa - s)),
	//	)
	var r wide.F32x8
	for i := range r {
		si, di, sai, dai := s[i], d[i], sa[i], da[i]
		var v float32
		if two(di) <= dai {
			v = two(float32(si * di))
		} else {
			v = float32(sai*dai) - two(float32((dai-di)*(sai-si)))
		}
		r[i] = float32(si*invHighp(dai)) + float32(di*invHighp(sai)) + v
	}
	return r
}

// softLight is the stage soft_light of blend_fn2! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) softLight() { p.blendFn2(softLightHighp) }

// softLightHighp is the function of the stage soft_light in tiny-skia
// src/pipeline/highp.rs.
func softLightHighp(s, d, sa, da wide.F32x8) wide.F32x8 {
	// Port (D33): one loop on the lanes, with softLightHighpLane for each
	// lane.
	var r wide.F32x8
	for i := range r {
		r[i] = softLightHighpLane(s[i], d[i], sa[i], da[i])
	}
	return r
}

// softLightHighpLane is softLightHighp on one lane (D33). It computes only
// the branch that the lane takes. Upstream:
//
//	let m = da.cmp_gt(f32x8::default()).blend(d / da, f32x8::default());
//	let s2 = two(s);
//	let m4 = two(two(m));
//	let dark_src = d * (sa + (s2 - sa) * (1.0 - m));
//	let dark_dst = (m4 * m4 + m4) * (m - 1.0) + 7.0 * m;
//	let lite_dst = m.sqrt() - m;
//	let lite_src = d * sa + da * (s2 - sa)
//	    * two(two(d)).cmp_le(da).blend(dark_dst, lite_dst);
//	s * inv(da) + d * inv(sa) + s2.cmp_le(sa).blend(dark_src, lite_src)
func softLightHighpLane(s, d, sa, da float32) float32 {
	const one = 1.0
	var m float32
	if da > 0 {
		m = d / da
	}
	s2 := two(s)
	m4 := two(two(m))

	// The logic forks three ways:
	//    1. dark src?
	//    2. light src, dark dst?
	//    3. light src, light dst?
	var v float32
	if s2 <= sa { // 1 or (2 or 3)?
		v = float32(d * (sa + float32((s2-sa)*(one-m)))) // darkSrc
	} else {
		var dst float32
		if two(two(d)) <= da { // 2 or 3?
			dst = float32((float32(m4*m4)+m4)*(m-one)) + float32(7.0*m) // darkDst
		} else {
			dst = wide.LaneSqrt(m) - m // liteDst
		}
		v = float32(d*sa) + float32(float32(da*(s2-sa))*dst) // liteSrc
	}

	return float32(s*invHighp(da)) + float32(d*invHighp(sa)) + v
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
	// Port (D33): one loop on the lanes, with the lane versions of sat, lum,
	// setSat, setLum and clipColor.
	for i := range r {
		ri, gi, bi, ai := r[i], g[i], b[i], a[i]
		dri, dgi, dbi, dai := dr[i], dg[i], db[i], da[i]

		rr := float32(ri * ai)
		gg := float32(gi * ai)
		bb := float32(bi * ai)

		rr, gg, bb = setSat(rr, gg, bb, float32(sat(dri, dgi, dbi)*ai))
		rr, gg, bb = setLum(rr, gg, bb, float32(lum(dri, dgi, dbi)*ai))
		rr, gg, bb = clipColor(rr, gg, bb, float32(ai*dai))

		r[i] = float32(ri*invHighp(dai)) + float32(dri*invHighp(ai)) + rr
		g[i] = float32(gi*invHighp(dai)) + float32(dgi*invHighp(ai)) + gg
		b[i] = float32(bi*invHighp(dai)) + float32(dbi*invHighp(ai)) + bb
		a[i] = ai + dai - float32(ai*dai)
	}

	return r, g, b, a
}

// saturation is the stage saturation of blend_fn3! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) saturation() { p.blendFn3(saturationK) }

// saturationK is saturation_k in tiny-skia src/pipeline/highp.rs.
func saturationK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	// Port (D33): one loop on the lanes, with the lane versions of sat, lum,
	// setSat, setLum and clipColor.
	for i := range r {
		ri, gi, bi, ai := r[i], g[i], b[i], a[i]
		dri, dgi, dbi, dai := dr[i], dg[i], db[i], da[i]

		rr := float32(dri * ai)
		gg := float32(dgi * ai)
		bb := float32(dbi * ai)

		rr, gg, bb = setSat(rr, gg, bb, float32(sat(ri, gi, bi)*dai))
		rr, gg, bb = setLum(rr, gg, bb, float32(lum(dri, dgi, dbi)*ai)) // (This is not redundant.)
		rr, gg, bb = clipColor(rr, gg, bb, float32(ai*dai))

		r[i] = float32(ri*invHighp(dai)) + float32(dri*invHighp(ai)) + rr
		g[i] = float32(gi*invHighp(dai)) + float32(dgi*invHighp(ai)) + gg
		b[i] = float32(bi*invHighp(dai)) + float32(dbi*invHighp(ai)) + bb
		a[i] = ai + dai - float32(ai*dai)
	}

	return r, g, b, a
}

// color is the stage color of blend_fn3! in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) color() { p.blendFn3(colorK) }

// colorK is color_k in tiny-skia src/pipeline/highp.rs.
func colorK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	// Port (D33): one loop on the lanes, with the lane versions of sat, lum,
	// setSat, setLum and clipColor.
	for i := range r {
		ri, gi, bi, ai := r[i], g[i], b[i], a[i]
		dri, dgi, dbi, dai := dr[i], dg[i], db[i], da[i]

		rr := float32(ri * dai)
		gg := float32(gi * dai)
		bb := float32(bi * dai)

		rr, gg, bb = setLum(rr, gg, bb, float32(lum(dri, dgi, dbi)*ai))
		rr, gg, bb = clipColor(rr, gg, bb, float32(ai*dai))

		r[i] = float32(ri*invHighp(dai)) + float32(dri*invHighp(ai)) + rr
		g[i] = float32(gi*invHighp(dai)) + float32(dgi*invHighp(ai)) + gg
		b[i] = float32(bi*invHighp(dai)) + float32(dbi*invHighp(ai)) + bb
		a[i] = ai + dai - float32(ai*dai)
	}

	return r, g, b, a
}

// luminosity is the stage luminosity of blend_fn3! in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) luminosity() { p.blendFn3(luminosityK) }

// luminosityK is luminosity_k in tiny-skia src/pipeline/highp.rs.
func luminosityK(r, g, b, a, dr, dg, db, da wide.F32x8) (wide.F32x8, wide.F32x8, wide.F32x8, wide.F32x8) {
	// Port (D33): one loop on the lanes, with the lane versions of sat, lum,
	// setSat, setLum and clipColor.
	for i := range r {
		ri, gi, bi, ai := r[i], g[i], b[i], a[i]
		dri, dgi, dbi, dai := dr[i], dg[i], db[i], da[i]

		rr := float32(dri * ai)
		gg := float32(dgi * ai)
		bb := float32(dbi * ai)

		rr, gg, bb = setLum(rr, gg, bb, float32(lum(ri, gi, bi)*dai))
		rr, gg, bb = clipColor(rr, gg, bb, float32(ai*dai))

		r[i] = float32(ri*invHighp(dai)) + float32(dri*invHighp(ai)) + rr
		g[i] = float32(gi*invHighp(dai)) + float32(dgi*invHighp(ai)) + gg
		b[i] = float32(bi*invHighp(dai)) + float32(dbi*invHighp(ai)) + bb
		a[i] = ai + dai - float32(ai*dai)
	}

	return r, g, b, a
}

// sat is sat in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, because only the loops of the stages of
// blend_fn3! call it. Upstream: r.max(g.max(b)) - r.min(g.min(b)).
func sat(r, g, b float32) float32 {
	return wide.LaneMax(r, wide.LaneMax(g, b)) - wide.LaneMin(r, wide.LaneMin(g, b))
}

// lum is lum in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as sat does. Upstream:
// r * 0.30 + g * 0.59 + b * 0.11.
func lum(r, g, b float32) float32 {
	return float32(r*0.30) + float32(g*0.59) + float32(b*0.11)
}

// setSat is set_sat in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as sat does. Upstream changes r, g and b
// in place. Go returns them.
func setSat(r, g, b, s float32) (float32, float32, float32) {
	mn := wide.LaneMin(r, wide.LaneMin(g, b))
	mx := wide.LaneMax(r, wide.LaneMax(g, b))
	sat := mx - mn

	// Map min channel to 0, max channel to s, and scale the middle proportionally.
	//
	// Port: upstream has a closure scale, sat.cmp_eq(0).blend(0,
	// (c - mn) * s / sat). Go writes it out for each channel.
	if sat == 0 {
		return 0, 0, 0
	}
	return float32((r-mn)*s) / sat, float32((g-mn)*s) / sat, float32((b-mn)*s) / sat
}

// setLum is set_lum in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as setSat does.
func setLum(r, g, b, l float32) (float32, float32, float32) {
	diff := l - lum(r, g, b)
	return r + diff, g + diff, b + diff
}

// clipColor is clip_color in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as setSat does.
func clipColor(r, g, b, a float32) (float32, float32, float32) {
	mn := wide.LaneMin(r, wide.LaneMin(g, b))
	mx := wide.LaneMax(r, wide.LaneMax(g, b))
	l := lum(r, g, b)

	return clipColorChannel(r, mn, mx, l, a),
		clipColorChannel(g, mn, mx, l, a),
		clipColorChannel(b, mn, mx, l, a)
}

// clipColorChannel is the closure clip of clip_color in tiny-skia
// src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as sat does. Upstream:
//
//	let c = mx.cmp_ge(0).blend(c, l + (c - l) * l / (l - mn));
//	let c = mx.cmp_gt(a).blend(l + (c - l) * (a - l) / (mx - l), c);
//	c.max(0)
func clipColorChannel(c, mn, mx, l, a float32) float32 {
	if !(mx >= 0) {
		c = l + float32((c-l)*l)/(l-mn)
	}
	if mx > a {
		c = l + float32((c-l)*(a-l))/(mx-l)
	}
	c = wide.LaneMax(c, 0) // Sometimes without this we can dip just a little negative.
	return c
}

// sourceOverRgba is source_over_rgba in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) sourceOverRgba() {
	pixels := p.pixmapDst.slice4AtXY(p.dx, p.dy)

	// Port (D33): one loop on the lanes. It loads a pixel as load_8888 does,
	// computes mad(dr, inv(a), r) for each channel, and stores the pixel as
	// store_8888 does.
	const factor float32 = 1.0 / 255.0
	for i := range highpStageWidth {
		px := (*[BytesPerPixel]byte)(pixels[i*BytesPerPixel:])
		dr := float32(float32(px[0]) * factor)
		dg := float32(float32(px[1]) * factor)
		db := float32(float32(px[2]) * factor)
		da := float32(float32(px[3]) * factor)
		p.dr[i], p.dg[i], p.db[i], p.da[i] = dr, dg, db, da

		invA := invHighp(p.a[i])
		r := madHighp(dr, invA, p.r[i])
		g := madHighp(dg, invA, p.g[i])
		b := madHighp(db, invA, p.b[i])
		a := madHighp(da, invA, p.a[i])
		p.r[i], p.g[i], p.b[i], p.a[i] = r, g, b, a

		px[0] = uint8(unnormLane(r))
		px[1] = uint8(unnormLane(g))
		px[2] = uint8(unnormLane(b))
		px[3] = uint8(unnormLane(a))
	}

	p.nextStage()
}

// sourceOverRgbaTail is source_over_rgba_tail in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) sourceOverRgbaTail() {
	pixels := p.pixmapDst.sliceAtXY(p.dx, p.dy)
	load8888TailHighp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)

	// Port (D33): one loop on the lanes. Upstream: mad(dr, inv(a), r), and
	// the same for g, b and a.
	for i := range p.a {
		invA := invHighp(p.a[i])
		p.r[i] = madHighp(p.dr[i], invA, p.r[i])
		p.g[i] = madHighp(p.dg[i], invA, p.g[i])
		p.b[i] = madHighp(p.db[i], invA, p.b[i])
		p.a[i] = madHighp(p.da[i], invA, p.a[i])
	}
	store8888TailHighp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)

	p.nextStage()
}

// transform is transform in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) transform() {
	ts := &p.ctx.transform

	// Port (D33): one loop on the lanes. Upstream:
	// mad(r, sx, mad(g, kx, tx)) and mad(r, ky, mad(g, sy, ty)).
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = madHighp(x, ts.SX, madHighp(y, ts.KX, ts.TX))
		p.g[i] = madHighp(x, ts.KY, madHighp(y, ts.SY, ts.TY))
	}

	p.nextStage()
}

// Tile x or y to [0,limit) == [0,limit - 1 ulp] (think, sampling from images).
// The gather stages hard clamp the output of these stages to [0,limit)...
// we just need to do the basic repeat or mirroring.

// reflect is reflect in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) reflect() {
	// Port (D33): one loop on the lanes, with exclusiveReflect for each lane.
	cx := &p.ctx.limitX
	cy := &p.ctx.limitY
	for i := range p.r {
		p.r[i] = exclusiveReflect(p.r[i], cx.scale, cx.invScale)
		p.g[i] = exclusiveReflect(p.g[i], cy.scale, cy.invScale)
	}

	p.nextStage()
}

// exclusiveReflect is exclusive_reflect in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, because only loops on the lanes call it.
// Upstream:
// ((v - limit) - (limit + limit) * ((v - limit) * (inv_limit * 0.5)).floor() - limit).abs().
func exclusiveReflect(v, limit, invLimit float32) float32 {
	vl := v - limit
	f := wide.LaneFloor(float32(vl * float32(invLimit*0.5)))
	return wide.LaneAbs(vl - float32((limit+limit)*f) - limit)
}

// repeat is repeat in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) repeat() {
	// Port (D33): one loop on the lanes, with exclusiveRepeat for each lane.
	cx := &p.ctx.limitX
	cy := &p.ctx.limitY
	for i := range p.r {
		p.r[i] = exclusiveRepeat(p.r[i], cx.scale, cx.invScale)
		p.g[i] = exclusiveRepeat(p.g[i], cy.scale, cy.invScale)
	}

	p.nextStage()
}

// exclusiveRepeat is exclusive_repeat in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as exclusiveReflect does. Upstream:
// v - (v * inv_limit).floor() * limit.
func exclusiveRepeat(v, limit, invLimit float32) float32 {
	return v - float32(wide.LaneFloor(float32(v*invLimit))*limit)
}

// bilinear is bilinear in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) bilinear() {
	x := p.r
	y := p.g

	// Port (D33): one loop on the lanes. Upstream: fx = (x + 0.5).fract(),
	// wx = [1.0 - fx, fx], and the same for y.
	var wx, wy [2]wide.F32x8
	for i := range x {
		fx := x[i] + 0.5
		fx -= wide.LaneFloor(fx)
		fy := y[i] + 0.5
		fy -= wide.LaneFloor(fy)
		wx[0][i], wx[1][i] = 1.0-fx, fx
		wy[0][i], wy[1][i] = 1.0-fy, fy
	}

	sampler2x2(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// bicubic is bicubic in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) bicubic() {
	x := p.r
	y := p.g

	// Port (D33): one loop on the lanes. Upstream: fx = (x + 0.5).fract(),
	// wx = [bicubic_far(1.0 - fx), bicubic_near(1.0 - fx), bicubic_near(fx),
	// bicubic_far(fx)], and the same for y.
	var wx, wy [4]wide.F32x8
	for i := range x {
		fx := x[i] + 0.5
		fx -= wide.LaneFloor(fx)
		fy := y[i] + 0.5
		fy -= wide.LaneFloor(fy)
		wx[0][i], wx[1][i], wx[2][i], wx[3][i] = bicubicFar(1.0-fx), bicubicNear(1.0-fx), bicubicNear(fx), bicubicFar(fx)
		wy[0][i], wy[1][i], wy[2][i], wy[3][i] = bicubicFar(1.0-fy), bicubicNear(1.0-fy), bicubicNear(fy), bicubicFar(fy)
	}

	sampler4x4(p.pixmapSrc, &p.ctx.sampler, x, y, &wx, &wy, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// In bicubic interpolation, the 16 pixels and +/- 0.5 and +/- 1.5 offsets from the sample
// pixel center are combined with a non-uniform cubic filter, with higher values near the center.
//
// We break this function into two parts, one for near 0.5 offsets and one for far 1.5 offsets.

// bicubicNear is bicubic_near in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, because only the loop of bicubic calls
// it.
func bicubicNear(t float32) float32 {
	// 1/18 + 9/18t + 27/18t^2 - 21/18t^3 == t ( t ( -21/18t + 27/18) + 9/18) + 1/18
	return madHighp(
		t,
		madHighp(t,
			madHighp(
				-21.0/18.0,
				t,
				27.0/18.0,
			),
			9.0/18.0,
		),
		1.0/18.0,
	)
}

// bicubicFar is bicubic_far in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as bicubicNear does.
func bicubicFar(t float32) float32 {
	// 0/18 + 0/18*t - 6/18t^2 + 7/18t^3 == t^2 (7/18t - 6/18)
	return float32(float32(t*t) * madHighp(7.0/18.0, t, -6.0/18.0))
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

			// Port (D33): one loop on the lanes. Upstream: w = wx[i] * wy[j],
			// then r = mad(w, rr, r), the same for g, b and a, and x + 1.0.
			for l := range highpStageWidth {
				w := float32(wx[i][l] * wy[j][l])
				r[l] = madHighp(w, rr[l], r[l])
				g[l] = madHighp(w, gg[l], g[l])
				b[l] = madHighp(w, bb[l], b[l])
				a[l] = madHighp(w, aa[l], a[l])
				x[l] += 1.0
			}
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

			// Port (D33): one loop on the lanes. Upstream: w = wx[i] * wy[j],
			// then r = mad(w, rr, r), the same for g, b and a, and x + 1.0.
			for l := range highpStageWidth {
				w := float32(wx[i][l] * wy[j][l])
				r[l] = madHighp(w, rr[l], r[l])
				g[l] = madHighp(w, gg[l], g[l])
				b[l] = madHighp(w, bb[l], b[l])
				a[l] = madHighp(w, aa[l], a[l])
				x[l] += 1.0
			}
		}

		y = y.Add(one)
	}
}

// sample is sample in tiny-skia src/pipeline/highp.rs.
func sample(
	pixmap PixmapRef, ctx *samplerCtx, x, y wide.F32x8,
	r, g, b, a *wide.F32x8,
) {
	// Port (D33): one loop on the lanes. For each lane, it tiles x and y,
	// computes the index as gather_ix does, and loads the pixel as gather
	// and load_8888 do.
	width := float32(pixmap.Width())
	height := float32(pixmap.Height())
	// Exclusive -> inclusive.
	w := ulpSub(width)
	h := ulpSub(height)
	stride := int32(pixmap.Width())
	pixels := pixmap.Data()
	const factor float32 = 1.0 / 255.0
	for i := range highpStageWidth {
		xi := tile(x[i], ctx.spreadMode, width, ctx.invWidth)
		yi := tile(y[i], ctx.spreadMode, height, ctx.invHeight)
		j := int(gatherIxLane(xi, yi, w, h, stride)) * BytesPerPixel
		px := (*[BytesPerPixel]byte)(pixels[j : j+BytesPerPixel])
		r[i] = float32(float32(px[0]) * factor)
		g[i] = float32(float32(px[1]) * factor)
		b[i] = float32(float32(px[2]) * factor)
		a[i] = float32(float32(px[3]) * factor)
	}
}

// tile is tile in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, because only the loop of sample calls it.
func tile(v float32, mode SpreadMode, limit, invLimit float32) float32 {
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
	// Port (D33): one loop on the lanes. Upstream: x.normalize().
	for i := range p.r {
		p.r[i] = wide.LaneMin(wide.LaneMax(p.r[i], 0), 1.0)
	}

	p.nextStage()
}

// reflectX1 is reflect_x1 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) reflectX1() {
	// Port (D33): one loop on the lanes. Upstream:
	// ((x - 1.0) - two(((x - 1.0) * 0.5).floor()) - 1.0).abs().normalize().
	for i := range p.r {
		x := p.r[i] - 1.0
		x = wide.LaneAbs(x - two(wide.LaneFloor(float32(x*0.5))) - 1.0)
		p.r[i] = wide.LaneMin(wide.LaneMax(x, 0), 1.0)
	}

	p.nextStage()
}

// repeatX1 is repeat_x1 in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) repeatX1() {
	// Port (D33): one loop on the lanes. Upstream: (x - x.floor()).normalize().
	for i := range p.r {
		x := p.r[i] - wide.LaneFloor(p.r[i])
		p.r[i] = wide.LaneMin(wide.LaneMax(x, 0), 1.0)
	}

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
		// Port (D33): one loop on the lanes, which adds n to idx.
		for j := range idx {
			idx[j] += b2u32(t[j] >= tt)
		}
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
	//
	// Port (D33): one loop on the lanes, which also computes mad(t, f, b)
	// for each channel.
	for i := range highpStageWidth {
		f := &ctx.factors[idx[i]]
		c := &ctx.biases[idx[i]]
		ti := t[i]
		r[i] = madHighp(ti, f.r, c.r)
		g[i] = madHighp(ti, f.g, c.g)
		b[i] = madHighp(ti, f.b, c.b)
		a[i] = madHighp(ti, f.a, c.a)
	}
}

// evenlySpaced2StopGradient is evenly_spaced_2_stop_gradient in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) evenlySpaced2StopGradient() {
	ctx := &p.ctx.evenlySpaced2StopGradient

	// Port (D33): one loop on the lanes. Upstream: mad(t, factor.r, bias.r),
	// and the same for g, b and a.
	f := ctx.factor
	c := ctx.bias
	for i := range p.r {
		t := p.r[i]
		p.r[i] = madHighp(t, f.r, c.r)
		p.g[i] = madHighp(t, f.g, c.g)
		p.b[i] = madHighp(t, f.b, c.b)
		p.a[i] = madHighp(t, f.a, c.a)
	}

	p.nextStage()
}

// xyToUnitAngle is xy_to_unit_angle in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xyToUnitAngle() {
	// Port (D33): one loop on the lanes. Each lane computes the expression of
	// upstream, and an if takes the place of each blend.
	for i := range p.r {
		x := p.r[i]
		y := p.g[i]
		xAbs := wide.LaneAbs(x)
		yAbs := wide.LaneAbs(y)
		slope := wide.LaneMin(xAbs, yAbs) / wide.LaneMax(xAbs, yAbs)
		s := float32(slope * slope)
		// Use a 7th degree polynomial to approximate atan.
		// This was generated using sollya.gforge.inria.fr.
		// A float optimized polynomial was generated using the following command.
		// P1 = fpminimax((1/(2*Pi))*atan(x),[|1,3,5,7|],[|24...|],[2^(-40),1],relative);
		phi := float32(slope * (0.15912117063999176025390625 +
			float32(s*(-5.185396969318389892578125e-2+
				float32(s*(2.476101927459239959716796875e-2+
					float32(s*-7.0547382347285747528076171875e-3)))))))
		if xAbs < yAbs {
			phi = 0.25 - phi
		}
		if x < 0.0 {
			phi = 0.5 - phi
		}
		if y < 0.0 {
			phi = 1.0 - phi
		}
		if phi != phi {
			phi = 0.0
		}
		p.r[i] = phi
	}
	p.nextStage()
}

// xyToRadius is xy_to_radius in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) xyToRadius() {
	// Port (D33): one loop on the lanes. Upstream: (x * x + y * y).sqrt().
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = wide.LaneSqrt(float32(x*x) + float32(y*y))
	}

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

	// Port (D33): one loop on the lanes. Upstream:
	// (x * x + y * y).sqrt() - x * p0.
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = wide.LaneSqrt(float32(x*x)+float32(y*y)) - float32(x*ctx.p0)
	}

	p.nextStage()
}

// xyTo2PtConicalGreater is xy_to_2pt_conical_greater in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalGreater() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream:
	// (x * x - y * y).sqrt() - x * p0.
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = wide.LaneSqrt(float32(x*x)-float32(y*y)) - float32(x*ctx.p0)
	}

	p.nextStage()
}

// xyTo2PtConicalSmaller is xy_to_2pt_conical_smaller in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalSmaller() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream:
	// -(x * x - y * y).sqrt() - x * p0, where the negation is 0 - v.
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = (0 - wide.LaneSqrt(float32(x*x)-float32(y*y))) - float32(x*ctx.p0)
	}

	p.nextStage()
}

// xyTo2PtConicalStrip is xy_to_2pt_conical_strip in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) xyTo2PtConicalStrip() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream: x + (p0 - y * y).sqrt().
	for i := range p.r {
		x, y := p.r[i], p.g[i]
		p.r[i] = x + wide.LaneSqrt(ctx.p0-float32(y*y))
	}

	p.nextStage()
}

// mask2PtConicalNan is mask_2pt_conical_nan in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) mask2PtConicalNan() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream: is_degenerate =
	// t.cmp_ne(t), r = is_degenerate.blend(0, t), and the mask is true where
	// is_degenerate is false.
	var notDegenerate wide.U32x8
	for i := range p.r {
		t := p.r[i]
		if t != t {
			p.r[i] = 0
		} else {
			notDegenerate[i] = ^uint32(0)
		}
	}
	ctx.mask = condToMask(notDegenerate)

	p.nextStage()
}

// mask2PtConicalDegenerates is mask_2pt_conical_degenerates in tiny-skia
// src/pipeline/highp.rs.
func (p *highpPipeline) mask2PtConicalDegenerates() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream: is_degenerate =
	// t.cmp_le(0) | t.cmp_ne(t), r = is_degenerate.blend(0, t), and the mask
	// is true where is_degenerate is false.
	var notDegenerate wide.U32x8
	for i := range p.r {
		t := p.r[i]
		if t <= 0 || t != t {
			p.r[i] = 0
		} else {
			notDegenerate[i] = ^uint32(0)
		}
	}
	ctx.mask = condToMask(notDegenerate)

	p.nextStage()
}

// applyVectorMask is apply_vector_mask in tiny-skia src/pipeline/highp.rs.
func (p *highpPipeline) applyVectorMask() {
	ctx := &p.ctx.twoPointConicalGradient

	// Port (D33): one loop on the lanes. Upstream:
	// (r.to_u32x8_bitcast() & mask).to_f32x8_bitcast(), and the same for g, b
	// and a.
	for i := range p.r {
		m := ctx.mask[i]
		p.r[i] = math.Float32frombits(math.Float32bits(p.r[i]) & m)
		p.g[i] = math.Float32frombits(math.Float32bits(p.g[i]) & m)
		p.b[i] = math.Float32frombits(math.Float32bits(p.b[i]) & m)
		p.a[i] = math.Float32frombits(math.Float32bits(p.a[i]) & m)
	}

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
	//
	// Port (D33): one loop on the lanes.
	for i := range p.r {
		p.r[i] = float32(p.r[i]*ctx.p0) + ctx.p1
	}

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
	// Port (D33): one loop on the lanes, with unnormLane for each lane.
	for i := range highpStageWidth {
		data[i*BytesPerPixel] = uint8(unnormLane(r[i]))
		data[i*BytesPerPixel+1] = uint8(unnormLane(g[i]))
		data[i*BytesPerPixel+2] = uint8(unnormLane(b[i]))
		data[i*BytesPerPixel+3] = uint8(unnormLane(a[i]))
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
	// Port (D33): one loop on the lanes, with unnormLane for each lane.
	var r wide.I32x8
	for i := range r {
		r[i] = unnormLane(v[i])
	}
	return r
}

// unnormLane is unnorm on one lane (D33). Upstream:
// (v.max(0).min(1.0) * 255.0).round_int().
func unnormLane(v float32) int32 {
	return wide.LaneRoundInt(float32(wide.LaneMin(wide.LaneMax(v, 0), 1.0) * 255.0))
}

// invHighp is inv in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, because only loops on the lanes call it.
func invHighp(v float32) float32 {
	return 1.0 - v
}

// two is two in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as invHighp does.
func two(v float32) float32 {
	return v + v
}

// madHighp is mad in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as invHighp does. The conversion rounds
// the product, so that the Go compiler cannot fuse it with the add (D16).
func madHighp(f, m, a float32) float32 {
	return float32(f*m) + a
}

// lerpHighp is lerp in tiny-skia src/pipeline/highp.rs.
//
// Port (D33): it works on one lane, as invHighp does.
func lerpHighp(from, to, t float32) float32 {
	return madHighp(to-from, t, from)
}
