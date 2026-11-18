// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"math"

	"github.com/xo/okage/path"
)

// FilterQuality is FilterQuality in tiny-skia src/shaders/pattern.rs. It
// controls how much filtering to be done when transforming images.
type FilterQuality uint8

// The values of FilterQuality, in the order of the Rust enum.
const (
	// FilterQualityNearest is FilterQuality::Nearest. It is
	// nearest-neighbor: low quality, but fastest.
	FilterQualityNearest FilterQuality = iota
	// FilterQualityBilinear is FilterQuality::Bilinear.
	FilterQualityBilinear
	// FilterQualityBicubic is FilterQuality::Bicubic: high quality, but
	// slow.
	FilterQualityBicubic
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (q FilterQuality) String() string {
	switch q {
	case FilterQualityNearest:
		return "Nearest"
	case FilterQualityBilinear:
		return "Bilinear"
	case FilterQualityBicubic:
		return "Bicubic"
	}
	return "FilterQuality(?)"
}

// PixmapPaint is PixmapPaint in tiny-skia src/shaders/pattern.rs. It
// controls how a pixmap is blended.
//
// It is like Paint, but for Pixmap.
type PixmapPaint struct {
	// Opacity is the pixmap opacity.
	//
	// Must be in 0..=1 range.
	//
	// Default: 1.0
	Opacity float32

	// BlendMode is the pixmap blending mode.
	//
	// Default: SourceOver
	BlendMode BlendMode

	// Quality specifies how much filtering to be done when transforming
	// images.
	//
	// Default: Nearest
	Quality FilterQuality
}

// DefaultPixmapPaint is the Default impl of PixmapPaint in tiny-skia
// src/shaders/pattern.rs.
func DefaultPixmapPaint() PixmapPaint {
	return PixmapPaint{
		Opacity:   1.0,
		BlendMode: BlendModeSourceOver,
		Quality:   FilterQualityNearest,
	}
}

// Pattern is Pattern in tiny-skia src/shaders/pattern.rs. It is a pattern
// shader.
//
// Essentially a SkImageShader.
//
// Unlike Skia, we do not support FilterQuality::Medium, because it involves
// mipmap generation, which adds too much complexity.
type Pattern struct {
	pixmap     PixmapRef
	quality    FilterQuality
	spreadMode SpreadMode
	opacity    path.NormalizedF32
	transform  Transform
}

// NewPattern is Pattern::new in tiny-skia src/shaders/pattern.rs. It creates
// a new pattern shader.
//
// opacity is clamped to the 0..=1 range.
func NewPattern(
	pixmap PixmapRef,
	spreadMode SpreadMode,
	quality FilterQuality,
	opacity float32,
	transform Transform,
) Shader {
	return Pattern{
		pixmap:     pixmap,
		spreadMode: spreadMode,
		quality:    quality,
		opacity:    path.NormalizedF32Clamped(opacity),
		transform:  transform,
	}
}

// IsOpaque is the arm Shader::Pattern of Shader::is_opaque in tiny-skia
// src/shaders/mod.rs.
func (p Pattern) IsOpaque() bool {
	return false
}

// pushStages is Pattern::push_stages in tiny-skia src/shaders/pattern.rs.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// b, as D20 says.
func (p Pattern) pushStages(cs ColorSpace, b *rasterPipelineBuilder) bool {
	ts, ok := p.transform.Invert()
	if !ok {
		warn(b.logger, "failed to invert a pattern transform. Nothing will be rendered")
		return false
	}

	// Port (D53): a pattern that moves by whole pixels and pads takes one
	// stage, which gives the same colors as the stages of upstream.
	if x, y, ok := alignedOffset(ts); ok && p.spreadMode == SpreadModePad {
		b.ctx.gatherAligned = gatherAlignedCtx{x: x, y: y, transform: ts}
		b.push(stageGatherAligned)
	} else {
		p.pushGather(ts, b)
	}

	// Unlike Skia, we do not support global opacity and only Pattern allows it.
	if p.opacity != path.NormalizedF32One {
		b.ctx.currentCoverage = p.opacity.Get()
		b.push(stageScale1Float)
	}

	if s, ok := cs.expandStage(); ok {
		// TODO: it is better to gamma-expand prior to sampling, but
		// there is not currently an intermediate pipeline stage for that.
		b.push(s)
	}

	return true
}

// alignedOffset returns the offset of a transform that only moves by whole
// pixels, of up to gatherAlignedMax on each axis (D53). It ports no upstream
// function.
func alignedOffset(ts Transform) (int, int, bool) {
	if ts.SX != 1 || ts.SY != 1 || ts.KX != 0 || ts.KY != 0 {
		return 0, 0, false
	}
	const limit float32 = gatherAlignedMax
	if !(ts.TX >= -limit && ts.TX <= limit && ts.TY >= -limit && ts.TY <= limit) ||
		ts.TX != truncF32(ts.TX) || ts.TY != truncF32(ts.TY) {
		return 0, 0, false
	}
	return int(ts.TX), int(ts.TY), true
}

// pushGather is the part of Pattern::push_stages in tiny-skia
// src/shaders/pattern.rs that pushes the stages that read the pixels of the
// pattern. It ports no upstream function. pushStages calls it for each
// pattern that does not take stageGatherAligned (D53).
func (p Pattern) pushGather(ts Transform, b *rasterPipelineBuilder) {
	b.push(stageSeedShader)

	b.pushTransform(ts)

	quality := p.quality

	if ts.IsIdentity() || ts.IsTranslate() {
		quality = FilterQualityNearest
	}

	if quality == FilterQualityBilinear {
		if ts.IsTranslate() {
			if ts.TX == truncF32(ts.TX) && ts.TY == truncF32(ts.TY) {
				// When the matrix is just an integer translate, bilerp == nearest neighbor.
				quality = FilterQualityNearest
			}
		}
	}

	// TODO: minimizing scale via mipmap

	switch quality {
	case FilterQualityNearest:
		b.ctx.limitX = tileCtx{
			scale:    float32(p.pixmap.Width()),
			invScale: 1.0 / float32(p.pixmap.Width()),
		}

		b.ctx.limitY = tileCtx{
			scale:    float32(p.pixmap.Height()),
			invScale: 1.0 / float32(p.pixmap.Height()),
		}

		switch p.spreadMode {
		case SpreadModePad: // The gather() stage clamps for us.
		case SpreadModeRepeat:
			b.push(stageRepeat)
		case SpreadModeReflect:
			b.push(stageReflect)
		}

		b.push(stageGather)
	case FilterQualityBilinear:
		b.ctx.sampler = samplerCtx{
			spreadMode: p.spreadMode,
			invWidth:   1.0 / float32(p.pixmap.Width()),
			invHeight:  1.0 / float32(p.pixmap.Height()),
		}
		b.push(stageBilinear)
	case FilterQualityBicubic:
		b.ctx.sampler = samplerCtx{
			spreadMode: p.spreadMode,
			invWidth:   1.0 / float32(p.pixmap.Width()),
			invHeight:  1.0 / float32(p.pixmap.Height()),
		}
		b.push(stageBicubic)

		// Bicubic filtering naturally produces out of range values on both sides of [0,1].
		b.push(stageClamp0)
		b.push(stageClampA)
	}
}

// truncF32 is f32::trunc. It ports no upstream function.
func truncF32(x float32) float32 {
	return float32(math.Trunc(float64(x)))
}
