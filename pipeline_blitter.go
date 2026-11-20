// Copyright 2016 Google Inc.
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "encoding/binary"

// rasterPipelineBlitter is RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
//
// Port: mask is nil for the Rust None. memset2dColor is valid only when
// hasMemset2dColor is true. The blitter draws only on the rows from rowTop
// to rowBottom, without rowBottom: the rows that the pixmap and the mask
// hold. The other rows of a mask are 0, and the stage mask_u8 stores
// nothing there, so the bytes are the same as upstream. It also draws only
// on the columns from colLeft to colRight, without colRight: the columns
// that the pixmap holds. The columns of a mask that a pixmap draws through
// do not cut a rect here, because maskBounds keeps the chunks of the stages
// in their places (D35).
type rasterPipelineBlitter struct {
	rowTop, rowBottom uint32
	colLeft, colRight uint32

	mask             *subMaskRef
	pixmapSrc        PixmapRef
	pixmap           *subPixmapMut
	memset2dColor    PremultipliedColorU8
	hasMemset2dColor bool
	blitAntiHRp      *rasterPipeline
	blitRectRp       *rasterPipeline
	blitMaskRp       *rasterPipeline
	isMask           bool

	// Port: hline holds the runs of callHLineBlitter, and rectClip the
	// blitter of newRectClipBlitter, which upstream makes on the stack
	// (D35).
	hline    hlineRuns
	rectClip rectClipBlitter
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

	rowTop, rowBottom := pixmap.top, pixmap.bottom
	if mask != nil {
		rowTop, rowBottom = max(rowTop, mask.top), min(rowBottom, mask.bottom)
	}

	return &rasterPipelineBlitter{
		rowTop:           uint32(max(rowTop, 0)),
		rowBottom:        uint32(max(rowBottom, 0)),
		colLeft:          uint32(max(pixmap.left, 0)),
		colRight:         uint32(max(pixmap.right, 0)),
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
		rowTop:           uint32(max(pixmap.top, 0)),
		rowBottom:        uint32(max(pixmap.bottom, 0)),
		colLeft:          uint32(max(pixmap.left, 0)),
		colRight:         uint32(max(pixmap.right, 0)),
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
	if y < b.rowTop || y >= b.rowBottom {
		return
	}
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
			rect := screenIntRectFromXYWHSafe(x, y, width, lengthU32One)
			// Port: the run leaves out the columns that the pixmap does not
			// hold, and the chunks where the mask is 0, as blitRect does
			// (D35, D60).
			var ok bool
			if x < b.colLeft || rect.Right() > b.colRight {
				if rect, ok = b.clipRect(&rect); !ok {
					break
				}
			}
			if b.mask != nil {
				if rect, ok = b.maskBounds(&rect, maskCtx); !ok {
					break
				}
			}

			b.blitAntiHRp.ctx.currentCoverage = float32(alpha) * (1.0 / 255.0)

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
	if b.outside(rect) {
		r, ok := b.clipRect(rect)
		if !ok {
			return
		}
		rect = &r
	}

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

	if b.mask != nil {
		r, ok := b.maskBounds(rect, maskCtx)
		if !ok {
			return
		}
		rect = &r
	}

	b.blitRectRp.run(
		rect,
		aaMaskCtx{},
		maskCtx,
		b.pixmapSrc,
		b.pixmap,
	)
}

// outside reports whether a part of rect is outside the rows from rowTop to
// rowBottom or the columns from colLeft to colRight. It ports no upstream
// function (D35).
func (b *rasterPipelineBlitter) outside(rect *screenIntRect) bool {
	return rect.Y() < b.rowTop || rect.Bottom() > b.rowBottom ||
		rect.X() < b.colLeft || rect.Right() > b.colRight
}

// clipRect returns the part of rect on the rows from rowTop to rowBottom and
// the columns from colLeft to colRight. It returns false when no pixel of
// rect is there. It ports no upstream function (D35).
func (b *rasterPipelineBlitter) clipRect(rect *screenIntRect) (screenIntRect, bool) {
	top, bottom := max(rect.Y(), b.rowTop), min(rect.Bottom(), b.rowBottom)
	left, right := max(rect.X(), b.colLeft), min(rect.Right(), b.colRight)
	if top >= bottom || left >= right {
		return screenIntRect{}, false
	}
	return screenIntRectFromXYWHSafe(left, top, lengthU32(right-left), lengthU32(bottom-top)), true
}

// maskBounds returns maskBounds of rect with the mask of the blitter. It
// ports no upstream function (D60).
func (b *rasterPipelineBlitter) maskBounds(rect *screenIntRect, ctx maskCtx) (screenIntRect, bool) {
	return maskBounds(rect, ctx, b.mask.left, b.mask.right)
}

// maskChunk is the number of pixels that maskBounds keeps together: a
// multiple of the stage widths of lowp and highp.
const maskChunk = 16

// maskBounds returns the part of rect that the pipeline must run on for the
// mask of ctx, which holds the columns from left to right, without right.
// It returns false when the mask is zero on the whole rect.
//
// Port: upstream runs the pipeline on the whole rect. The stage mask_u8
// returns without the next stages for a chunk of pixels whose coverage is
// zero, so such a chunk leaves the pixels as they were. maskBounds leaves
// out the rows of rect whose coverage is zero, and on the left and on the
// right the chunks whose coverage is zero. The chunks keep their places,
// so the pixels are the same bytes as upstream. An image that a small clip
// cuts, or a shading that fills a clip, then runs on the clip and not on
// the whole pixmap (D60). maskBounds reads only the columns of rect that the
// mask holds. A mask holds maskChunk-1 columns of 0 on each side of its
// pixels that are not 0, or ends at the side of the pixmap, so the chunks
// that maskBounds keeps are inside the columns that the mask holds (D35).
func maskBounds(rect *screenIntRect, ctx maskCtx, left, right int) (screenIntRect, bool) {
	x, w := int(rect.X()), int(rect.Width())
	// The columns of rect that the mask holds start at x+skip and hold n
	// pixels.
	skip := max(left-x, 0)
	n := min(right, x+w) - x - skip
	if n <= 0 {
		return screenIntRect{}, false
	}
	row := func(y uint32) []byte {
		start := ctx.offset(x+skip, int(y))
		return ctx.data[start : start+n]
	}
	top, bottom := rect.Y(), rect.Bottom()
	for top < bottom && allZero(row(top)) {
		top++
	}
	if top == bottom {
		return screenIntRect{}, false
	}
	for allZero(row(bottom - 1)) {
		bottom--
	}
	// first and last are the first and the last pixel with a coverage, from
	// x+skip.
	first, last := n, -1
	for y := top; y < bottom; y++ {
		r := row(y)
		for i := 0; i < first; i++ {
			if r[i] != 0 {
				first = i
				break
			}
		}
		for i := n - 1; i > last; i-- {
			if r[i] != 0 {
				last = i
				break
			}
		}
	}
	// The chunks start at x, as in upstream.
	start := (skip + first) / maskChunk * maskChunk
	end := min(w, ((skip+last)/maskChunk+1)*maskChunk)
	return screenIntRectFromXYWHSafe(uint32(x+start), top, lengthU32(end-start), lengthU32(bottom-top)), true
}

// allZero reports whether each byte of b is zero.
func allZero(b []byte) bool {
	for len(b) >= 8 {
		if binary.NativeEndian.Uint64(b) != 0 {
			return false
		}
		b = b[8:]
	}
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// blitMask is Blitter::blit_mask of RasterPipelineBlitter in tiny-skia
// src/pipeline/blitter.rs.
func (b *rasterPipelineBlitter) blitMask(mask *maskBlitter, clip *screenIntRect) {
	if b.outside(clip) {
		r, ok := b.clipRect(clip)
		if !ok {
			return
		}
		clip = &r
	}

	aaMaskCtx := aaMaskCtx{
		pixels: mask.image,
		stride: mask.rowBytes,
		shift:  uint(mask.bounds.Left() + mask.bounds.Top()*mask.rowBytes),
	}

	maskCtx := b.maskCtx()

	// Port: the rect leaves out the rows where the mask is 0, as blitRect
	// does (D60).
	if b.mask != nil {
		r, ok := b.maskBounds(clip, maskCtx)
		if !ok {
			return
		}
		clip = &r
	}

	b.blitMaskRp.run(clip, aaMaskCtx, maskCtx, b.pixmapSrc, b.pixmap)
}
