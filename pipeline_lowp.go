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
	p.r = div255(p.r.Mul(p.a))
	p.g = div255(p.g.Mul(p.a))
	p.b = div255(p.b.Mul(p.a))

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

	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

	p.nextStage()
}

// scaleU8 is scale_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) scaleU8() {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{
		uint16(data[0]),
		uint16(data[1]),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	}

	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

	p.nextStage()
}

// lerpU8 is lerp_u8 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) lerpU8() {
	// Load u8xTail and cast it to u16x16.
	data := p.aaMaskCtx.copyAtXY(p.dx, p.dy, p.tail)
	c := wide.U16x16{
		uint16(data[0]),
		uint16(data[1]),
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
		0,
	}

	p.r = lerpLowp(p.dr, p.r, c)
	p.g = lerpLowp(p.dg, p.g, c)
	p.b = lerpLowp(p.db, p.b, c)
	p.a = lerpLowp(p.da, p.a, c)

	p.nextStage()
}

// scale1Float is scale_1_float in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) scale1Float() {
	c := fromFloat(p.ctx.currentCoverage)
	p.r = div255(p.r.Mul(c))
	p.g = div255(p.g.Mul(c))
	p.b = div255(p.b.Mul(c))
	p.a = div255(p.a.Mul(c))

	p.nextStage()
}

// lerp1Float is lerp_1_float in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) lerp1Float() {
	c := fromFloat(p.ctx.currentCoverage)
	p.r = lerpLowp(p.dr, p.r, c)
	p.g = lerpLowp(p.dg, p.g, c)
	p.b = lerpLowp(p.db, p.b, c)
	p.a = lerpLowp(p.da, p.a, c)

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
	p.r = div255(p.r.Mul(p.da).Add(p.dr.Mul(invLowp(p.a))))
	p.g = div255(p.g.Mul(p.da).Add(p.dg.Mul(invLowp(p.a))))
	p.b = div255(p.b.Mul(p.da).Add(p.db.Mul(invLowp(p.a))))
	p.a = div255(p.a.Mul(p.da).Add(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// destinationAtop is the stage destination_atop of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationAtop() {
	p.r = div255(p.dr.Mul(p.a).Add(p.r.Mul(invLowp(p.da))))
	p.g = div255(p.dg.Mul(p.a).Add(p.g.Mul(invLowp(p.da))))
	p.b = div255(p.db.Mul(p.a).Add(p.b.Mul(invLowp(p.da))))
	p.a = div255(p.da.Mul(p.a).Add(p.a.Mul(invLowp(p.da))))

	p.nextStage()
}

// sourceIn is the stage source_in of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceIn() {
	p.r = div255(p.r.Mul(p.da))
	p.g = div255(p.g.Mul(p.da))
	p.b = div255(p.b.Mul(p.da))
	p.a = div255(p.a.Mul(p.da))

	p.nextStage()
}

// destinationIn is the stage destination_in of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationIn() {
	p.r = div255(p.dr.Mul(p.a))
	p.g = div255(p.dg.Mul(p.a))
	p.b = div255(p.db.Mul(p.a))
	p.a = div255(p.da.Mul(p.a))

	p.nextStage()
}

// sourceOut is the stage source_out of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOut() {
	p.r = div255(p.r.Mul(invLowp(p.da)))
	p.g = div255(p.g.Mul(invLowp(p.da)))
	p.b = div255(p.b.Mul(invLowp(p.da)))
	p.a = div255(p.a.Mul(invLowp(p.da)))

	p.nextStage()
}

// destinationOut is the stage destination_out of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationOut() {
	p.r = div255(p.dr.Mul(invLowp(p.a)))
	p.g = div255(p.dg.Mul(invLowp(p.a)))
	p.b = div255(p.db.Mul(invLowp(p.a)))
	p.a = div255(p.da.Mul(invLowp(p.a)))

	p.nextStage()
}

// sourceOver is the stage source_over of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOver() {
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// destinationOver is the stage destination_over of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) destinationOver() {
	p.r = p.dr.Add(div255(p.r.Mul(invLowp(p.da))))
	p.g = p.dg.Add(div255(p.g.Mul(invLowp(p.da))))
	p.b = p.db.Add(div255(p.b.Mul(invLowp(p.da))))
	p.a = p.da.Add(div255(p.a.Mul(invLowp(p.da))))

	p.nextStage()
}

// modulate is the stage modulate of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) modulate() {
	p.r = div255(p.r.Mul(p.dr))
	p.g = div255(p.g.Mul(p.dg))
	p.b = div255(p.b.Mul(p.db))
	p.a = div255(p.a.Mul(p.da))

	p.nextStage()
}

// multiply is the stage multiply of blend_fn! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) multiply() {
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.r.Mul(p.dr)))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.g.Mul(p.dg)))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.b.Mul(p.db)))
	p.a = div255(p.a.Mul(invLowp(p.da)).Add(p.da.Mul(invLowp(p.a))).Add(p.a.Mul(p.da)))

	p.nextStage()
}

// screen is the stage screen of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) screen() {
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.dr)))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.dg)))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.db)))
	p.a = p.a.Add(p.da).Sub(div255(p.a.Mul(p.da)))

	p.nextStage()
}

// xor is the stage xor of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) xor() {
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))))
	p.a = div255(p.a.Mul(invLowp(p.da)).Add(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// plus is the stage plus of blend_fn! in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) plus() {
	p.r = p.r.Add(p.dr).Min(wide.SplatU16x16(255))
	p.g = p.g.Add(p.dg).Min(wide.SplatU16x16(255))
	p.b = p.b.Add(p.db).Min(wide.SplatU16x16(255))
	p.a = p.a.Add(p.da).Min(wide.SplatU16x16(255))

	p.nextStage()
}

// The stages below are the blend_fn2! macro of tiny-skia
// src/pipeline/lowp.rs. Each one applies its function to each color
// channel, and source_over to the alpha.

// darken is the stage darken of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) darken() {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.da).Max(p.dr.Mul(p.a))))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.da).Max(p.dg.Mul(p.a))))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.da).Max(p.db.Mul(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// lighten is the stage lighten of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) lighten() {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(div255(p.r.Mul(p.da).Min(p.dr.Mul(p.a))))
	p.g = p.g.Add(p.dg).Sub(div255(p.g.Mul(p.da).Min(p.dg.Mul(p.a))))
	p.b = p.b.Add(p.db).Sub(div255(p.b.Mul(p.da).Min(p.db.Mul(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// exclusion is the stage exclusion of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) exclusion() {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(wide.SplatU16x16(2).Mul(div255(p.r.Mul(p.dr))))
	p.g = p.g.Add(p.dg).Sub(wide.SplatU16x16(2).Mul(div255(p.g.Mul(p.dg))))
	p.b = p.b.Add(p.db).Sub(wide.SplatU16x16(2).Mul(div255(p.b.Mul(p.db))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// difference is the stage difference of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) difference() {
	// The same logic applied to color, and source_over for alpha.
	p.r = p.r.Add(p.dr).Sub(wide.SplatU16x16(2).Mul(div255(p.r.Mul(p.da).Min(p.dr.Mul(p.a)))))
	p.g = p.g.Add(p.dg).Sub(wide.SplatU16x16(2).Mul(div255(p.g.Mul(p.da).Min(p.dg.Mul(p.a)))))
	p.b = p.b.Add(p.db).Sub(wide.SplatU16x16(2).Mul(div255(p.b.Mul(p.da).Min(p.db.Mul(p.a)))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// hardLight is the stage hard_light of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) hardLight() {
	// The same logic applied to color, and source_over for alpha.
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.r.Add(p.r).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.r).Mul(p.dr), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.r)).Mul(p.da.Sub(p.dr))))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.g.Add(p.g).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.g).Mul(p.dg), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.g)).Mul(p.da.Sub(p.dg))))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.b.Add(p.b).CmpLe(p.a).Blend(wide.SplatU16x16(2).Mul(p.b).Mul(p.db), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.b)).Mul(p.da.Sub(p.db))))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// overlay is the stage overlay of blend_fn2! in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) overlay() {
	// The same logic applied to color, and source_over for alpha.
	p.r = div255(p.r.Mul(invLowp(p.da)).Add(p.dr.Mul(invLowp(p.a))).Add(p.dr.Add(p.dr).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.r).Mul(p.dr), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.r)).Mul(p.da.Sub(p.dr))))))
	p.g = div255(p.g.Mul(invLowp(p.da)).Add(p.dg.Mul(invLowp(p.a))).Add(p.dg.Add(p.dg).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.g).Mul(p.dg), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.g)).Mul(p.da.Sub(p.dg))))))
	p.b = div255(p.b.Mul(invLowp(p.da)).Add(p.db.Mul(invLowp(p.a))).Add(p.db.Add(p.db).CmpLe(p.da).Blend(wide.SplatU16x16(2).Mul(p.b).Mul(p.db), p.a.Mul(p.da).Sub(wide.SplatU16x16(2).Mul(p.a.Sub(p.b)).Mul(p.da.Sub(p.db))))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))

	p.nextStage()
}

// sourceOverRgba is source_over_rgba in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOverRgba() {
	pixels := p.pixmap.slice16AtXY(p.dx, p.dy)
	load8888Lowp(pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))
	store8888Lowp(&p.r, &p.g, &p.b, &p.a, pixels)

	p.nextStage()
}

// sourceOverRgbaTail is source_over_rgba_tail in tiny-skia
// src/pipeline/lowp.rs.
func (p *lowpPipeline) sourceOverRgbaTail() {
	pixels := p.pixmap.sliceAtXY(p.dx, p.dy)
	load8888TailLowp(p.tail, pixels, &p.dr, &p.dg, &p.db, &p.da)
	p.r = p.r.Add(div255(p.dr.Mul(invLowp(p.a))))
	p.g = p.g.Add(div255(p.dg.Mul(invLowp(p.a))))
	p.b = p.b.Add(div255(p.db.Mul(invLowp(p.a))))
	p.a = p.a.Add(div255(p.da.Mul(invLowp(p.a))))
	store8888TailLowp(&p.r, &p.g, &p.b, &p.a, p.tail, pixels)

	p.nextStage()
}

// transform is transform in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) transform() {
	ts := &p.ctx.transform

	x := join(&p.r, &p.g)
	y := join(&p.b, &p.a)

	nx := madLowp(x, wide.SplatF32x16(ts.SX), madLowp(y, wide.SplatF32x16(ts.KX), wide.SplatF32x16(ts.TX)))
	ny := madLowp(x, wide.SplatF32x16(ts.KY), madLowp(y, wide.SplatF32x16(ts.SY), wide.SplatF32x16(ts.TY)))

	split(&nx, &p.r, &p.g)
	split(&ny, &p.b, &p.a)

	p.nextStage()
}

// padX1 is pad_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) padX1() {
	x := join(&p.r, &p.g)
	x = x.Normalize()
	split(&x, &p.r, &p.g)

	p.nextStage()
}

// reflectX1 is reflect_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) reflectX1() {
	x := join(&p.r, &p.g)
	one := wide.SplatF32x16(1.0)
	t := x.Sub(one).Mul(wide.SplatF32x16(0.5)).Floor()
	x = x.Sub(one).
		Sub(t.Add(t)). // two(t)
		Sub(one).
		Abs().Normalize()
	split(&x, &p.r, &p.g)

	p.nextStage()
}

// repeatX1 is repeat_x1 in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) repeatX1() {
	x := join(&p.r, &p.g)
	x = x.Sub(x.Floor()).Normalize()
	split(&x, &p.r, &p.g)

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

	t := join(&p.r, &p.g)
	roundF32ToU16(
		madLowp(t, wide.SplatF32x16(ctx.factor.r), wide.SplatF32x16(ctx.bias.r)),
		madLowp(t, wide.SplatF32x16(ctx.factor.g), wide.SplatF32x16(ctx.bias.g)),
		madLowp(t, wide.SplatF32x16(ctx.factor.b), wide.SplatF32x16(ctx.bias.b)),
		madLowp(t, wide.SplatF32x16(ctx.factor.a), wide.SplatF32x16(ctx.bias.a)),
		&p.r, &p.g, &p.b, &p.a,
	)

	p.nextStage()
}

// xyToRadius is xy_to_radius in tiny-skia src/pipeline/lowp.rs.
func (p *lowpPipeline) xyToRadius() {
	x := join(&p.r, &p.g)
	y := join(&p.b, &p.a)
	x = x.Mul(x).Add(y.Mul(y)).Sqrt()
	split(&x, &p.r, &p.g)
	split(&y, &p.b, &p.a)

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
	var fr, fg, fb, fa, br, bg, bb, ba wide.F32x16
	for i := range lowpStageWidth {
		f := &ctx.factors[idx[i]]
		fr[i/8][i%8] = f.r
		fg[i/8][i%8] = f.g
		fb[i/8][i%8] = f.b
		fa[i/8][i%8] = f.a
	}
	for i := range lowpStageWidth {
		c := &ctx.biases[idx[i]]
		br[i/8][i%8] = c.r
		bg[i/8][i%8] = c.g
		bb[i/8][i%8] = c.b
		ba[i/8][i%8] = c.a
	}

	roundF32ToU16(
		madLowp(t, fr, br),
		madLowp(t, fg, bg),
		madLowp(t, fb, bb),
		madLowp(t, fa, ba),
		r, g, b, a,
	)
}

// roundF32ToU16 is round_f32_to_u16 in tiny-skia src/pipeline/lowp.rs.
func roundF32ToU16(
	rf, gf, bf, af wide.F32x16,
	r, g, b, a *wide.U16x16,
) {
	// TODO: can produce a slightly different result to Skia
	//       affects the two_stops_linear_mirror test

	c255 := wide.SplatF32x16(255.0)
	half := wide.SplatF32x16(0.5)
	rf = rf.Normalize().Mul(c255).Add(half)
	gf = gf.Normalize().Mul(c255).Add(half)
	bf = bf.Normalize().Mul(c255).Add(half)
	af = af.Mul(c255).Add(half)

	rf.SaveToU16x16(r)
	gf.SaveToU16x16(g)
	bf.SaveToU16x16(b)
	af.SaveToU16x16(a)
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
func div255(v wide.U16x16) wide.U16x16 {
	// Skia uses `vrshrq_n_u16(vrsraq_n_u16(v, v, 8), 8)` here when NEON is available,
	// but it does not affect performance much and breaks reproducible result. Ignore it.
	// NOTE: the compiler does not replace the division with a shift.
	return v.Add(wide.SplatU16x16(255)).Shr(wide.SplatU16x16(8)) // / u16x16::splat(256)
}

// invLowp is inv in tiny-skia src/pipeline/lowp.rs.
func invLowp(v wide.U16x16) wide.U16x16 {
	return wide.SplatU16x16(255).Sub(v)
}

// fromFloat is from_float in tiny-skia src/pipeline/lowp.rs.
func fromFloat(f float32) wide.U16x16 {
	return wide.SplatU16x16(wide.TruncU16(float32(f*255.0) + 0.5))
}

// lerpLowp is lerp in tiny-skia src/pipeline/lowp.rs.
func lerpLowp(from, to, t wide.U16x16) wide.U16x16 {
	return div255(from.Mul(invLowp(t)).Add(to.Mul(t)))
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
func madLowp(f, m, a wide.F32x16) wide.F32x16 {
	// NEON vmlaq_f32 does not seem to affect performance in any way. Ignore it.
	return f.Mul(m).Add(a)
}
