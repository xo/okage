// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// FillRule is FillRule in tiny-skia src/painter.rs. It is a path filling
// rule.
type FillRule uint8

const (
	// FillRuleWinding is FillRule::Winding, the default. It specifies that
	// "inside" is computed by a non-zero sum of signed edge crossings.
	FillRuleWinding FillRule = iota
	// FillRuleEvenOdd is FillRule::EvenOdd. It specifies that "inside" is
	// computed by an odd number of edge crossings.
	FillRuleEvenOdd
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (r FillRule) String() string {
	switch r {
	case FillRuleWinding:
		return "Winding"
	case FillRuleEvenOdd:
		return "EvenOdd"
	}
	return "FillRule(?)"
}

// Paint is Paint in tiny-skia src/painter.rs. It controls how a shape is
// painted.
//
// Port: the zero Paint is not the default of upstream. Its Shader is nil, its
// BlendMode is Clear and AntiAlias is false. Use DefaultPaint to get the
// default of upstream. A nil Shader is not valid, and drawing with it panics.
type Paint struct {
	// Shader is the paint shader.
	//
	// Default: black color
	Shader Shader

	// BlendMode is the paint blending mode.
	//
	// Default: SourceOver
	BlendMode BlendMode

	// AntiAlias enables anti-aliased painting.
	//
	// Default: true
	AntiAlias bool

	// Colorspace is the colorspace for blending.
	//
	// This enables gamma correction during the blend operation. While skia
	// supports full color-space conversions, we only support a few (simple)
	// cases. Note that any color space other than Linear forces using the
	// high-quality pipeline.
	//
	// Default: Linear
	Colorspace ColorSpace

	// ForceHQPipeline forces the high quality/precision rendering pipeline.
	//
	// tiny-skia, just like Skia, has two rendering pipelines: one uses f32
	// and another one uses u16. The u16 one is usually way faster, but less
	// precise. Which can lead to slight differences.
	//
	// By default, tiny-skia chooses the pipeline automatically, depending on
	// a blending mode and other parameters. But you can force the high
	// quality one using this flag.
	//
	// This feature is especially useful during testing.
	//
	// Unlike the high quality pipeline, the low quality one does not support
	// all rendering stages, therefore we cannot force it like the hq one.
	//
	// Default: false
	ForceHQPipeline bool
}

// DefaultPaint is the Default impl of Paint in tiny-skia src/painter.rs. It
// returns the default paint: solid black, SourceOver, anti-aliased, linear.
func DefaultPaint() Paint {
	return Paint{
		Shader:          SolidColor(ColorBlack),
		BlendMode:       BlendModeSourceOver,
		AntiAlias:       true,
		Colorspace:      ColorSpaceLinear,
		ForceHQPipeline: false,
	}
}

// SetColor is Paint::set_color in tiny-skia src/painter.rs. It sets a paint
// source to a solid color.
func (p *Paint) SetColor(color Color) {
	p.Shader = SolidColor(color)
}

// SetColorRGBA8 is Paint::set_color_rgba8 in tiny-skia src/painter.rs. It
// sets a paint source to a solid color.
//
// It is a shorthand for p.Shader = SolidColor(ColorFromRGBA8(r, g, b, a)).
func (p *Paint) SetColorRGBA8(r, g, b, a uint8) {
	p.SetColor(ColorFromRGBA8(r, g, b, a))
}

// IsSolidColor is Paint::is_solid_color in tiny-skia src/painter.rs. It
// checks that the paint source is a solid color.
func (p *Paint) IsSolidColor() bool {
	_, ok := p.Shader.(SolidColor)
	return ok
}

// FillRect is Pixmap::fill_rect in tiny-skia src/painter.rs. It draws a
// filled rectangle onto the pixmap.
//
// See PixmapMut.FillRect for details. A nil mask is the Rust None.
func (p *Pixmap) FillRect(
	rect Rect,
	paint *Paint,
	transform Transform,
	mask *Mask,
) {
	m := p.AsMut()
	m.FillRect(rect, paint, transform, mask)
}

// FillPath is Pixmap::fill_path in tiny-skia src/painter.rs. It draws a
// filled path onto the pixmap.
//
// See PixmapMut.FillPath for details. A nil mask is the Rust None.
func (p *Pixmap) FillPath(
	pa *Path,
	paint *Paint,
	fillRule FillRule,
	transform Transform,
	mask *Mask,
) {
	m := p.AsMut()
	m.FillPath(pa, paint, fillRule, transform, mask)
}

// StrokePath is Pixmap::stroke_path in tiny-skia src/painter.rs. It strokes
// a path.
//
// See PixmapMut.StrokePath for details. A nil mask is the Rust None.
func (p *Pixmap) StrokePath(
	pa *Path,
	paint *Paint,
	stroke *Stroke,
	transform Transform,
	mask *Mask,
) {
	m := p.AsMut()
	m.StrokePath(pa, paint, stroke, transform, mask)
}

// DrawPixmap is Pixmap::draw_pixmap in tiny-skia src/painter.rs. It draws a
// Pixmap on top of the current Pixmap.
//
// See PixmapMut.DrawPixmap for details. A nil mask is the Rust None.
func (p *Pixmap) DrawPixmap(
	x, y int32,
	pixmap PixmapRef,
	paint *PixmapPaint,
	transform Transform,
	mask *Mask,
) {
	m := p.AsMut()
	m.DrawPixmap(x, y, pixmap, paint, transform, mask)
}

// ApplyMask is Pixmap::apply_mask in tiny-skia src/painter.rs. It applies a
// mask.
//
// See PixmapMut.ApplyMask for details.
func (p *Pixmap) ApplyMask(mask *Mask) {
	m := p.AsMut()
	m.ApplyMask(mask)
}

// asSubmask returns mask.map(|mask| mask.as_submask()) of tiny-skia
// src/painter.rs. It ports no upstream function.
func asSubmask(mask *Mask) *subMaskRef {
	if mask == nil {
		return nil
	}
	s := mask.asSubmask()
	return &s
}

// submaskOf returns mask.and_then(|mask| mask.submask(rect)) of tiny-skia
// src/painter.rs. It ports no upstream function.
func submaskOf(mask *Mask, rect IntRect) *subMaskRef {
	if mask == nil {
		return nil
	}
	s, ok := mask.submask(rect)
	if !ok {
		return nil
	}
	return &s
}

// FillRect is PixmapMut::fill_rect in tiny-skia src/painter.rs. It draws a
// filled rectangle onto the pixmap.
//
// This function is usually slower than filling a rectangular path, but it
// produces better results. Mainly it does not suffer from weird clipping of
// horizontal/vertical edges.
//
// It is used mainly to render a pixmap onto a pixmap.
//
// A nil mask is the Rust None.
func (m *PixmapMut) FillRect(
	rect Rect,
	paint *Paint,
	transform Transform,
	mask *Mask,
) {
	// TODO: accept NonZeroRect?
	// TODO: we probably can use tiler for rect too
	if transform.IsIdentity() && !drawTilerRequired(m.Width(), m.Height()) {
		// TODO: ignore rects outside the pixmap

		clip := intSizeToScreenIntRect(m.Size(), 0, 0)

		submask := asSubmask(mask)
		subpix := m.asSubpixmap()
		blitter, ok := newRasterPipelineBlitter(paint, submask, &subpix)
		if !ok {
			return // nothing to do, all good
		}

		if paint.AntiAlias {
			fillRectAA(&rect, &clip, blitter)
		} else {
			fillRect(&rect, &clip, blitter)
		}
	} else {
		p := path.PathFromRect(rect)
		m.FillPath(p, paint, FillRuleWinding, transform, mask)
	}
}

// FillPath is PixmapMut::fill_path in tiny-skia src/painter.rs. It draws a
// filled path onto the pixmap.
//
// A nil mask is the Rust None.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// the pixmap, as D20 says.
func (m *PixmapMut) FillPath(
	p *Path,
	paint *Paint,
	fillRule FillRule,
	transform Transform,
	mask *Mask,
) {
	if transform.IsIdentity() {
		// This is sort of similar to SkDraw::drawPath

		// Skip empty paths and horizontal/vertical lines.
		pathBounds := p.Bounds()
		if path.IsNearlyZero(pathBounds.Width()) || path.IsNearlyZero(pathBounds.Height()) {
			warn(m.logger, "empty paths and horizontal/vertical lines cannot be filled")
			return
		}

		if isTooBigForMath(p) {
			warn(m.logger, "path coordinates are too big")
			return
		}

		// TODO: ignore paths outside the pixmap

		if tiler, ok := newDrawTiler(m.Width(), m.Height()); ok {
			p := p.Clone() // TODO: avoid cloning
			paint := *paint

			for tile, ok := tiler.next(); ok; tile, ok = tiler.next() {
				ts := path.TransformFromTranslate(-float32(tile.X()), -float32(tile.Y()))
				var tok bool
				p, tok = p.Transform(ts)
				if !tok {
					warn(m.logger, "path transformation failed")
					return
				}
				paint.Shader = paint.Shader.Transform(ts)

				clipRect := intSizeToScreenIntRect(tile.Size(), 0, 0)
				subpix, ok := m.subpixmap(tile.ToIntRect())
				if !ok {
					continue // technically unreachable
				}

				submask := submaskOf(mask, tile.ToIntRect())
				blitter, ok := newRasterPipelineBlitter(&paint, submask, &subpix)
				if !ok {
					continue // nothing to do, all good
				}

				// We are ignoring "errors" here, because `fill_path` returns `None`
				// when rendering a tile that does not have a path on it.
				// Which is not an error in this case.
				if paint.AntiAlias {
					fillPathPathAA(p, fillRule, &clipRect, blitter, m.logger)
				} else {
					fillPathPath(p, fillRule, &clipRect, blitter, m.logger)
				}

				ts = path.TransformFromTranslate(float32(tile.X()), float32(tile.Y()))
				p, tok = p.Transform(ts)
				if !tok {
					return // technically unreachable
				}
				paint.Shader = paint.Shader.Transform(ts)
			}
		} else {
			clipRect := intSizeToScreenIntRect(m.Size(), 0, 0)
			submask := asSubmask(mask)
			subpix := m.asSubpixmap()
			blitter, ok := newRasterPipelineBlitter(paint, submask, &subpix)
			if !ok {
				return // nothing to do, all good
			}

			if paint.AntiAlias {
				fillPathPathAA(p, fillRule, &clipRect, blitter, m.logger)
			} else {
				fillPathPath(p, fillRule, &clipRect, blitter, m.logger)
			}
		}
	} else {
		p, ok := p.Clone().Transform(transform)
		if !ok {
			warn(m.logger, "path transformation failed")
			return
		}

		paint := *paint
		paint.Shader = paint.Shader.Transform(transform)

		m.FillPath(p, &paint, fillRule, path.IdentityTransform(), mask)
	}
}

// StrokePath is PixmapMut::stroke_path in tiny-skia src/painter.rs. It
// strokes a path.
//
// Stroking is implemented using two separate algorithms:
//
//  1. If a stroke width is wider than 1px (after applying the
//     transformation), a path is converted into a stroked path and then
//     filled using FillPath. Which means that we have to allocate a separate
//     Path, that can be 2-3x larger than the original path.
//  2. If a stroke width is thinner than 1px (after applying the
//     transformation), we use hairline stroking, which does not involve a
//     separate path allocation.
//
// Also, if a stroke has a dash array, then the path is converted into a
// dashed path first and then stroked. Which means yet another allocation.
//
// A nil mask is the Rust None.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// the pixmap, as D20 says.
func (m *PixmapMut) StrokePath(
	p *Path,
	paint *Paint,
	stroke *Stroke,
	transform Transform,
	mask *Mask,
) {
	if stroke.Width < 0.0 {
		warn(m.logger, "negative stroke width isn't allowed")
		return
	}

	resScale := path.ComputeResolutionScale(transform)

	if stroke.Dash != nil {
		dashPath, ok := p.Dash(stroke.Dash, resScale)
		if !ok {
			warn(m.logger, "path dashing failed")
			return
		}
		p = dashPath
	}

	if coverage, ok := treatAsHairline(paint, stroke, transform); ok {
		paint := *paint
		if coverage == 1.0 {
			// No changes to the `paint`.
		} else if paint.BlendMode.shouldPreScaleCoverage() {
			// This is the old technique, which we preserve for now so
			// we do not change previous results (testing)
			// the new way seems fine, it is just (a tiny bit) different.
			scale := wide.TruncI32(coverage * 256.0)
			newAlpha := (255 * scale) >> 8
			paint.Shader = paint.Shader.ApplyOpacity(float32(newAlpha) / 255.0)
		}

		if tiler, ok := newDrawTiler(m.Width(), m.Height()); ok {
			p := p.Clone() // TODO: avoid cloning

			if !transform.IsIdentity() {
				paint.Shader = paint.Shader.Transform(transform)
				var tok bool
				p, tok = p.Transform(transform)
				if !tok {
					warn(m.logger, "path transformation failed")
					return
				}
			}

			for tile, ok := tiler.next(); ok; tile, ok = tiler.next() {
				ts := path.TransformFromTranslate(-float32(tile.X()), -float32(tile.Y()))
				var tok bool
				p, tok = p.Transform(ts)
				if !tok {
					warn(m.logger, "path transformation failed")
					return
				}
				paint.Shader = paint.Shader.Transform(ts)

				subpix, ok := m.subpixmap(tile.ToIntRect())
				if !ok {
					continue // technically unreachable
				}
				submask := submaskOf(mask, tile.ToIntRect())

				// We are ignoring "errors" here, because `stroke_hairline` returns `None`
				// when rendering a tile that does not have a path on it.
				// Which is not an error in this case.
				strokeHairline(p, &paint, stroke.LineCap, submask, &subpix)

				ts = path.TransformFromTranslate(float32(tile.X()), float32(tile.Y()))
				p, tok = p.Transform(ts)
				if !tok {
					return
				}
				paint.Shader = paint.Shader.Transform(ts)
			}
		} else {
			subpix := m.asSubpixmap()
			submask := asSubmask(mask)
			if !transform.IsIdentity() {
				paint.Shader = paint.Shader.Transform(transform)

				// TODO: avoid clone
				p, ok := p.Clone().Transform(transform)
				if !ok {
					warn(m.logger, "path transformation failed")
					return
				}

				strokeHairline(p, &paint, stroke.LineCap, submask, &subpix)
			} else {
				strokeHairline(p, &paint, stroke.LineCap, submask, &subpix)
			}
		}
	} else {
		p, ok := p.Stroke(stroke, resScale)
		if !ok {
			warn(m.logger, "path stroking failed")
			return
		}

		m.FillPath(p, paint, FillRuleWinding, transform, mask)
	}
}

// strokeHairline is PixmapMut::stroke_hairline in tiny-skia src/painter.rs.
// It strokes a path with subpixel/hairline width.
func strokeHairline(
	p *Path,
	paint *Paint,
	lineCap LineCap,
	mask *subMaskRef,
	pixmap *subPixmapMut,
) {
	clip := intSizeToScreenIntRect(pixmap.size, 0, 0)
	blitter, ok := newRasterPipelineBlitter(paint, mask, pixmap)
	if !ok {
		return // nothing to do, all good
	}
	if paint.AntiAlias {
		strokePathHairlineAA(p, lineCap, &clip, blitter)
	} else {
		strokePathHairline(p, lineCap, &clip, blitter)
	}
}

// DrawPixmap is PixmapMut::draw_pixmap in tiny-skia src/painter.rs. It draws
// a Pixmap on top of the current Pixmap.
//
// It is the same as filling a rectangle with a pixmap pattern.
//
// A nil mask is the Rust None.
func (m *PixmapMut) DrawPixmap(
	x, y int32,
	pixmap PixmapRef,
	paint *PixmapPaint,
	transform Transform,
	mask *Mask,
) {
	rect := pixmap.Size().ToIntRect(x, y).ToRect()

	// TODO: SkSpriteBlitter
	// TODO: partially clipped
	// TODO: clipped out

	// Translate pattern as well as bounds.
	pattTransform := path.TransformFromTranslate(float32(x), float32(y))

	p := Paint{
		Shader: NewPattern(
			pixmap,
			SpreadModePad, // Pad, otherwise we get weird borders overlap.
			paint.Quality,
			paint.Opacity,
			pattTransform,
		),
		BlendMode:       paint.BlendMode,
		AntiAlias:       false, // Skia does not use it too.
		ForceHQPipeline: false, // Pattern uses hq anyway.
		Colorspace:      ColorSpaceLinear,
	}

	m.FillRect(rect, &p, transform, mask)
}

// ApplyMask is PixmapMut::apply_mask in tiny-skia src/painter.rs. It applies
// a mask.
//
// When a Mask is passed to drawing methods, it is used to mask-out content
// we are about to draw. This method masks-out an already drawn content. It
// is not as fast, but can be useful when a mask is not available during
// drawing.
//
// This method is similar to filling the whole pixmap with another, mask-like
// pixmap using the DestinationOut blend mode.
//
// The Mask must have the same size as the Pixmap. No transform or offset are
// allowed.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// the pixmap, as D20 says.
func (m *PixmapMut) ApplyMask(mask *Mask) {
	if m.Size() != mask.Size() {
		warn(m.logger, "Pixmap and Mask are expected to have the same size")
		return
	}

	// Just a dummy.
	pixmapSrc := dummyPixmapSrc()

	b := newRasterPipelineBuilder()
	b.push(stageLoadMaskU8)
	b.push(stageLoadDestination)
	b.push(stageDestinationIn)
	b.push(stageStore)
	p := b.compile()
	rect := intSizeToScreenIntRect(m.Size(), 0, 0)
	submask := mask.asSubmask()
	subpix := m.asSubpixmap()
	p.run(
		&rect,
		aaMaskCtx{},
		submask.maskCtx(),
		pixmapSrc,
		&subpix,
	)
}

// treatAsHairline is treat_as_hairline in tiny-skia src/painter.rs.
func treatAsHairline(paint *Paint, stroke *Stroke, ts Transform) (float32, bool) {
	fastLen := func(p Point) float32 {
		x := absF32(p.X)
		y := absF32(p.Y)
		if x < y {
			x, y = y, x
		}

		return x + path.Half(y)
	}

	if stroke.Width == 0.0 {
		return 1.0, true
	}

	if !paint.AntiAlias {
		return 0, false
	}

	// We do not care about translate.
	ts.TX = 0.0
	ts.TY = 0.0

	// We need to try to fake a thick-stroke with a modulated hairline.
	points := []Point{
		path.PointFromXY(stroke.Width, 0.0),
		path.PointFromXY(0.0, stroke.Width),
	}
	ts.MapPoints(points)

	len0 := fastLen(points[0])
	len1 := fastLen(points[1])

	if len0 <= 1.0 && len1 <= 1.0 {
		return path.Ave(len0, len1), true
	}

	return 0, false
}

// isTooBigForMath is is_too_big_for_math in tiny-skia src/painter.rs.
//
// Sometimes in the drawing pipeline, we have to perform math on path
// coordinates, even after the path is in device-coordinates. Tessellation and
// clipping are two examples. Usually this is pretty modest, but it can
// involve subtracting/adding coordinates, or multiplying by small constants
// (e.g. 2,3,4). To try to preflight issues where these operations can turn
// finite path values into infinities (or NaNs), we allow the upper drawing
// code to reject the path if its bounds (in device coordinates) is too close
// to max float.
func isTooBigForMath(p *Path) bool {
	// This value is just a guess. smaller is safer, but we do not want to reject largish paths
	// that we do not have to.
	const scaleDownToAllowForSmallMultiplies float32 = 0.25
	const maxValue float32 = path.ScalarMax * scaleDownToAllowForSmallMultiplies

	b := p.Bounds()

	// use ! expression so we return true if bounds contains NaN
	return !(b.Left() >= -maxValue && b.Top() >= -maxValue && b.Right() <= maxValue && b.Bottom() <= maxValue)
}

// drawTiler is DrawTiler in tiny-skia src/painter.rs. It splits the target
// pixmap into a list of tiles.
//
// Skia/tiny-skia uses a lot of fixed-point math during path rendering.
// Probably more for precision than performance. And our fixed-point types are
// limited by 8192 and 32768. Which means that we cannot render a path larger
// than 8192 onto a pixmap. When a pixmap is smaller than 8192, the path is
// automatically clipped anyway, but for large pixmaps we have to render in
// tiles.
type drawTiler struct {
	imageWidth  uint32
	imageHeight uint32
	xOffset     uint32
	yOffset     uint32
	finished    bool
}

// drawTilerMaxDimensions is DrawTiler::MAX_DIMENSIONS in tiny-skia
// src/painter.rs.
//
// 8K is 1 too big, since 8K << supersample == 32768 which is too big for
// Fixed.
const drawTilerMaxDimensions uint32 = 8192 - 1

// drawTilerRequired is DrawTiler::required in tiny-skia src/painter.rs.
func drawTilerRequired(imageWidth, imageHeight uint32) bool {
	return imageWidth > drawTilerMaxDimensions || imageHeight > drawTilerMaxDimensions
}

// newDrawTiler is DrawTiler::new in tiny-skia src/painter.rs.
func newDrawTiler(imageWidth, imageHeight uint32) (*drawTiler, bool) {
	if drawTilerRequired(imageWidth, imageHeight) {
		return &drawTiler{
			imageWidth:  imageWidth,
			imageHeight: imageHeight,
			xOffset:     0,
			yOffset:     0,
			finished:    false,
		}, true
	}
	return nil, false
}

// next is the Iterator impl of DrawTiler in tiny-skia src/painter.rs.
func (t *drawTiler) next() (screenIntRect, bool) {
	if t.finished {
		return screenIntRect{}, false
	}

	// TODO: iterate only over tiles that actually affected by the shape

	if t.xOffset < t.imageWidth && t.yOffset < t.imageHeight {
		var h uint32
		if t.yOffset < t.imageHeight {
			h = min(t.imageHeight-t.yOffset, drawTilerMaxDimensions)
		} else {
			h = t.imageHeight
		}

		r, ok := screenIntRectFromXYWH(
			t.xOffset,
			t.yOffset,
			min(t.imageWidth-t.xOffset, drawTilerMaxDimensions),
			h,
		)

		t.xOffset += drawTilerMaxDimensions
		if t.xOffset >= t.imageWidth {
			t.xOffset = 0
			t.yOffset += drawTilerMaxDimensions
		}

		return r, ok
	}

	return screenIntRect{}, false
}
