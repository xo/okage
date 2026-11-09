// Copyright 2016 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

// rasterPipelineBlitter is RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
//
// Port: mask is nil for the Rust None. memset2dColor is valid only when
// hasMemset2dColor is true.
type rasterPipelineBlitter struct {
	mask             *subMaskRef
	pixmapSrc        PixmapRef
	pixmap           *subPixmapMut
	memset2dColor    PremultipliedColorU8
	hasMemset2dColor bool
	blitAntiHRp      *rasterPipeline
	blitRectRp       *rasterPipeline
	blitMaskRp       *rasterPipeline
	isMask           bool
}

// The blitter interface that rasterPipelineBlitter implements.
var _ blitter = (*rasterPipelineBlitter)(nil)

// dummyPixmapSrc returns PixmapRef::from_bytes(&[0, 0, 0, 0], 1, 1).unwrap(),
// the dummy source pixmap of tiny-skia src/pipeline/blitter.rs and
// src/painter.rs. It ports no upstream function.
func dummyPixmapSrc() PixmapRef {
	r, ok := PixmapRefFromBytes([]byte{0, 0, 0, 0}, 1, 1)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	return r
}

// newRasterPipelineBlitter is RasterPipelineBlitter::new in tiny-skia
// src/pipeline/blitter.rs.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// pixmap, as D20 says, and gives that logger to the pipeline builders, for
// the shaders.
func newRasterPipelineBlitter(paint *Paint, mask *subMaskRef, pixmap *subPixmapMut) (*rasterPipelineBlitter, bool) {
	logger := pixmap.logger

	// Make sure that `mask` has the same size as `pixmap`.
	if mask != nil {
		if mask.size.Width() != pixmap.size.Width() ||
			mask.size.Height() != pixmap.size.Height() {
			warn(logger, "Pixmap and Mask are expected to have the same size")
			return nil, false
		}
	}

	// Fast-reject.
	// This is basically SkInterpretXfermode().
	switch {
	// `Destination` keep the pixmap unchanged. Nothing to do here.
	case paint.BlendMode == BlendModeDestination:
		return nil, false
	case paint.BlendMode == BlendModeDestinationIn && paint.Shader.IsOpaque() && paint.IsSolidColor():
		return nil, false
	}

	// We can strength-reduce SourceOver into Source when opaque.
	blendMode := paint.BlendMode
	if paint.Shader.IsOpaque() && blendMode == BlendModeSourceOver && mask == nil {
		blendMode = BlendModeSource
	}

	// When we are drawing a constant color in Source mode, we can sometimes just memset.
	var memset2dColor PremultipliedColorU8
	hasMemset2dColor := false
	if paint.IsSolidColor() && blendMode == BlendModeSource && mask == nil {
		// Unlike Skia, our shader cannot be constant.
		// Therefore there is no need to run a raster pipeline to get shader's color.
		if color, ok := paint.Shader.(SolidColor); ok {
			memset2dColor = Color(color).Premultiply().ToColorU8()
			hasMemset2dColor = true
		}
	}

	// Clear is just a transparent color memset.
	if blendMode == BlendModeClear && !paint.AntiAlias && mask == nil {
		blendMode = BlendModeSource
		memset2dColor = PremultipliedColorU8Transparent
		hasMemset2dColor = true
	}

	var blitAntiHRp *rasterPipeline
	{ //nolint:dupl // upstream repeats this block for each kind of pipeline
		p := newRasterPipelineBuilder()
		p.logger = logger
		p.setForceHqPipeline(paint.ForceHQPipeline)
		if !paint.Shader.pushStages(paint.Colorspace, p) {
			return nil, false
		}

		if mask != nil {
			p.push(stageMaskU8)
		}

		if blendMode.shouldPreScaleCoverage() {
			p.push(stageScale1Float)
			p.push(stageLoadDestination)
			if s, ok := paint.Colorspace.expandDestStage(); ok {
				p.push(s)
			}
			if blendStage, ok := blendMode.toStage(); ok {
				p.push(blendStage)
			}
		} else {
			p.push(stageLoadDestination)
			if s, ok := paint.Colorspace.expandDestStage(); ok {
				p.push(s)
			}
			if blendStage, ok := blendMode.toStage(); ok {
				p.push(blendStage)
			}

			p.push(stageLerp1Float)
		}

		if s, ok := paint.Colorspace.compressStage(); ok {
			p.push(s)
		}
		p.push(stageStore)

		blitAntiHRp = p.compile()
	}

	var blitRectRp *rasterPipeline
	{
		p := newRasterPipelineBuilder()
		p.logger = logger
		p.setForceHqPipeline(paint.ForceHQPipeline)
		if !paint.Shader.pushStages(paint.Colorspace, p) {
			return nil, false
		}

		if mask != nil {
			p.push(stageMaskU8)
		}

		if blendMode == BlendModeSourceOver && mask == nil {
			if s, ok := paint.Colorspace.compressStage(); ok {
				p.push(s)
			}
			// TODO: ignore when dither_rate is non-zero
			p.push(stageSourceOverRgba)
		} else {
			if blendMode != BlendModeSource {
				p.push(stageLoadDestination)
				if blendStage, ok := blendMode.toStage(); ok {
					if s, ok := paint.Colorspace.expandDestStage(); ok {
						p.push(s)
					}
					p.push(blendStage)
				}
			}

			if s, ok := paint.Colorspace.compressStage(); ok {
				p.push(s)
			}
			p.push(stageStore)
		}

		blitRectRp = p.compile()
	}

	var blitMaskRp *rasterPipeline
	{ //nolint:dupl // upstream repeats this block for each kind of pipeline
		p := newRasterPipelineBuilder()
		p.logger = logger
		p.setForceHqPipeline(paint.ForceHQPipeline)
		if !paint.Shader.pushStages(paint.Colorspace, p) {
			return nil, false
		}

		if mask != nil {
			p.push(stageMaskU8)
		}

		if blendMode.shouldPreScaleCoverage() {
			p.push(stageScaleU8)
			p.push(stageLoadDestination)
			if s, ok := paint.Colorspace.expandDestStage(); ok {
				p.push(s)
			}
			if blendStage, ok := blendMode.toStage(); ok {
				p.push(blendStage)
			}
		} else {
			p.push(stageLoadDestination)
			if s, ok := paint.Colorspace.expandDestStage(); ok {
				p.push(s)
			}
			if blendStage, ok := blendMode.toStage(); ok {
				p.push(blendStage)
			}

			p.push(stageLerpU8)
		}

		if s, ok := paint.Colorspace.compressStage(); ok {
			p.push(s)
		}
		p.push(stageStore)

		blitMaskRp = p.compile()
	}

	var pixmapSrc PixmapRef
	if patt, ok := paint.Shader.(Pattern); ok {
		pixmapSrc = patt.pixmap
	} else {
		// Just a dummy one.
		pixmapSrc = dummyPixmapSrc()
	}

	return &rasterPipelineBlitter{
		mask:             mask,
		pixmapSrc:        pixmapSrc,
		pixmap:           pixmap,
		memset2dColor:    memset2dColor,
		hasMemset2dColor: hasMemset2dColor,
		blitAntiHRp:      blitAntiHRp,
		blitRectRp:       blitRectRp,
		blitMaskRp:       blitMaskRp,
		isMask:           false,
	}, true
}

// newRasterPipelineBlitterMask is RasterPipelineBlitter::new_mask in
// tiny-skia src/pipeline/blitter.rs.
func newRasterPipelineBlitterMask(pixmap *subPixmapMut) (*rasterPipelineBlitter, bool) {
	color := ColorWhite.Premultiply()

	memset2dColor := color.ToColorU8()

	var blitAntiHRp *rasterPipeline
	{
		p := newRasterPipelineBuilder()
		p.pushUniformColor(color)
		p.push(stageLoadDestinationU8)
		p.push(stageLerp1Float)
		p.push(stageStoreU8)
		blitAntiHRp = p.compile()
	}

	var blitRectRp *rasterPipeline
	{
		p := newRasterPipelineBuilder()
		p.pushUniformColor(color)
		p.push(stageStoreU8)
		blitRectRp = p.compile()
	}

	var blitMaskRp *rasterPipeline
	{
		p := newRasterPipelineBuilder()
		p.pushUniformColor(color)
		p.push(stageLoadDestinationU8)
		p.push(stageLerpU8)
		p.push(stageStoreU8)
		blitMaskRp = p.compile()
	}

	return &rasterPipelineBlitter{
		mask:             nil,
		pixmapSrc:        dummyPixmapSrc(),
		pixmap:           pixmap,
		memset2dColor:    memset2dColor,
		hasMemset2dColor: true,
		blitAntiHRp:      blitAntiHRp,
		blitRectRp:       blitRectRp,
		blitMaskRp:       blitMaskRp,
		isMask:           true,
	}, true
}

// maskCtx returns self.mask.map(|c| c.mask_ctx()).unwrap_or_default() of
// tiny-skia src/pipeline/blitter.rs. It ports no upstream function.
func (b *rasterPipelineBlitter) maskCtx() maskCtx {
	if b.mask == nil {
		return maskCtx{}
	}
	return b.mask.maskCtx()
}

// blitH is Blitter::blit_h of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitH(x, y uint32, width lengthU32) {
	r := screenIntRectFromXYWHSafe(x, y, width, lengthU32One)
	b.blitRect(&r)
}

// blitAntiH is Blitter::blit_anti_h of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitAntiH(x, y uint32, aa []alphaU8, runs []alphaRun) {
	maskCtx := b.maskCtx()

	aaOffset := 0
	runOffset := 0
	run := runs[0]
	for run != 0 {
		width := lengthU32(run)

		switch alpha := aa[aaOffset]; alpha {
		case AlphaU8Transparent:
		case AlphaU8Opaque:
			b.blitH(x, y, width)
		default:
			b.blitAntiHRp.ctx.currentCoverage = float32(alpha) * (1.0 / 255.0)

			rect := screenIntRectFromXYWHSafe(x, y, width, lengthU32One)
			b.blitAntiHRp.run(
				&rect,
				aaMaskCtx{},
				maskCtx,
				b.pixmapSrc,
				b.pixmap,
			)
		}

		x += width
		runOffset += int(run)
		aaOffset += int(run)
		run = runs[runOffset]
	}
}

// blitV is Blitter::blit_v of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitV(x, y uint32, height lengthU32, alpha alphaU8) {
	bounds := screenIntRectFromXYWHSafe(x, y, lengthU32One, height)

	mask := maskBlitter{
		image:    [2]uint8{alpha, alpha},
		bounds:   bounds,
		rowBytes: 0, // so we reuse the 1 "row" for all of height
	}

	b.blitMask(&mask, &bounds)
}

// blitAntiH2 is Blitter::blit_anti_h2 of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitAntiH2(x, y uint32, alpha0, alpha1 alphaU8) {
	bounds, ok := screenIntRectFromXYWH(x, y, 2, 1)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}

	mask := maskBlitter{
		image:    [2]uint8{alpha0, alpha1},
		bounds:   bounds,
		rowBytes: 2,
	}

	b.blitMask(&mask, &bounds)
}

// blitAntiV2 is Blitter::blit_anti_v2 of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitAntiV2(x, y uint32, alpha0, alpha1 alphaU8) {
	bounds, ok := screenIntRectFromXYWH(x, y, 1, 2)
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}

	mask := maskBlitter{
		image:    [2]uint8{alpha0, alpha1},
		bounds:   bounds,
		rowBytes: 1,
	}

	b.blitMask(&mask, &bounds)
}

// blitRect is Blitter::blit_rect of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitRect(rect *screenIntRect) {
	if b.hasMemset2dColor {
		c := b.memset2dColor
		if b.isMask {
			for y := range rect.Height() {
				start := b.pixmap.offset(int(rect.X()), int(rect.Y()+y))
				end := start + int(rect.Width())
				data := b.pixmap.data[start:end]
				for i := range data {
					data[i] = c.Alpha()
				}
			}
		} else {
			for y := range rect.Height() {
				start := b.pixmap.offset(int(rect.X()), int(rect.Y()+y))
				end := start + int(rect.Width())
				pixels := b.pixmap.pixelsMut()[start*BytesPerPixel : end*BytesPerPixel]
				for i := range pixels.Len() {
					pixels.Set(i, c)
				}
			}
		}

		return
	}

	maskCtx := b.maskCtx()

	b.blitRectRp.run(
		rect,
		aaMaskCtx{},
		maskCtx,
		b.pixmapSrc,
		b.pixmap,
	)
}

// blitMask is Blitter::blit_mask of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitMask(mask *maskBlitter, clip *screenIntRect) {
	aaMaskCtx := aaMaskCtx{
		pixels: mask.image,
		stride: mask.rowBytes,
		shift:  uint(mask.bounds.Left() + mask.bounds.Top()*mask.rowBytes),
	}

	maskCtx := b.maskCtx()

	b.blitMaskRp.run(clip, aaMaskCtx, maskCtx, b.pixmapSrc, b.pixmap)
}
