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

// This file is tiny-skia src/pipeline/lowp.rs, a low precision raster
// pipeline implementation.
//
// A lowp pipeline uses u16 instead of f32 for math. Because of that, it does
// not implement stages that require high precision. The pipeline compiler
// automatically decides which one to use.
//
// Skia uses u16x8 (128bit) types for a generic CPU and u16x16 (256bit) for
// modern x86 CPUs. But instead of explicit SIMD instructions, it mainly relies
// on clang's vector extensions. And since they are unavailable in Rust, we
// have to do everything manually.
//
// According to our benchmarks, a SIMD-accelerated u16x8 in Rust is almost 2x
// slower than in Skia. Not sure why. For example, there is no div instruction
// for u16x8, so we have to use a basic scalar version. Which means
// unnecessary load/store. No idea what clang does in this case. Surprisingly,
// a SIMD-accelerated u16x8 is even slower than a scalar one. Again, not sure
// why.
//
// Therefore we are using scalar u16x16 by default and relying on rustc/llvm
// auto vectorization instead. When targeting a generic CPU, we are just 5-10%
// slower than Skia. While u16x8 is 30-40% slower. And while
// `-C target-cpu=haswell` boosts our performance by around 25%, we are still
// 40-60% behind Skia built for Haswell.
//
// On ARM AArch64 the story is different and explicit SIMD makes our code up
// to 2-3x faster.

// lowpStageWidth is STAGE_WIDTH in tiny-skia src/pipeline/lowp.rs.
const lowpStageWidth = 16

// lowpPipeline is Pipeline in tiny-skia src/pipeline/lowp.rs.
//
// Port: each stage is a method of lowpPipeline (D10).
type lowpPipeline struct {
	index     int
	functions []func(*lowpPipeline)
	pixmap    *subPixmapMut
	maskCtx   maskCtx
	aaMaskCtx aaMaskCtx
	ctx       *context
	r         wide.U16x16
	g         wide.U16x16
	b         wide.U16x16
	a         wide.U16x16
	dr        wide.U16x16
	dg        wide.U16x16
	db        wide.U16x16
	da        wide.U16x16
	tail      int
	dx        int
	dy        int
}

// nextStage is Pipeline::next_stage in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) nextStage() {
	next := p.functions[p.index]
	p.index++
	next(p)
}

// lowpStages is STAGES in tiny-skia src/pipeline/lowp.rs. It must be in the
// same order as the values of stage.
//
// Port: upstream puts null_fn where lowp has no implementation. Go puts nil
// there, so that compile can find such a stage (see compile).
var lowpStages = [stagesCount]func(*lowpPipeline){
	(*lowpPipeline).moveSourceToDestination,
	(*lowpPipeline).moveDestinationToSource,
	nil, // Clamp0
	nil, // ClampA
	(*lowpPipeline).premultiply,
	(*lowpPipeline).uniformColor,
	(*lowpPipeline).seedShader,
	(*lowpPipeline).loadDst,
	(*lowpPipeline).store,
	(*lowpPipeline).loadDstU8,
	(*lowpPipeline).storeU8,
	nil, // Gather
	(*lowpPipeline).loadMaskU8,
	(*lowpPipeline).maskU8,
	(*lowpPipeline).scaleU8,
	(*lowpPipeline).lerpU8,
	(*lowpPipeline).scale1Float,
	(*lowpPipeline).lerp1Float,
	(*lowpPipeline).destinationAtop,
	(*lowpPipeline).destinationIn,
	(*lowpPipeline).destinationOut,
	(*lowpPipeline).destinationOver,
	(*lowpPipeline).sourceAtop,
	(*lowpPipeline).sourceIn,
	(*lowpPipeline).sourceOut,
	(*lowpPipeline).sourceOver,
	(*lowpPipeline).clear,
	(*lowpPipeline).modulate,
	(*lowpPipeline).multiply,
	(*lowpPipeline).plus,
	(*lowpPipeline).screen,
	(*lowpPipeline).xor,
	nil, // ColorBurn
	nil, // ColorDodge
	(*lowpPipeline).darken,
	(*lowpPipeline).difference,
	(*lowpPipeline).exclusion,
	(*lowpPipeline).hardLight,
	(*lowpPipeline).lighten,
	(*lowpPipeline).overlay,
	nil, // SoftLight
	nil, // Hue
	nil, // Saturation
	nil, // Color
	nil, // Luminosity
	(*lowpPipeline).sourceOverRgba,
	(*lowpPipeline).transform,
	nil, // Reflect
	nil, // Repeat
	nil, // Bilinear
	nil, // Bicubic
	(*lowpPipeline).padX1,
	(*lowpPipeline).reflectX1,
	(*lowpPipeline).repeatX1,
	(*lowpPipeline).gradient,
	(*lowpPipeline).evenlySpaced2StopGradient,
	// TODO: Can be implemented for lowp as well. The implementation is very similar to its highp
	// variant.
	nil, // XYToUnitAngle
	(*lowpPipeline).xyToRadius,
	nil, // XYTo2PtConicalFocalOnCircle
	nil, // XYTo2PtConicalWellBehaved
	nil, // XYTo2PtConicalSmaller
	nil, // XYTo2PtConicalGreater
	nil, // XYTo2PtConicalStrip
	nil, // Mask2PtConicalNan
	nil, // Mask2PtConicalDegenerates
	nil, // ApplyVectorMask
	nil, // Alter2PtConicalCompensateFocal
	nil, // Alter2PtConicalUnswap
	nil, // NegateX
	nil, // ApplyConcentricScaleBias
	nil, // GammaExpand2
	nil, // GammaExpandDestination2
	nil, // GammaCompress2
	nil, // GammaExpand22
	nil, // GammaExpandDestination22
	nil, // GammaCompress22
	nil, // GammaExpandSrgb
	nil, // GammaExpandDestinationSrgb
	nil, // GammaCompressSrgb
}

// startLowp is start in tiny-skia src/pipeline/lowp.rs.
//
// Port: upstream makes the Pipeline here. Go takes it from the caller, so
// that a run does not allocate (see rasterPipeline).
func startLowp(
	p *lowpPipeline,
	functions []func(*lowpPipeline),
	functionsTail []func(*lowpPipeline),
	rect *screenIntRect,
	aaMaskCtx aaMaskCtx,
	maskCtx maskCtx,
	ctx *context,
	pixmap *subPixmapMut,
) {
	*p = lowpPipeline{
		index:     0,
		functions: nil,
		pixmap:    pixmap,
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
		for x+lowpStageWidth <= end {
			p.index = 0
			p.dx = x
			p.dy = int(y)
			p.tail = lowpStageWidth
			p.nextStage()
			x += lowpStageWidth
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
// src/pipeline/lowp.rs.
func (p *lowpPipeline) moveSourceToDestination() {
	p.dr = p.r
	p.dg = p.g
	p.db = p.b
	p.da = p.a

	p.nextStage()
}

// moveDestinationToSource is move_destination_to_source in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) moveDestinationToSource() {
	p.r = p.dr
	p.g = p.dg
	p.b = p.db
	p.a = p.da

	p.nextStage()
}

// premultiply is premultiply in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) premultiply() {
	for i := range lowpStageWidth {
		a := p.a[i]
		p.r[i] = div255l(p.r[i] * a)
		p.g[i] = div255l(p.g[i] * a)
		p.b[i] = div255l(p.b[i] * a)
	}

	p.nextStage()
}

// uniformColor is uniform_color in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) uniformColor() {
	ctx := &p.ctx.uniformColor
	p.r = wide.SplatU16x16(ctx.rgba[0])
	p.g = wide.SplatU16x16(ctx.rgba[1])
	p.b = wide.SplatU16x16(ctx.rgba[2])
	p.a = wide.SplatU16x16(ctx.rgba[3])

	p.nextStage()
}

// seedShader is seed_shader in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) seedShader() {
	iota := wide.F32x16{
		wide.F32x8{0.5, 1.5, 2.5, 3.5, 4.5, 5.5, 6.5, 7.5},
		wide.F32x8{8.5, 9.5, 10.5, 11.5, 12.5, 13.5, 14.5, 15.5},
	}

	x := wide.SplatF32x16(float32(p.dx)).Add(iota)
	y := wide.SplatF32x16(float32(p.dy) + 0.5)
	split(&x, &p.r, &p.g)
	split(&y, &p.b, &p.a)

	p.nextStage()
}

// loadDst is load_dst in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) loadDst() {
	load8888Lowp(p.pixmap.slice16AtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
	p.nextStage()
}

// loadDstTail is load_dst_tail in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) loadDstTail() {
	load8888TailLowp(p.tail, p.pixmap.sliceAtXY(p.dx, p.dy), &p.dr, &p.dg, &p.db, &p.da)
	p.nextStage()
}

// store is store in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) store() {
	store8888Lowp(&p.r, &p.g, &p.b, &p.a, p.pixmap.slice16AtXY(p.dx, p.dy))
	p.nextStage()
}

// storeTail is store_tail in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) storeTail() {
	store8888TailLowp(&p.r, &p.g, &p.b, &p.a, p.tail, p.pixmap.sliceAtXY(p.dx, p.dy))
	p.nextStage()
}

// loadDstU8 is load_dst_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) loadDstU8() {
	load8(p.pixmap.slice16MaskAtXY(p.dx, p.dy), &p.da)
	p.nextStage()
}

// loadDstU8Tail is load_dst_u8_tail in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) loadDstU8Tail() {
	// Fill a dummy array with `tail` values. `tail` is always in a 1..STAGE_WIDTH-1 range.
	// This way we can reuse the `load_8888__` method and remove any branches.
	data := p.pixmap.sliceMaskAtXY(p.dx, p.dy)
	var tmp [lowpStageWidth]uint8
	copy(tmp[0:p.tail], data[0:p.tail])
	load8(&tmp, &p.da)

	p.nextStage()
}

// storeU8 is store_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) storeU8() {
	data := p.pixmap.slice16MaskAtXY(p.dx, p.dy)
	a := p.a.AsSlice()

	data[0] = uint8(a[0])
	data[1] = uint8(a[1])
	data[2] = uint8(a[2])
	data[3] = uint8(a[3])
	data[4] = uint8(a[4])
	data[5] = uint8(a[5])
	data[6] = uint8(a[6])
	data[7] = uint8(a[7])
	data[8] = uint8(a[8])
	data[9] = uint8(a[9])
	data[10] = uint8(a[10])
	data[11] = uint8(a[11])
	data[12] = uint8(a[12])
	data[13] = uint8(a[13])
	data[14] = uint8(a[14])
	data[15] = uint8(a[15])

	p.nextStage()
}

// storeU8Tail is store_u8_tail in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) storeU8Tail() {
	data := p.pixmap.sliceMaskAtXY(p.dx, p.dy)
	a := p.a.AsSlice()

	// This is better than `for i in 0..tail`, because this way the compiler
	// knows that we have only 16 steps and slices access is guarantee to be valid.
	// This removes bounds checking and a possible panic call.
	for i := range lowpStageWidth {
		data[i] = uint8(a[i])

		if i+1 == p.tail {
			break
		}
	}

	p.nextStage()
}

// loadMaskU8 is load_mask_u8 in tiny-skia src/pipeline/lowp.rs.
//
// It is similar to maskU8, but it only loads the mask values without actually
// masking the pipeline.
func (p *lowpPipeline) loadMaskU8() {
	offset := p.maskCtx.offset(p.dx, p.dy)

	var c wide.U16x16
	for i := range p.tail {
		c[i] = uint16(p.maskCtx.data[offset+i])
	}

	p.r = wide.SplatU16x16(0)
	p.g = wide.SplatU16x16(0)
	p.b = wide.SplatU16x16(0)
	p.a = c

	p.nextStage()
}

// maskU8 is mask_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) maskU8() {
	offset := p.maskCtx.offset(p.dx, p.dy)

	var c wide.U16x16
	for i := range p.tail {
		c[i] = uint16(p.maskCtx.data[offset+i])
	}

	if c.Equal(wide.U16x16{}) {
		return
	}

	// Port (D33): one loop on the lanes.
	for i := range lowpStageWidth {
		p.r[i] = div255l(p.r[i] * c[i])
		p.g[i] = div255l(p.g[i] * c[i])
		p.b[i] = div255l(p.b[i] * c[i])
		p.a[i] = div255l(p.a[i] * c[i])
	}

	p.nextStage()
}

// scaleU8 is scale_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) scaleU8() {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{uint16(data[0]), uint16(data[1])}

	// Port (D33): one loop on the lanes.
	for i := range lowpStageWidth {
		p.r[i] = div255l(p.r[i] * c[i])
		p.g[i] = div255l(p.g[i] * c[i])
		p.b[i] = div255l(p.b[i] * c[i])
		p.a[i] = div255l(p.a[i] * c[i])
	}

	p.nextStage()
}

// lerpU8 is lerp_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) lerpU8() {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{uint16(data[0]), uint16(data[1])}

	// Port (D33): one loop on the lanes.
	for i := range lowpStageWidth {
		p.r[i] = lerpl(p.dr[i], p.r[i], c[i])
		p.g[i] = lerpl(p.dg[i], p.g[i], c[i])
		p.b[i] = lerpl(p.db[i], p.b[i], c[i])
		p.a[i] = lerpl(p.da[i], p.a[i], c[i])
	}

	p.nextStage()
}

// scale1Float is scale_1_float in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) scale1Float() {
	c := fromFloat(p.ctx.currentCoverage)[0]

	// Port (D33): one loop on the lanes. Each lane of from_float is the same.
	for i := range lowpStageWidth {
		p.r[i] = div255l(p.r[i] * c)
		p.g[i] = div255l(p.g[i] * c)
		p.b[i] = div255l(p.b[i] * c)
		p.a[i] = div255l(p.a[i] * c)
	}

	p.nextStage()
}

// lerp1Float is lerp_1_float in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) lerp1Float() {
	c := fromFloat(p.ctx.currentCoverage)[0]

	// Port (D33): one loop on the lanes. Each lane of from_float is the same.
	for i := range lowpStageWidth {
		p.r[i] = lerpl(p.dr[i], p.r[i], c)
		p.g[i] = lerpl(p.dg[i], p.g[i], c)
		p.b[i] = lerpl(p.db[i], p.b[i], c)
		p.a[i] = lerpl(p.da[i], p.a[i], c)
	}

	p.nextStage()
}

// The stages below are the blend_fn! macro of tiny-skia
// src/pipeline/lowp.rs. Each one applies its function to each channel, with
// the source, the destination, the source alpha and the destination alpha.
//
// Port: Go has no macro, so each stage writes out the expression of its
// function for each channel.

// clear is the stage clear of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) clear() {
	p.r = wide.SplatU16x16(0)
	p.g = wide.SplatU16x16(0)
	p.b = wide.SplatU16x16(0)
	p.a = wide.SplatU16x16(0)

	p.nextStage()
}

// sourceAtop is the stage source_atop of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceAtop() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * da + d * inv(sa)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i]*da + p.dr[i]*invl(sa))
		p.g[i] = div255l(p.g[i]*da + p.dg[i]*invl(sa))
		p.b[i] = div255l(p.b[i]*da + p.db[i]*invl(sa))
		p.a[i] = div255l(sa*da + da*invl(sa))
	}

	p.nextStage()
}

// destinationAtop is the stage destination_atop of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationAtop() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(d * sa + s * inv(da)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.dr[i]*sa + p.r[i]*invl(da))
		p.g[i] = div255l(p.dg[i]*sa + p.g[i]*invl(da))
		p.b[i] = div255l(p.db[i]*sa + p.b[i]*invl(da))
		p.a[i] = div255l(da*sa + sa*invl(da))
	}

	p.nextStage()
}

// sourceIn is the stage source_in of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceIn() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * da).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i] * da)
		p.g[i] = div255l(p.g[i] * da)
		p.b[i] = div255l(p.b[i] * da)
		p.a[i] = div255l(sa * da)
	}

	p.nextStage()
}

// destinationIn is the stage destination_in of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationIn() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(d * sa).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.dr[i] * sa)
		p.g[i] = div255l(p.dg[i] * sa)
		p.b[i] = div255l(p.db[i] * sa)
		p.a[i] = div255l(da * sa)
	}

	p.nextStage()
}

// sourceOut is the stage source_out of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOut() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * inv(da)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i] * invl(da))
		p.g[i] = div255l(p.g[i] * invl(da))
		p.b[i] = div255l(p.b[i] * invl(da))
		p.a[i] = div255l(sa * invl(da))
	}

	p.nextStage()
}

// destinationOut is the stage destination_out of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationOut() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(d * inv(sa)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.dr[i] * invl(sa))
		p.g[i] = div255l(p.dg[i] * invl(sa))
		p.b[i] = div255l(p.db[i] * invl(sa))
		p.a[i] = div255l(da * invl(sa))
	}

	p.nextStage()
}

// sourceOver is the stage source_over of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOver() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + div255(d * inv(sa)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += div255l(p.dr[i] * invl(sa))
		p.g[i] += div255l(p.dg[i] * invl(sa))
		p.b[i] += div255l(p.db[i] * invl(sa))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// destinationOver is the stage destination_over of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationOver() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// d + div255(s * inv(da)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = p.dr[i] + div255l(p.r[i]*invl(da))
		p.g[i] = p.dg[i] + div255l(p.g[i]*invl(da))
		p.b[i] = p.db[i] + div255l(p.b[i]*invl(da))
		p.a[i] = da + div255l(sa*invl(da))
	}

	p.nextStage()
}

// modulate is the stage modulate of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) modulate() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * d).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i] * p.dr[i])
		p.g[i] = div255l(p.g[i] * p.dg[i])
		p.b[i] = div255l(p.b[i] * p.db[i])
		p.a[i] = div255l(sa * da)
	}

	p.nextStage()
}

// multiply is the stage multiply of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) multiply() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * inv(da) + d * inv(sa) + s * d).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i]*invl(da) + p.dr[i]*invl(sa) + p.r[i]*p.dr[i])
		p.g[i] = div255l(p.g[i]*invl(da) + p.dg[i]*invl(sa) + p.g[i]*p.dg[i])
		p.b[i] = div255l(p.b[i]*invl(da) + p.db[i]*invl(sa) + p.b[i]*p.db[i])
		p.a[i] = div255l(sa*invl(da) + da*invl(sa) + sa*da)
	}

	p.nextStage()
}

// screen is the stage screen of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) screen() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + d - div255(s * d).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += p.dr[i] - div255l(p.r[i]*p.dr[i])
		p.g[i] += p.dg[i] - div255l(p.g[i]*p.dg[i])
		p.b[i] += p.db[i] - div255l(p.b[i]*p.db[i])
		p.a[i] = sa + da - div255l(sa*da)
	}

	p.nextStage()
}

// xor is the stage xor of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) xor() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * inv(da) + d * inv(sa)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i]*invl(da) + p.dr[i]*invl(sa))
		p.g[i] = div255l(p.g[i]*invl(da) + p.dg[i]*invl(sa))
		p.b[i] = div255l(p.b[i]*invl(da) + p.db[i]*invl(sa))
		p.a[i] = div255l(sa*invl(da) + da*invl(sa))
	}

	p.nextStage()
}

// plus is the stage plus of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) plus() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// (s + d).min(&u16x16::splat(255)).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = min(p.r[i]+p.dr[i], 255)
		p.g[i] = min(p.g[i]+p.dg[i], 255)
		p.b[i] = min(p.b[i]+p.db[i], 255)
		p.a[i] = min(sa+da, 255)
	}

	p.nextStage()
}

// The stages below are the blend_fn2! macro of tiny-skia
// src/pipeline/lowp.rs. Each one applies its function to each color
// channel, and source_over to the alpha.

// darken is the stage darken of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) darken() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + d - div255((s * da).max(&(d * sa))).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += p.dr[i] - div255l(max(p.r[i]*da, p.dr[i]*sa))
		p.g[i] += p.dg[i] - div255l(max(p.g[i]*da, p.dg[i]*sa))
		p.b[i] += p.db[i] - div255l(max(p.b[i]*da, p.db[i]*sa))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// lighten is the stage lighten of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) lighten() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + d - div255((s * da).min(&(d * sa))).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += p.dr[i] - div255l(min(p.r[i]*da, p.dr[i]*sa))
		p.g[i] += p.dg[i] - div255l(min(p.g[i]*da, p.dg[i]*sa))
		p.b[i] += p.db[i] - div255l(min(p.b[i]*da, p.db[i]*sa))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// exclusion is the stage exclusion of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) exclusion() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + d - u16x16::splat(2) * div255(s * d).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += p.dr[i] - 2*div255l(p.r[i]*p.dr[i])
		p.g[i] += p.dg[i] - 2*div255l(p.g[i]*p.dg[i])
		p.b[i] += p.db[i] - 2*div255l(p.b[i]*p.db[i])
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// difference is the stage difference of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) difference() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// s + d - u16x16::splat(2) * div255((s * da).min(&(d * sa))).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] += p.dr[i] - 2*div255l(min(p.r[i]*da, p.dr[i]*sa))
		p.g[i] += p.dg[i] - 2*div255l(min(p.g[i]*da, p.dg[i]*sa))
		p.b[i] += p.db[i] - 2*div255l(min(p.b[i]*da, p.db[i]*sa))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// hardLight is the stage hard_light of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) hardLight() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * inv(da) + d * inv(sa) + (s + s).cmp_le(&sa).blend(two(s * d), sa * da - two((sa - s) * (da - d)))).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i]*invl(da) + p.dr[i]*invl(sa) + hardLightTerm(p.r[i], p.dr[i], sa, da, p.r[i]+p.r[i] <= sa))
		p.g[i] = div255l(p.g[i]*invl(da) + p.dg[i]*invl(sa) + hardLightTerm(p.g[i], p.dg[i], sa, da, p.g[i]+p.g[i] <= sa))
		p.b[i] = div255l(p.b[i]*invl(da) + p.db[i]*invl(sa) + hardLightTerm(p.b[i], p.db[i], sa, da, p.b[i]+p.b[i] <= sa))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// overlay is the stage overlay of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) overlay() {
	// Port (D33): one loop on the lanes, which gives the same bits as the
	// methods of the vector types. The function of upstream is
	// div255(s * inv(da) + d * inv(sa) + (d + d).cmp_le(&da).blend(two(s * d), sa * da - two((sa - s) * (da - d)))).
	for i := range lowpStageWidth {
		sa, da := p.a[i], p.da[i]
		p.r[i] = div255l(p.r[i]*invl(da) + p.dr[i]*invl(sa) + hardLightTerm(p.r[i], p.dr[i], sa, da, p.dr[i]+p.dr[i] <= da))
		p.g[i] = div255l(p.g[i]*invl(da) + p.dg[i]*invl(sa) + hardLightTerm(p.g[i], p.dg[i], sa, da, p.dg[i]+p.dg[i] <= da))
		p.b[i] = div255l(p.b[i]*invl(da) + p.db[i]*invl(sa) + hardLightTerm(p.b[i], p.db[i], sa, da, p.db[i]+p.db[i] <= da))
		p.a[i] = sa + div255l(da*invl(sa))
	}

	p.nextStage()
}

// sourceOverRgba is source_over_rgba in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOverRgba() {
	pixels := p.pixmap.slice16AtXY(p.dx, p.dy)
	load8888Lowp(pixels, &p.dr, &p.dg, &p.db, &p.da)
	// Port (D33): one loop on the lanes, as the stage source_over.
	for i := range lowpStageWidth {
		ia := invl(p.a[i])
		p.r[i] += div255l(p.dr[i] * ia)
		p.g[i] += div255l(p.dg[i] * ia)
		p.b[i] += div255l(p.db[i] * ia)
		p.a[i] += div255l(p.da[i] * ia)
	}
	store8888Lowp(&p.r, &p.g, &p.b, &p.a, pixels)

	p.nextStage()
}

// sourceOverRgbaTail is source_over_rgba_tail in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOverRgbaTail() {
	pixels := p.pixmap.sliceAtXY(p.dx, p.dy)
	load8888TailLowp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)
	// Port (D33): one loop on the lanes, as the stage source_over.
	for i := range lowpStageWidth {
		ia := invl(p.a[i])
		p.r[i] += div255l(p.dr[i] * ia)
		p.g[i] += div255l(p.dg[i] * ia)
		p.b[i] += div255l(p.db[i] * ia)
		p.a[i] += div255l(p.da[i] * ia)
	}
	store8888TailLowp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)

	p.nextStage()
}

// transform is transform in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) transform() {
	ts := &p.ctx.transform

	// Port (D33): one loop on the lanes, with joinLane and splitLane for the
	// floats in r, g and b, a. Upstream: nx = mad(x, sx, mad(y, kx, tx)) and
	// ny = mad(x, ky, mad(y, sy, ty)).
	xs := [2]*wide.U16x16{&p.r, &p.g}
	ys := [2]*wide.U16x16{&p.b, &p.a}
	for h := range xs {
		xh, yh := xs[h], ys[h]
		for i := range 8 {
			x := joinLane(xh[2*i], xh[2*i+1])
			y := joinLane(yh[2*i], yh[2*i+1])
			nx := madLowp(x, ts.SX, madLowp(y, ts.KX, ts.TX))
			ny := madLowp(x, ts.KY, madLowp(y, ts.SY, ts.TY))
			xh[2*i], xh[2*i+1] = splitLane(nx)
			yh[2*i], yh[2*i+1] = splitLane(ny)
		}
	}

	p.nextStage()
}

// padX1 is pad_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) padX1() {
	// Port (D33): one loop on the lanes. Upstream: x.normalize().
	xs := [2]*wide.U16x16{&p.r, &p.g}
	for _, xh := range xs {
		for i := range 8 {
			x := joinLane(xh[2*i], xh[2*i+1])
			x = wide.LaneMin(wide.LaneMax(x, 0), 1.0)
			xh[2*i], xh[2*i+1] = splitLane(x)
		}
	}

	p.nextStage()
}

// reflectX1 is reflect_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) reflectX1() {
	// Port (D33): one loop on the lanes. Upstream: t = ((x - 1.0) *
	// 0.5).floor(), then ((x - 1.0) - two(t) - 1.0).abs().normalize(). The
	// floor of f32x16 rounds, see wide.LaneFloor16.
	xs := [2]*wide.U16x16{&p.r, &p.g}
	for _, xh := range xs {
		for i := range 8 {
			x := joinLane(xh[2*i], xh[2*i+1]) - 1.0
			t := wide.LaneFloor16(float32(x * 0.5))
			x = wide.LaneAbs(x - (t + t) - 1.0)
			x = wide.LaneMin(wide.LaneMax(x, 0), 1.0)
			xh[2*i], xh[2*i+1] = splitLane(x)
		}
	}

	p.nextStage()
}

// repeatX1 is repeat_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) repeatX1() {
	// Port (D33): one loop on the lanes. Upstream:
	// (x - x.floor()).normalize().
	xs := [2]*wide.U16x16{&p.r, &p.g}
	for _, xh := range xs {
		for i := range 8 {
			x := joinLane(xh[2*i], xh[2*i+1])
			x -= wide.LaneFloor16(x)
			x = wide.LaneMin(wide.LaneMax(x, 0), 1.0)
			xh[2*i], xh[2*i+1] = splitLane(x)
		}
	}

	p.nextStage()
}

// gradient is gradient in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) gradient() {
	ctx := &p.ctx.gradient

	// N.B. The loop starts at 1 because idx 0 is the color to use before the first stop.
	t := join(&p.r, &p.g)
	var idx wide.U16x16
	for i := 1; i < ctx.len; i++ {
		tt := ctx.tValues[i].Get()
		t0 := t[0]
		t1 := t[1]
		idx[0] += b2u16(t0[0] >= tt)
		idx[1] += b2u16(t0[1] >= tt)
		idx[2] += b2u16(t0[2] >= tt)
		idx[3] += b2u16(t0[3] >= tt)
		idx[4] += b2u16(t0[4] >= tt)
		idx[5] += b2u16(t0[5] >= tt)
		idx[6] += b2u16(t0[6] >= tt)
		idx[7] += b2u16(t0[7] >= tt)
		idx[8] += b2u16(t1[0] >= tt)
		idx[9] += b2u16(t1[1] >= tt)
		idx[10] += b2u16(t1[2] >= tt)
		idx[11] += b2u16(t1[3] >= tt)
		idx[12] += b2u16(t1[4] >= tt)
		idx[13] += b2u16(t1[5] >= tt)
		idx[14] += b2u16(t1[6] >= tt)
		idx[15] += b2u16(t1[7] >= tt)
	}
	gradientLookupLowp(ctx, &idx, t, &p.r, &p.g, &p.b, &p.a)

	p.nextStage()
}

// b2u16 is the Rust cast "b as u16" for a bool. It ports no upstream
// function.
func b2u16(b bool) uint16 {
	if b {
		return 1
	}
	return 0
}

// evenlySpaced2StopGradient is evenly_spaced_2_stop_gradient in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) evenlySpaced2StopGradient() {
	ctx := &p.ctx.evenlySpaced2StopGradient

	// Port (D33): one loop on the lanes after join. Upstream:
	// round_f32_to_u16 of mad(t, factor.r, bias.r), and the same for g, b
	// and a.
	t := join(&p.r, &p.g)
	f := ctx.factor
	c := ctx.bias
	for i := range lowpStageWidth {
		ti := t[i/8][i%8]
		p.r[i], p.g[i], p.b[i], p.a[i] = roundF32ToU16(
			madLowp(ti, f.r, c.r),
			madLowp(ti, f.g, c.g),
			madLowp(ti, f.b, c.b),
			madLowp(ti, f.a, c.a),
		)
	}

	p.nextStage()
}

// xyToRadius is xy_to_radius in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) xyToRadius() {
	// Port (D33): one loop on the lanes. Upstream: (x * x + y * y).sqrt().
	// y does not change, so the loop does not write b and a.
	xs := [2]*wide.U16x16{&p.r, &p.g}
	ys := [2]*wide.U16x16{&p.b, &p.a}
	for h := range xs {
		xh, yh := xs[h], ys[h]
		for i := range 8 {
			x := joinLane(xh[2*i], xh[2*i+1])
			y := joinLane(yh[2*i], yh[2*i+1])
			x = wide.LaneSqrt(float32(x*x) + float32(y*y))
			xh[2*i], xh[2*i+1] = splitLane(x)
		}
	}

	p.nextStage()
}

// gradientLookupLowp is gradient_lookup in tiny-skia src/pipeline/lowp.rs.
//
// We are using u16 for index, not u32 as Skia, to simplify the code a bit.
// The gradient creation code does not allow that many stops anyway.
func gradientLookupLowp(
	ctx *gradientCtx, idx *wide.U16x16, t wide.F32x16,
	r, g, b, a *wide.U16x16,
) {
	// Surprisingly, but bound checking does not affect the performance.
	// And since `idx` can contain any number, we leave it in place.
	//
	// Port: the macro gather! of upstream reads one field for each lane. Go
	// reads all the fields of a lane at once, which gives the same values.
	//
	// Port (D33): one loop on the lanes, which also computes
	// round_f32_to_u16 of mad(t, f, b) for each channel.
	for i := range lowpStageWidth {
		f := &ctx.factors[idx[i]]
		c := &ctx.biases[idx[i]]
		ti := t[i/8][i%8]
		r[i], g[i], b[i], a[i] = roundF32ToU16(
			madLowp(ti, f.r, c.r),
			madLowp(ti, f.g, c.g),
			madLowp(ti, f.b, c.b),
			madLowp(ti, f.a, c.a),
		)
	}
}

// roundF32ToU16 is round_f32_to_u16 in tiny-skia src/pipeline/lowp.rs.
//
// Port (D33): it works on one lane, because only loops on the lanes call it.
// Upstream changes r, g, b and a in place. Go returns them.
func roundF32ToU16(rf, gf, bf, af float32) (r, g, b, a uint16) {
	// TODO: can produce a slightly different result to Skia
	//       affects the two_stops_linear_mirror test

	// Upstream: rf.normalize() * 255.0 + 0.5, the same for gf and bf, and
	// af * 255.0 + 0.5, each saved with save_to_u16x16.
	rf = float32(wide.LaneMin(wide.LaneMax(rf, 0), 1.0)*255.0) + 0.5
	gf = float32(wide.LaneMin(wide.LaneMax(gf, 0), 1.0)*255.0) + 0.5
	bf = float32(wide.LaneMin(wide.LaneMax(bf, 0), 1.0)*255.0) + 0.5
	af = float32(af*255.0) + 0.5

	return wide.TruncU16(rf), wide.TruncU16(gf), wide.TruncU16(bf), wide.TruncU16(af)
}

// justReturn is just_return in tiny-skia src/pipeline/lowp.rs.
func (*lowpPipeline) justReturn() {
	// Ends the loop.
}

// load8888Lowp is load_8888 in tiny-skia src/pipeline/lowp.rs.
//
// Port: data holds the bytes of STAGE_WIDTH pixels.
func load8888Lowp(
	data *[lowpStageWidth * BytesPerPixel]byte,
	r, g, b, a *wide.U16x16,
) {
	for i := range lowpStageWidth {
		r[i] = uint16(data[i*BytesPerPixel])
		g[i] = uint16(data[i*BytesPerPixel+1])
		b[i] = uint16(data[i*BytesPerPixel+2])
		a[i] = uint16(data[i*BytesPerPixel+3])
	}
}

// load8888TailLowp is load_8888_tail in tiny-skia src/pipeline/lowp.rs.
func load8888TailLowp(
	tail int, data PixelSlice,
	r, g, b, a *wide.U16x16,
) {
	// Fill a dummy array with `tail` values. `tail` is always in a 1..STAGE_WIDTH-1 range.
	// This way we can reuse the `load_8888__` method and remove any branches.
	var tmp [lowpStageWidth * BytesPerPixel]byte
	copy(tmp[0:tail*BytesPerPixel], data[0:tail*BytesPerPixel])
	load8888Lowp(&tmp, r, g, b, a)
}

// store8888Lowp is store_8888 in tiny-skia src/pipeline/lowp.rs.
//
// Port: data holds the bytes of STAGE_WIDTH pixels.
func store8888Lowp(
	r, g, b, a *wide.U16x16,
	data *[lowpStageWidth * BytesPerPixel]byte,
) {
	rs := r.AsSlice()
	gs := g.AsSlice()
	bs := b.AsSlice()
	as := a.AsSlice()

	for i := range lowpStageWidth {
		data[i*BytesPerPixel] = uint8(rs[i])
		data[i*BytesPerPixel+1] = uint8(gs[i])
		data[i*BytesPerPixel+2] = uint8(bs[i])
		data[i*BytesPerPixel+3] = uint8(as[i])
	}
}

// store8888TailLowp is store_8888_tail in tiny-skia src/pipeline/lowp.rs.
func store8888TailLowp(
	r, g, b, a *wide.U16x16,
	tail int, data PixelSlice,
) {
	rs := r.AsSlice()
	gs := g.AsSlice()
	bs := b.AsSlice()
	as := a.AsSlice()

	// This is better than `for i in 0..tail`, because this way the compiler
	// knows that we have only 16 steps and slices access is guarantee to be valid.
	// This removes bounds checking and a possible panic call.
	for i := range lowpStageWidth {
		data.Set(i, premultipliedColorU8FromRGBAUnchecked(
			uint8(rs[i]), uint8(gs[i]), uint8(bs[i]), uint8(as[i]),
		))

		if i+1 == tail {
			break
		}
	}
}

// load8 is load_8 in tiny-skia src/pipeline/lowp.rs.
func load8(data *[lowpStageWidth]uint8, a *wide.U16x16) {
	*a = wide.U16x16{
		uint16(data[0]), uint16(data[1]), uint16(data[2]), uint16(data[3]),
		uint16(data[4]), uint16(data[5]), uint16(data[6]), uint16(data[7]),
		uint16(data[8]), uint16(data[9]), uint16(data[10]), uint16(data[11]),
		uint16(data[12]), uint16(data[13]), uint16(data[14]), uint16(data[15]),
	}
}

// div255 is div255 in tiny-skia src/pipeline/lowp.rs.
//
// Port (D33): one loop on the lanes, with div255l for each lane, gives the
// same bits as (v + 255) >> 8 with the methods of the vector types.
func div255(v wide.U16x16) wide.U16x16 {
	// Skia uses `vrshrq_n_u16(vrsraq_n_u16(v, v, 8), 8)` here when NEON is available,
	// but it does not affect performance much and breaks reproducible result. Ignore it.
	// NOTE: the compiler does not replace the division with a shift.
	var r wide.U16x16
	for i := range r {
		r[i] = div255l(v[i])
	}
	return r
}

// div255l is one lane of div255: (v + 255) / 256, with the sum wrapped to
// 16 bits as u16 wraps in Rust.
func div255l(v uint16) uint16 {
	return (v + 255) >> 8
}

// invLowp is inv in tiny-skia src/pipeline/lowp.rs.
func invLowp(v wide.U16x16) wide.U16x16 {
	return wide.SplatU16x16(255).Sub(v)
}

// invl is one lane of invLowp.
func invl(v uint16) uint16 {
	return 255 - v
}

// hardLightTerm is one lane of the blend of the stages hard_light and overlay
// in tiny-skia src/pipeline/lowp.rs: two(s * d) where le holds, and
// sa * da - two((sa - s) * (da - d)) elsewhere, with u16 wrapped.
func hardLightTerm(s, d, sa, da uint16, le bool) uint16 {
	if le {
		return 2 * s * d
	}
	return sa*da - 2*(sa-s)*(da-d)
}

// fromFloat is from_float in tiny-skia src/pipeline/lowp.rs.
func fromFloat(f float32) wide.U16x16 {
	return wide.SplatU16x16(wide.TruncU16(float32(f*255.0) + 0.5))
}

// lerpLowp is lerp in tiny-skia src/pipeline/lowp.rs.
//
// Port (D33): one loop on the lanes, with lerpl for each lane.
func lerpLowp(from, to, t wide.U16x16) wide.U16x16 {
	var r wide.U16x16
	for i := range r {
		r[i] = lerpl(from[i], to[i], t[i])
	}
	return r
}

// lerpl is one lane of lerpLowp: div255(from * inv(t) + to * t).
func lerpl(from, to, t uint16) uint16 {
	return div255l(from*invl(t) + to*t)
}

// split is split in tiny-skia src/pipeline/lowp.rs.
//
// Port: upstream casts the bytes with bytemuck, in the byte order of the
// machine. Go stores the low half of the bits of each float first. Only join
// reads them, so the order does not change a result.
func split(v *wide.F32x16, lo, hi *wide.U16x16) {
	// We are splitting f32x16 (512bit) into two u16x16 (256 bit).
	for i := range 8 {
		bits := math.Float32bits(v[0][i])
		lo[2*i] = uint16(bits)
		lo[2*i+1] = uint16(bits >> 16)
	}
	for i := range 8 {
		bits := math.Float32bits(v[1][i])
		hi[2*i] = uint16(bits)
		hi[2*i+1] = uint16(bits >> 16)
	}
}

// splitLane is split on one lane (D33). It gives the low and the high half
// of the bits of v.
func splitLane(v float32) (uint16, uint16) {
	bits := math.Float32bits(v)
	return uint16(bits), uint16(bits >> 16)
}

// join is join in tiny-skia src/pipeline/lowp.rs.
//
// Port: see split.
func join(lo, hi *wide.U16x16) wide.F32x16 {
	// We are joining two u16x16 (256 bit) into f32x16 (512bit).
	var v wide.F32x16
	for i := range 8 {
		v[0][i] = math.Float32frombits(uint32(lo[2*i]) | uint32(lo[2*i+1])<<16)
	}
	for i := range 8 {
		v[1][i] = math.Float32frombits(uint32(hi[2*i]) | uint32(hi[2*i+1])<<16)
	}
	return v
}

// madLowp is mad in tiny-skia src/pipeline/lowp.rs.
//
// Port (D33): it works on one lane, because only loops on the lanes call it.
// The conversion rounds the product, so that the Go compiler cannot fuse it
// with the add (D16).
func madLowp(f, m, a float32) float32 {
	// NEON vmlaq_f32 does not seem to affect performance in any way. Ignore it.
	return float32(f*m) + a
}

// joinLane is join on one lane (D33). It gives the float with the low half
// lo and the high half hi of its bits.
func joinLane(lo, hi uint16) float32 {
	return math.Float32frombits(uint32(lo) | uint32(hi)<<16)
}
