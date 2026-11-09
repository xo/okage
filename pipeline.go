// Copyright 2016 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"log/slog"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file is tiny-skia src/pipeline/mod.rs, a raster pipeline
// implementation.
//
// Despite having a lot of changes compared to SkRasterPipeline, the core
// principles are the same:
//
//  1. A pipeline consists of stages.
//  2. A pipeline has a global context shared by all stages. Unlike Skia,
//     where each stage has its own, possibly shared, context.
//  3. Each stage has a high precision implementation. See pipeline_highp.go.
//  4. Some stages have a low precision implementation. See pipeline_lowp.go.
//  5. Each stage calls the "next" stage after it is done.
//  6. During pipeline "compilation", if all stages have a lowp
//     implementation, the lowp pipeline is used. Otherwise, the highp variant
//     is used.
//  7. The pipeline "compilation" produces a list of functions. The last one
//     is the "return" function, which simply stops the execution of the
//     pipeline.
//
// This implementation is a bit tricky, but it gives the maximum performance.
// A simple and straightforward implementation with traits and loops is at
// least 20-30% slower in Rust. Not really sure why.
//
// Also, since this module is all about performance, any kind of branching is
// strictly forbidden. All stage functions must not use if, match or loops.
// There are still some exceptions, which are basically imperfect
// implementations, and a later change can optimize them out.

// maxStages is MAX_STAGES in tiny-skia src/pipeline/mod.rs.
const maxStages = 32 // More than enough.

// stage is Stage in tiny-skia src/pipeline/mod.rs. It names a stage of the
// raster pipeline.
type stage uint8

// The values of stage, in the order of the Rust enum.
const (
	stageMoveSourceToDestination stage = iota
	stageMoveDestinationToSource
	stageClamp0
	stageClampA
	stagePremultiply
	stageUniformColor
	stageSeedShader
	stageLoadDestination
	stageStore
	stageLoadDestinationU8
	stageStoreU8
	stageGather
	stageLoadMaskU8
	stageMaskU8
	stageScaleU8
	stageLerpU8
	stageScale1Float
	stageLerp1Float
	stageDestinationAtop
	stageDestinationIn
	stageDestinationOut
	stageDestinationOver
	stageSourceAtop
	stageSourceIn
	stageSourceOut
	stageSourceOver
	stageClear
	stageModulate
	stageMultiply
	stagePlus
	stageScreen
	stageXor
	stageColorBurn
	stageColorDodge
	stageDarken
	stageDifference
	stageExclusion
	stageHardLight
	stageLighten
	stageOverlay
	stageSoftLight
	stageHue
	stageSaturation
	stageColor
	stageLuminosity
	stageSourceOverRgba
	stageTransform
	stageReflect
	stageRepeat
	stageBilinear
	stageBicubic
	stagePadX1
	stageReflectX1
	stageRepeatX1
	stageGradient
	stageEvenlySpaced2StopGradient
	stageXYToUnitAngle
	stageXYToRadius
	stageXYTo2PtConicalFocalOnCircle
	stageXYTo2PtConicalWellBehaved
	stageXYTo2PtConicalSmaller
	stageXYTo2PtConicalGreater
	stageXYTo2PtConicalStrip
	stageMask2PtConicalNan
	stageMask2PtConicalDegenerates
	stageApplyVectorMask
	stageAlter2PtConicalCompensateFocal
	stageAlter2PtConicalUnswap
	stageNegateX
	stageApplyConcentricScaleBias
	stageGammaExpand2
	stageGammaExpandDestination2
	stageGammaCompress2
	stageGammaExpand22
	stageGammaExpandDestination22
	stageGammaCompress22
	stageGammaExpandSrgb
	stageGammaExpandDestinationSrgb
	stageGammaCompressSrgb
)

// stagesCount is STAGES_COUNT in tiny-skia src/pipeline/mod.rs.
const stagesCount = int(stageGammaCompressSrgb) + 1

// gather is PixmapRef::gather in tiny-skia src/pipeline/mod.rs. It returns
// the pixels at the eight indexes of index, as RGBA bytes.
//
// Port: upstream returns an array of eight PremultipliedColorU8. Go returns
// their bytes, which have the same layout. It panics when an index is out of
// range, as the index of a Rust slice does.
func (r PixmapRef) gather(index wide.U32x8) [highpStageWidth * BytesPerPixel]byte {
	pixels := r.Pixels()
	var out [highpStageWidth * BytesPerPixel]byte
	for i := range highpStageWidth {
		j := int(index[i]) * BytesPerPixel
		copy(out[i*BytesPerPixel:i*BytesPerPixel+BytesPerPixel], pixels[j:j+BytesPerPixel])
	}
	return out
}

// offset is SubPixmapMut::offset in tiny-skia src/pipeline/mod.rs.
func (s *subPixmapMut) offset(dx, dy int) int {
	return s.realWidth*dy + dx
}

// sliceAtXY is SubPixmapMut::slice_at_xy in tiny-skia src/pipeline/mod.rs.
func (s *subPixmapMut) sliceAtXY(dx, dy int) PixelSlice {
	offset := s.offset(dx, dy)
	return s.pixelsMut()[offset*BytesPerPixel:]
}

// sliceMaskAtXY is SubPixmapMut::slice_mask_at_xy in tiny-skia
// src/pipeline/mod.rs.
func (s *subPixmapMut) sliceMaskAtXY(dx, dy int) []byte {
	offset := s.offset(dx, dy)
	return s.data[offset:]
}

// slice4AtXY is SubPixmapMut::slice4_at_xy in tiny-skia src/pipeline/mod.rs.
//
// Port: upstream returns an array of highp::STAGE_WIDTH pixels. Go returns a
// pointer to their bytes.
func (s *subPixmapMut) slice4AtXY(dx, dy int) *[highpStageWidth * BytesPerPixel]byte {
	offset := s.offset(dx, dy) * BytesPerPixel
	return (*[highpStageWidth * BytesPerPixel]byte)(s.pixelsMut()[offset:])
}

// slice16AtXY is SubPixmapMut::slice16_at_xy in tiny-skia
// src/pipeline/mod.rs.
//
// Port: upstream returns an array of lowp::STAGE_WIDTH pixels. Go returns a
// pointer to their bytes.
func (s *subPixmapMut) slice16AtXY(dx, dy int) *[lowpStageWidth * BytesPerPixel]byte {
	offset := s.offset(dx, dy) * BytesPerPixel
	return (*[lowpStageWidth * BytesPerPixel]byte)(s.pixelsMut()[offset:])
}

// slice16MaskAtXY is SubPixmapMut::slice16_mask_at_xy in tiny-skia
// src/pipeline/mod.rs.
func (s *subPixmapMut) slice16MaskAtXY(dx, dy int) *[lowpStageWidth]byte {
	return (*[lowpStageWidth]byte)(s.data[s.offset(dx, dy):])
}

// aaMaskCtx is AAMaskCtx in tiny-skia src/pipeline/mod.rs.
type aaMaskCtx struct {
	pixels [2]uint8
	stride uint32 // can be zero
	shift  uint   // mask offset/position in pixmap coordinates
}

// copyAtXY is AAMaskCtx::copy_at_xy in tiny-skia src/pipeline/mod.rs.
func (c *aaMaskCtx) copyAtXY(dx, dy, tail int) [2]uint8 {
	offset := (uint(c.stride)*uint(dy) + uint(dx)) - c.shift
	// We have only 3 variants, so unroll them.
	switch {
	case offset == 0 && tail == 1:
		return [2]uint8{c.pixels[0], 0}
	case offset == 0 && tail == 2:
		return [2]uint8{c.pixels[0], c.pixels[1]}
	case offset == 1 && tail == 1:
		return [2]uint8{c.pixels[1], 0}
	}
	return [2]uint8{0, 0} // unreachable
}

// maskCtx is MaskCtx in tiny-skia src/pipeline/mod.rs.
type maskCtx struct {
	data      []byte
	realWidth uint32
}

// offset is MaskCtx::offset in tiny-skia src/pipeline/mod.rs.
func (c *maskCtx) offset(dx, dy int) int {
	return int(c.realWidth)*dy + dx
}

// context is Context in tiny-skia src/pipeline/mod.rs. It is the global
// context that all the stages of a pipeline share.
type context struct {
	currentCoverage           float32
	sampler                   samplerCtx
	uniformColor              uniformColorCtx
	evenlySpaced2StopGradient evenlySpaced2StopGradientCtx
	gradient                  gradientCtx
	twoPointConicalGradient   twoPointConicalGradientCtx
	limitX                    tileCtx
	limitY                    tileCtx
	transform                 Transform
}

// newContext is the Default impl of Context in tiny-skia
// src/pipeline/mod.rs.
//
// Port: the default Transform is the identity, which is not the zero value of
// a Go Transform.
func newContext() context {
	return context{transform: path.DefaultTransform()}
}

// samplerCtx is SamplerCtx in tiny-skia src/pipeline/mod.rs.
type samplerCtx struct {
	spreadMode SpreadMode
	invWidth   float32
	invHeight  float32
}

// uniformColorCtx is UniformColorCtx in tiny-skia src/pipeline/mod.rs.
type uniformColorCtx struct {
	r    float32
	g    float32
	b    float32
	a    float32
	rgba [4]uint16 // [0,255] in a 16-bit lane.
}

// gradientColor is GradientColor in tiny-skia src/pipeline/mod.rs.
//
// A gradient color is an unpremultiplied RGBA not in a 0..1 range. It
// basically can have any float value.
type gradientColor struct {
	r float32
	g float32
	b float32
	a float32
}

// newGradientColor is GradientColor::new in tiny-skia src/pipeline/mod.rs.
func newGradientColor(r, g, b, a float32) gradientColor {
	return gradientColor{r: r, g: g, b: b, a: a}
}

// gradientColorFromColor is the From<Color> impl of GradientColor in
// tiny-skia src/pipeline/mod.rs.
func gradientColorFromColor(c Color) gradientColor {
	return gradientColor{
		r: c.Red(),
		g: c.Green(),
		b: c.Blue(),
		a: c.Alpha(),
	}
}

// evenlySpaced2StopGradientCtx is EvenlySpaced2StopGradientCtx in tiny-skia
// src/pipeline/mod.rs.
type evenlySpaced2StopGradientCtx struct {
	factor gradientColor
	bias   gradientColor
}

// gradientCtx is GradientCtx in tiny-skia src/pipeline/mod.rs.
type gradientCtx struct {
	// len stores the actual colors count. factors and biases must store at
	// least 16 values, since this is the length of a lowp pipeline stage. So
	// any value past len is just zeros.
	len     int
	factors []gradientColor
	biases  []gradientColor
	tValues []path.NormalizedF32
}

// pushConstColor is GradientCtx::push_const_color in tiny-skia
// src/pipeline/mod.rs.
func (c *gradientCtx) pushConstColor(color gradientColor) {
	c.factors = append(c.factors, newGradientColor(0.0, 0.0, 0.0, 0.0))
	c.biases = append(c.biases, color)
}

// twoPointConicalGradientCtx is TwoPointConicalGradientCtx in tiny-skia
// src/pipeline/mod.rs.
type twoPointConicalGradientCtx struct {
	// This context is used only in highp, where we use Tx4.
	mask wide.U32x8
	p0   float32
	p1   float32
}

// tileCtx is TileCtx in tiny-skia src/pipeline/mod.rs.
type tileCtx struct {
	scale    float32
	invScale float32 // cache of 1/scale
}

// rasterPipelineBuilder is RasterPipelineBuilder in tiny-skia
// src/pipeline/mod.rs.
//
// Port: upstream holds the stages in an ArrayVec. Go holds them in an array
// with a length. logger is the logger of D20, which the shaders log to. The
// painter sets it. A nil logger logs nothing.
type rasterPipelineBuilder struct {
	stages          [maxStages]stage
	stagesLen       int
	forceHqPipeline bool
	ctx             context
	logger          *slog.Logger
}

// newRasterPipelineBuilder is RasterPipelineBuilder::new in tiny-skia
// src/pipeline/mod.rs.
func newRasterPipelineBuilder() *rasterPipelineBuilder {
	return &rasterPipelineBuilder{
		forceHqPipeline: false,
		ctx:             newContext(),
	}
}

// setForceHqPipeline is RasterPipelineBuilder::set_force_hq_pipeline in
// tiny-skia src/pipeline/mod.rs.
func (b *rasterPipelineBuilder) setForceHqPipeline(hq bool) {
	b.forceHqPipeline = hq
}

// push is RasterPipelineBuilder::push in tiny-skia src/pipeline/mod.rs.
//
// It panics when the builder already holds maxStages stages, as the push of
// an ArrayVec does.
func (b *rasterPipelineBuilder) push(s stage) {
	if b.stagesLen == maxStages {
		panic("ArrayVec: capacity exceeded in push")
	}
	b.stages[b.stagesLen] = s
	b.stagesLen++
}

// pushTransform is RasterPipelineBuilder::push_transform in tiny-skia
// src/pipeline/mod.rs.
func (b *rasterPipelineBuilder) pushTransform(ts Transform) {
	if ts.IsFinite() && !ts.IsIdentity() {
		b.push(stageTransform)
		b.ctx.transform = ts
	}
}

// pushUniformColor is RasterPipelineBuilder::push_uniform_color in tiny-skia
// src/pipeline/mod.rs.
func (b *rasterPipelineBuilder) pushUniformColor(c PremultipliedColor) {
	r := c.Red()
	g := c.Green()
	bl := c.Blue()
	a := c.Alpha()
	rgba := [4]uint16{
		wide.TruncU16(float32(r*255.0) + 0.5),
		wide.TruncU16(float32(g*255.0) + 0.5),
		wide.TruncU16(float32(bl*255.0) + 0.5),
		wide.TruncU16(float32(a*255.0) + 0.5),
	}

	ctx := uniformColorCtx{r: r, g: g, b: bl, a: a, rgba: rgba}

	b.push(stageUniformColor)
	b.ctx.uniformColor = ctx
}

// compile is RasterPipelineBuilder::compile in tiny-skia
// src/pipeline/mod.rs.
//
// Port: upstream compares function pointers. It finds a stage without a lowp
// implementation by its pointer to null_fn, and it swaps the load and store
// functions for their tail variants by their pointers. Go cannot compare
// functions, so it reads the stage itself. A stage without a lowp
// implementation has a nil entry in lowpStages.
func (b *rasterPipelineBuilder) compile() *rasterPipeline {
	stages := b.stages[:b.stagesLen]
	if len(stages) == 0 {
		return &rasterPipeline{
			kind: rasterPipelineKindHigh,
			ctx:  newContext(),
		}
	}

	isLowpCompatible := true
	for _, s := range stages {
		if lowpStages[s] == nil {
			isLowpCompatible = false
			break
		}
	}

	if b.forceHqPipeline || !isLowpCompatible {
		functions := make([]func(*highpPipeline), 0, len(stages)+1)
		for _, s := range stages {
			functions = append(functions, highpStages[s])
		}
		functions = append(functions, (*highpPipeline).justReturn)

		// I was not able to reproduce Skia's load_8888_/store_8888_ performance.
		// Skia uses fallthrough switch, which is probably the reason.
		// In Rust, any branching in load/store code drastically affects the performance.
		// So instead, we are using two "programs": one for "full stages" and one for "tail stages".
		// While the only difference is the load/store methods.
		tailFunctions := make([]func(*highpPipeline), len(functions))
		copy(tailFunctions, functions)
		for i, s := range stages {
			switch s {
			case stageLoadDestination:
				tailFunctions[i] = (*highpPipeline).loadDstTail
			case stageStore:
				tailFunctions[i] = (*highpPipeline).storeTail
			case stageLoadDestinationU8:
				tailFunctions[i] = (*highpPipeline).loadDstU8Tail
			case stageStoreU8:
				tailFunctions[i] = (*highpPipeline).storeU8Tail
			case stageSourceOverRgba:
				// SourceOverRgba calls load/store manually, without the pipeline,
				// therefore we have to switch it too.
				tailFunctions[i] = (*highpPipeline).sourceOverRgbaTail
			}
		}

		return &rasterPipeline{
			kind:              rasterPipelineKindHigh,
			highFunctions:     functions,
			highTailFunctions: tailFunctions,
			ctx:               b.ctx,
		}
	}

	functions := make([]func(*lowpPipeline), 0, len(stages)+1)
	for _, s := range stages {
		functions = append(functions, lowpStages[s])
	}
	functions = append(functions, (*lowpPipeline).justReturn)

	// See above.
	tailFunctions := make([]func(*lowpPipeline), len(functions))
	copy(tailFunctions, functions)
	for i, s := range stages {
		switch s {
		case stageLoadDestination:
			tailFunctions[i] = (*lowpPipeline).loadDstTail
		case stageStore:
			tailFunctions[i] = (*lowpPipeline).storeTail
		case stageLoadDestinationU8:
			tailFunctions[i] = (*lowpPipeline).loadDstU8Tail
		case stageStoreU8:
			tailFunctions[i] = (*lowpPipeline).storeU8Tail
		case stageSourceOverRgba:
			// SourceOverRgba calls load/store manually, without the pipeline,
			// therefore we have to switch it too.
			tailFunctions[i] = (*lowpPipeline).sourceOverRgbaTail
		}
	}

	return &rasterPipeline{
		kind:             rasterPipelineKindLow,
		lowFunctions:     functions,
		lowTailFunctions: tailFunctions,
		ctx:              b.ctx,
	}
}

// rasterPipelineKind is RasterPipelineKind in tiny-skia src/pipeline/mod.rs.
//
// Port: the Rust enum holds the functions of each variant. Go holds the
// variant here and the functions in rasterPipeline.
type rasterPipelineKind uint8

// The values of rasterPipelineKind.
const (
	// rasterPipelineKindHigh is RasterPipelineKind::High.
	rasterPipelineKindHigh rasterPipelineKind = iota
	// rasterPipelineKindLow is RasterPipelineKind::Low.
	rasterPipelineKindLow
)

// rasterPipeline is RasterPipeline in tiny-skia src/pipeline/mod.rs.
//
// Port: highp and lowp hold the state of a run. Upstream makes a new Pipeline
// on the stack for each run. In Go, a pipeline that the stages reach through
// a function value moves to the heap, so the rasterPipeline keeps one and
// reuses it, and a run does not allocate (D25).
type rasterPipeline struct {
	kind              rasterPipelineKind
	highFunctions     []func(*highpPipeline)
	highTailFunctions []func(*highpPipeline)
	lowFunctions      []func(*lowpPipeline)
	lowTailFunctions  []func(*lowpPipeline)
	ctx               context

	highp highpPipeline
	lowp  lowpPipeline
}

// run is RasterPipeline::run in tiny-skia src/pipeline/mod.rs.
func (rp *rasterPipeline) run(
	rect *screenIntRect,
	aaMaskCtx aaMaskCtx,
	maskCtx maskCtx,
	pixmapSrc PixmapRef,
	pixmapDst *subPixmapMut,
) {
	switch rp.kind {
	case rasterPipelineKindHigh:
		startHighp(
			&rp.highp,
			rp.highFunctions,
			rp.highTailFunctions,
			rect,
			aaMaskCtx,
			maskCtx,
			&rp.ctx,
			pixmapSrc,
			pixmapDst,
		)
	case rasterPipelineKindLow:
		startLowp(
			&rp.lowp,
			rp.lowFunctions,
			rp.lowTailFunctions,
			rect,
			aaMaskCtx,
			maskCtx,
			&rp.ctx,
			// lowp does not support pattern, so no `pixmap_src` for it.
			pixmapDst,
		)
	}
}
