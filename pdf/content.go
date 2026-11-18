// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports krilla src/content.rs, a low-level abstraction over a
// single content stream.
//
// The port leaves out these parts of upstream:
//
//   - The text: draw_glyphs, encode_consecutive_glyph_run,
//     encode_single_glyph, encode_glyphs_with_individual_positioning,
//     fill_stroke_glyph_run, fill_stroke_glyph_span, fill_stroke_glyph_group
//     and get_glyphs_bbox, with the field scratch of ContentBuilder. Phase 16
//     ports them into this file, as methods of contentBuilder.
//   - The images: draw_image. Phase 15 ports it into this file.
//   - The marked content of the tags and of the forms: the fields
//     active_marked_content and xobject_identifier of ContentBuilder, and
//     start_marked_content, start_marked_content_with_properties and
//     end_marked_content. The base port has no tags and no forms (D62).
//   - draw_xobject_by_reference, for the pages of an embedded PDF file,
//     which the base port leaves out (D62).

// contentBuilder is ContentBuilder in krilla src/content.rs.
type contentBuilder struct {
	rdBuilder *resourceDictionaryBuilder
	content   *pdfwriter.Content
	// validationErrors is the HashSet of upstream. It keeps each error once,
	// in the order of its first use.
	validationErrors []validationError
	rootTransform    Transform
	graphicsStates   *graphicsStates
	// bbox is nil until the first drawing.
	bbox     *Rect
	usesMask bool
	// bboxImportant is true inside an XObject. Calculating the bbox of text
	// is expensive, so we avoid it if not needed. The only time we really
	// need it is if we are currently inside of an XObject, where we need to
	// provide a bbox of all its contents.
	bboxImportant bool
}

// contentColorSpace is ContentColorSpace in krilla src/content.rs. It stores
// either a device-specific color space, or the name of a different color
// space (e.g. ICCBased) stored in the current resource dictionary. An empty
// name is the case Device, because the names of resources are not empty.
type contentColorSpace struct {
	named string
}

// contentColorSpaceFromColorSpace is ContentColorSpace::from_color_space in
// krilla src/content.rs.
func contentColorSpaceFromColorSpace(sc *serializeContext, cc *chunkContainer, rdBuilder *resourceDictionaryBuilder, cs colorSpace) contentColorSpace {
	if m := sc.registerColorSpace(cc, cs); m.kind == maybeDeviceResource {
		return contentColorSpace{named: rdBuilder.registerResource(m.cs)}
	}
	return contentColorSpace{}
}

// newContentBuilder is ContentBuilder::new in krilla src/content.rs.
func newContentBuilder(rootTransform Transform, bboxImportant bool, sc *serializeContext) *contentBuilder {
	return &contentBuilder{
		rdBuilder:      newResourceDictionaryBuilder(),
		content:        sc.newContent(),
		rootTransform:  rootTransform,
		bboxImportant:  bboxImportant,
		graphicsStates: newGraphicsStates(),
	}
}

// addValidationError is the insertion into the HashSet validation_errors of
// upstream.
func (b *contentBuilder) addValidationError(err validationError) {
	if !slices.Contains(b.validationErrors, err) {
		b.validationErrors = append(b.validationErrors, err)
	}
}

// contentSaveState is ContentBuilder::content_save_state in krilla
// src/content.rs.
func (b *contentBuilder) contentSaveState() {
	b.content.SaveState()

	if b.content.StateNestingDepth() > 28 {
		b.addValidationError(validationError{kind: validationTooHighQNestingLevel})
	}
}

// finish is ContentBuilder::finish in krilla src/content.rs.
func (b *contentBuilder) finish(sc *serializeContext) *Stream {
	buf := b.content.Finish()
	sc.registerLimits(buf.Limits())

	bbox, _ := path.RectFromXYWH(0, 0, 1, 1)
	if b.bbox != nil {
		bbox = *b.bbox
	}
	return newStream(buf.IntoVec(), bbox, b.validationErrors, b.rdBuilder.finish(), b.usesMask)
}

// concatTransform is ContentBuilder::concat_transform in krilla
// src/content.rs.
func (b *contentBuilder) concatTransform(transform Transform) {
	b.graphicsStates.transform(transform)
}

// curTransformWithRootTransform is
// ContentBuilder::cur_transform_with_root_transform in krilla src/content.rs.
func (b *contentBuilder) curTransformWithRootTransform() Transform {
	return b.rootTransform.PreConcat(b.curTransform())
}

// curTransform is ContentBuilder::cur_transform in krilla src/content.rs.
func (b *contentBuilder) curTransform() Transform {
	return b.graphicsStates.cur().transform()
}

// saveGraphicsState is ContentBuilder::save_graphics_state in krilla
// src/content.rs.
func (b *contentBuilder) saveGraphicsState() {
	b.graphicsStates.saveState()
}

// restoreGraphicsState is ContentBuilder::restore_graphics_state in krilla
// src/content.rs.
func (b *contentBuilder) restoreGraphicsState() {
	b.graphicsStates.restoreState()
}

// setBlendMode is ContentBuilder::set_blend_mode in krilla src/content.rs.
func (b *contentBuilder) setBlendMode(blendMode pdfwriter.BlendMode) {
	if blendMode != pdfwriter.BlendModeNormal {
		state := newExtGState().blendMode(blendMode)
		b.graphicsStates.combine(state)
	}
}

// expandBBox is ContentBuilder::expand_bbox in krilla src/content.rs. krilla
// panics when the current transform makes the rectangle invalid, which a
// transform with large values does. The port then leaves the bbox as it is,
// as rectExpand does.
func (b *contentBuilder) expandBBox(newBBox Rect) {
	newBBox, ok := b.graphicsStates.transformBBox(newBBox)
	if !ok {
		return
	}
	if b.bbox != nil {
		rectExpand(b.bbox, newBBox)
	} else {
		b.bbox = &newBBox
	}
}

// drawPath is ContentBuilder::draw_path in krilla src/content.rs. A nil fill
// or stroke is None.
func (b *contentBuilder) drawPath(p *Path, fill *Fill, stroke *Stroke, sc *serializeContext, cc *chunkContainer) {
	if fill == nil && stroke == nil {
		return
	}

	// Zero-size geometry, don't draw.
	bounds := p.Bounds()
	if bounds.Width() == 0 && bounds.Height() == 0 &&
		(stroke == nil || stroke.Width == 0 || stroke.LineCap != LineCapRound) {
		return
	}

	// See issue 199.
	verbs := p.Verbs()
	isLine := len(verbs) == 2 && verbs[0] == path.PathVerbMove && verbs[1] == path.PathVerbLine
	dontFill := bounds.Width() == 0 || bounds.Height() == 0 || isLine

	if dontFill {
		if stroke == nil {
			// Some PDF viewers have bugs where they slightly fill zero-sized
			// lines, so don't draw them in the first place.
			return
		}
		// Don't fill, but we still want to draw the stroke, so don't
		// return.
		fill = nil
	}

	bboxImportant := b.bboxImportant
	calculateBBox := func(isSolid bool) bool { return bboxImportant || !isSolid }

	strokeBBox := func(stroke *Stroke) Rect {
		_, isSolid := stroke.Paint.(Color)
		if calculateBBox(isSolid) {
			// The stroker of krilla can panic for a path with coordinates
			// near the largest float32, or not end for a point that is NaN.
			// pdf records ErrPDFInvalidPath, which Document.Finish returns,
			// and does not stroke the path (D66).
			if !strokeFits(p, stroke.Width) {
				sc.setError(fmt.Errorf("%w: the path with the bounds %s and a stroke of the width %v", okage.ErrPDFInvalidPath, bounds, stroke.Width))
				return bounds
			}
			ts := stroke.intoTinySkia()
			if r, ok := calculateStrokeBBox(&ts, p); ok {
				return r
			}
		}
		return bounds
	}

	fillPrep := func(sb *contentBuilder, fill *Fill) {
		_, hasPattern := fill.Paint.(Pattern)
		fillOpacity := fill.Opacity
		sb.expandBBox(bounds)

		// PDF viewers don't show patterns with fill/stroke opacities
		// consistently. Because of this, the opacity is accounted for in the
		// pattern itself.
		if !hasPattern {
			sb.setFillOpacity(fillOpacity)
		}
	}

	strokePrep := func(sb *contentBuilder, stroke *Stroke, strokeBBox Rect) {
		_, isPattern := stroke.Paint.(Pattern)
		strokeOpacity := stroke.Opacity
		sb.expandBBox(strokeBBox)

		// See comment in `set_fill_properties`
		if !isPattern {
			sb.setStrokeOpacity(strokeOpacity)
		}
	}

	fillOp := func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer, fill *Fill) {
		fillRule := fill.Rule
		sb.contentSetFillProperties(bounds, fill, sc, cc)
		sb.contentDrawPath(p.Segments())

		switch fillRule {
		case FillRuleNonZero:
			sb.content.FillNonzero()
		case FillRuleEvenOdd:
			sb.content.FillEvenOdd()
		}
	}

	strokeOp := func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer, stroke *Stroke, strokeBBox Rect) {
		sb.contentSetStrokeProperties(strokeBBox, stroke, sc, cc)
		sb.contentDrawPath(p.Segments())
		sb.content.Stroke()
	}

	fillStrokeOp := func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer, fill *Fill, stroke *Stroke, strokeBBox Rect) {
		fillRule := fill.Rule
		sb.contentSetFillProperties(bounds, fill, sc, cc)
		sb.contentSetStrokeProperties(strokeBBox, stroke, sc, cc)
		sb.contentDrawPath(p.Segments())

		switch fillRule {
		case FillRuleNonZero:
			sb.content.FillNonzeroAndStroke()
		case FillRuleEvenOdd:
			sb.content.FillEvenOddAndStroke()
		}
	}

	switch {
	case fill != nil && stroke == nil:
		b.applyIsolatedOp(
			func(sb *contentBuilder) {
				fillPrep(sb, fill)
			},
			func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
				fillOp(sb, sc, cc, fill)
			},
			sc,
			cc,
		)
	case fill == nil && stroke != nil:
		strokeBBox := strokeBBox(stroke)

		b.applyIsolatedOp(
			func(sb *contentBuilder) {
				strokePrep(sb, stroke, strokeBBox)
			},
			func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
				strokeOp(sb, sc, cc, stroke, strokeBBox)
			},
			sc,
			cc,
		)
	default:
		strokeBBox := strokeBBox(stroke)

		b.applyIsolatedOp(
			func(sb *contentBuilder) {
				fillPrep(sb, fill)
				strokePrep(sb, stroke, strokeBBox)
			},
			func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
				fillStrokeOp(sb, sc, cc, fill, stroke, strokeBBox)
			},
			sc,
			cc,
		)
	}
}

// maxStrokeCoordinate is the largest magnitude of a coordinate of a path,
// with the width of its stroke, that pdf gives to the stroker. It is 2^60.
// The stroker multiplies the differences of the coordinates, and below this
// value no product overflows a float32.
const maxStrokeCoordinate = 1 << 60

// strokeFits reports whether the stroker can stroke the path with the width.
// It is not in krilla. tiny-skia panics in find_quad_max_curvature for a
// quadratic curve with coordinates near the largest float32, so krilla
// panics there too. It does not end for some paths with a point that is
// NaN, whose bounds leave the NaN out. A width that is not finite gives no
// stroke, so it fits.
func strokeFits(p *Path, width float32) bool {
	w := float32(0)
	switch {
	case math.IsInf(float64(width), 0) || width != width:
	case width > maxStrokeCoordinate:
		return false
	case width > 0:
		w = width
	}
	for _, pt := range p.Points() {
		// A NaN fails both comparisons.
		if !(max(pt.X, -pt.X)+w <= maxStrokeCoordinate) || !(max(pt.Y, -pt.Y)+w <= maxStrokeCoordinate) {
			return false
		}
	}
	return true
}

// gradientStopCount gives the number of stops of a gradient. It is not in
// krilla.
func gradientStopCount(gp gradientProperties) int {
	switch g := gp.(type) {
	case radialAxialGradient:
		return len(g.stops)
	case postScriptGradient:
		return len(g.stops)
	}
	return 0
}

// pushClipPath is ContentBuilder::push_clip_path in krilla src/content.rs.
// krilla panics when the current transform makes the path invalid, which a
// transform with large values does. The port then records
// ErrPDFInvalidClipPath, which Document.Finish returns, and clips with the
// path as it is, so that the content stream stays valid.
func (b *contentBuilder) pushClipPath(p *Path, clipRule FillRule, sc *serializeContext) {
	b.contentSaveState()
	transformed, ok := p.Clone().Transform(b.curTransformWithRootTransform())
	if !ok {
		sc.setError(fmt.Errorf("%w: the path with the bounds %s", okage.ErrPDFInvalidClipPath, p.Bounds()))
		transformed = p
	}
	b.contentDrawPath(transformed.Segments())

	switch clipRule {
	case FillRuleNonZero:
		b.content.ClipNonzero()
	case FillRuleEvenOdd:
		b.content.ClipEvenOdd()
	}

	b.content.EndPath()
}

// popClipPath is ContentBuilder::pop_clip_path in krilla src/content.rs.
func (b *contentBuilder) popClipPath() {
	b.content.RestoreState()
}

// drawXObject is ContentBuilder::draw_xobject in krilla src/content.rs.
func (b *contentBuilder) drawXObject(sc *serializeContext, cc *chunkContainer, xObject xObject, state extGState) {
	bbox := xObject.bbox()
	b.applyIsolatedOp(
		func(sb *contentBuilder) {
			sb.graphicsStates.combine(state)
			sb.expandBBox(bbox)
		},
		func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
			xObjectName := sb.rdBuilder.registerResource(registerResourceable(sc, cc, xObject))
			sb.content.XObject(pdfwriter.Name(xObjectName))
		},
		sc,
		cc,
	)
}

// drawMasked is ContentBuilder::draw_masked in krilla src/content.rs.
func (b *contentBuilder) drawMasked(sc *serializeContext, cc *chunkContainer, mask *Mask, stream *Stream) {
	state := newExtGState().mask(mask, sc, cc)
	b.usesMask = true
	xObject := newXObject(stream, false, true, nil)
	b.drawXObject(sc, cc, xObject, state)
}

// drawOpacified is ContentBuilder::draw_opacified in krilla src/content.rs.
func (b *contentBuilder) drawOpacified(sc *serializeContext, cc *chunkContainer, opacity NormalizedF32, stream *Stream) {
	state := newExtGState().
		strokingAlpha(opacity).
		nonStrokingAlpha(opacity)
	xObject := newXObject(stream, true, false, nil)
	b.drawXObject(sc, cc, xObject, state)
}

// drawIsolated is ContentBuilder::draw_isolated in krilla src/content.rs.
func (b *contentBuilder) drawIsolated(sc *serializeContext, cc *chunkContainer, stream *Stream) {
	state := newExtGState()
	xObject := newXObject(stream, true, false, nil)
	b.drawXObject(sc, cc, xObject, state)
}

// drawShading is ContentBuilder::draw_shading in krilla src/content.rs.
func (b *contentBuilder) drawShading(shading shadingFunction, sc *serializeContext, cc *chunkContainer) {
	b.applyIsolatedOp(
		func(*contentBuilder) {},
		func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer) {
			sh := sb.rdBuilder.registerResource(registerResourceable(sc, cc, shading))
			sb.content.Shading(pdfwriter.Name(sh))
		},
		sc,
		cc,
	)
}

// setFillOpacity is ContentBuilder::set_fill_opacity in krilla
// src/content.rs.
func (b *contentBuilder) setFillOpacity(alpha NormalizedF32) {
	if alpha.Get() != 1 {
		state := newExtGState().nonStrokingAlpha(alpha)
		b.graphicsStates.combine(state)
	}
}

// setStrokeOpacity is ContentBuilder::set_stroke_opacity in krilla
// src/content.rs.
func (b *contentBuilder) setStrokeOpacity(alpha NormalizedF32) {
	if alpha.Get() != 1 {
		state := newExtGState().strokingAlpha(alpha)
		b.graphicsStates.combine(state)
	}
}

// applyIsolatedOp is ContentBuilder::apply_isolated_op in krilla
// src/content.rs. The function prep of upstream takes the serializer and the
// chunk container too, which no caller uses, so it takes the builder only.
func (b *contentBuilder) applyIsolatedOp(
	prep func(sb *contentBuilder),
	op func(sb *contentBuilder, sc *serializeContext, cc *chunkContainer),
	sc *serializeContext,
	cc *chunkContainer,
) {
	b.saveGraphicsState()
	b.contentSaveState()

	prep(b)

	transform := b.curTransformWithRootTransform()

	if transform != path.IdentityTransform() {
		b.content.Transform(transformToPDF(transform))
	}

	state := b.graphicsStates.cur().extGState()

	if !state.empty() {
		ext := b.rdBuilder.registerResource(registerResourceable(sc, cc, state))
		b.content.SetParameters(pdfwriter.Name(ext))
	}

	op(b, sc, cc)

	b.content.RestoreState()
	b.restoreGraphicsState()
}

// contentSetFillStrokeProperties is
// ContentBuilder::content_set_fill_stroke_properties in krilla
// src/content.rs.
func (b *contentBuilder) contentSetFillStrokeProperties(
	bounds Rect,
	paint Paint,
	opacity NormalizedF32,
	sc *serializeContext,
	cc *chunkContainer,
	setPatternFn func(content *pdfwriter.Content, colorSpace string),
	setSolidFn func(content *pdfwriter.Content, colorSpace contentColorSpace, color Color),
) {
	patternTransform := func(transform Transform) Transform {
		return transform.PostConcat(b.curTransformWithRootTransform())
	}

	writeGradient := func(gradientProps gradientProperties, sc *serializeContext, cc *chunkContainer, transform Transform, contentBuilder *contentBuilder) {
		if color, opacity, ok := gradientProps.singleStopColor(); ok {
			// Write gradients with one stop as a solid color fill.
			contentBuilder.setFillOpacity(opacity)
			cs := color.colorSpace(sc)
			colorSpaceResource := contentColorSpaceFromColorSpace(sc, cc, contentBuilder.rdBuilder, cs)
			setSolidFn(contentBuilder.content, colorSpaceResource, color)
		} else {
			shadingMask := newMaskFromShading(gradientProps, transform, bounds, sc, cc)

			shadingPattern := newShadingPattern(
				gradientProps,
				contentBuilder.curTransformWithRootTransform().PreConcat(transform),
			)
			colorSpace := contentBuilder.rdBuilder.registerResource(registerResourceable(sc, cc, shadingPattern))

			if shadingMask != nil {
				state := newExtGState().mask(shadingMask, sc, cc)
				contentBuilder.usesMask = true

				ext := contentBuilder.rdBuilder.registerResource(registerResourceable(sc, cc, state))
				contentBuilder.content.SetParameters(pdfwriter.Name(ext))
			}

			setPatternFn(contentBuilder.content, colorSpace)
		}
	}

	// paintGradient writes a gradient, from the result of its method
	// gradientProperties. krilla panics for a gradient with no stop, and for
	// a transform of a gradient that cannot be inverted where the gradient
	// needs its inverse. pdf records ErrPDFInvalidGradient, which
	// Document.Finish returns, and sets no paint (D62).
	paintGradient := func(gradientProps gradientProperties, transform Transform, ok bool, gradientTransform Transform) {
		if !ok {
			sc.setError(fmt.Errorf("%w: the transform %v cannot be inverted for the bounds %s", okage.ErrPDFInvalidGradient, gradientTransform, bounds))
			return
		}
		if gradientStopCount(gradientProps) == 0 {
			sc.setError(fmt.Errorf("%w: the gradient has no stop", okage.ErrPDFInvalidGradient))
			return
		}
		writeGradient(gradientProps, sc, cc, transform, b)
	}

	switch p := paint.(type) {
	case Color:
		cs := p.colorSpace(sc)
		colorSpaceResource := contentColorSpaceFromColorSpace(sc, cc, b.rdBuilder, cs)
		setSolidFn(b.content, colorSpaceResource, p)
	case LinearGradient:
		gradientProps, transform, ok := p.gradientProperties(bounds)
		paintGradient(gradientProps, transform, ok, p.Transform)
	case RadialGradient:
		gradientProps, transform, ok := p.gradientProperties(bounds)
		paintGradient(gradientProps, transform, ok, p.Transform)
	case SweepGradient:
		gradientProps, transform, ok := p.gradientProperties(bounds)
		paintGradient(gradientProps, transform, ok, p.Transform)
	case Pattern:
		pat := p
		pat.Transform = patternTransform(pat.Transform)

		tilingPattern := newTilingPattern(
			pat.Stream,
			pat.Transform,
			opacity,
			pat.Width,
			pat.Height,
			sc,
			cc,
		)

		colorSpace := b.rdBuilder.registerResource(registerResourceable(sc, cc, tilingPattern))
		setPatternFn(b.content, colorSpace)
	}
}

// contentSetFillProperties is ContentBuilder::content_set_fill_properties in
// krilla src/content.rs.
func (b *contentBuilder) contentSetFillProperties(bounds Rect, fill *Fill, sc *serializeContext, cc *chunkContainer) {
	setPatternFn := func(content *pdfwriter.Content, colorSpace string) {
		content.SetFillColorSpace(pdfwriter.ColorSpaceOperand{Kind: pdfwriter.ColorSpaceOperandPattern})
		content.SetFillPattern(nil, pdfwriter.Name(colorSpace))
	}

	b.contentSetFillStrokeProperties(
		bounds,
		fill.Paint,
		fill.Opacity,
		sc,
		cc,
		setPatternFn,
		setSolidFill,
	)
}

// contentSetStrokeProperties is
// ContentBuilder::content_set_stroke_properties in krilla src/content.rs.
func (b *contentBuilder) contentSetStrokeProperties(bounds Rect, stroke *Stroke, sc *serializeContext, cc *chunkContainer) {
	setPatternFn := func(content *pdfwriter.Content, colorSpace string) {
		content.SetStrokeColorSpace(pdfwriter.ColorSpaceOperand{Kind: pdfwriter.ColorSpaceOperandPattern})
		content.SetStrokePattern(nil, pdfwriter.Name(colorSpace))
	}

	setSolidFn := func(content *pdfwriter.Content, colorSpace contentColorSpace, color Color) {
		if colorSpace.named == "" {
			// krilla panics here for a separation color, which the base port
			// leaves out (D62).
			comps := color.toPDFColor()
			switch color.(type) {
			case ColorRGB:
				content.SetStrokeRGB(comps[0], comps[1], comps[2])
			case ColorLuma:
				content.SetStrokeGray(comps[0])
			case ColorCMYK:
				content.SetStrokeCMYK(comps[0], comps[1], comps[2], comps[3])
			}
		} else {
			content.SetStrokeColorSpace(pdfwriter.ColorSpaceOperandFromName(pdfwriter.Name(colorSpace.named)))
			content.SetStrokeColor(color.toPDFColor()...)
		}
	}

	b.contentSetFillStrokeProperties(
		bounds,
		stroke.Paint,
		stroke.Opacity,
		sc,
		cc,
		setPatternFn,
		setSolidFn,
	)

	// Only write if they don't correspond to the default values as defined
	// in the PDF specification.
	if stroke.Width != 1 {
		// pdf-writer panics for a width that is negative or NaN, and so
		// does krilla. The port records ErrPDFInvalidStroke, which
		// Document.Finish returns, and writes no width.
		if stroke.Width >= 0 {
			b.content.SetLineWidth(stroke.Width)
		} else {
			sc.setError(fmt.Errorf("%w: the width %v", okage.ErrPDFInvalidStroke, stroke.Width))
		}
	}

	if stroke.MiterLimit != 10 {
		b.content.SetMiterLimit(stroke.MiterLimit)
	}

	if stroke.LineCap != LineCapButt {
		b.content.SetLineCap(stroke.LineCap.toPDFLineCap())
	}

	if stroke.LineJoin != LineJoinMiter {
		b.content.SetLineJoin(stroke.LineJoin.toPDFLineJoin())
	}

	if stroke.Dash != nil {
		b.content.SetDashPattern(stroke.Dash.Array, stroke.Dash.Offset)
	}
}

// contentDrawPath is ContentBuilder::content_draw_path in krilla
// src/content.rs.
func (b *contentBuilder) contentDrawPath(pathData path.PathSegmentsIter) {
	// Taken from resvg
	calc := func(n1, n2 float32) float32 {
		return (n1 + n2*2) / 3
	}

	var pPrev Point

	for {
		operation, ok := pathData.Next()
		if !ok {
			break
		}
		switch operation.Kind {
		case path.PathSegmentMoveTo:
			p := operation.Points[0]
			b.content.MoveTo(p.X, p.Y)
			pPrev = p
		case path.PathSegmentLineTo:
			p := operation.Points[0]
			b.content.LineTo(p.X, p.Y)
			pPrev = p
		case path.PathSegmentQuadTo:
			// Since PDF doesn't support quad curves, we need to convert them
			// into cubic.
			p1, p2 := operation.Points[0], operation.Points[1]
			prev := pPrev
			b.content.CubicTo(
				calc(prev.X, p1.X),
				calc(prev.Y, p1.Y),
				calc(p2.X, p1.X),
				calc(p2.Y, p1.Y),
				p2.X,
				p2.Y,
			)
			pPrev = p2
		case path.PathSegmentCubicTo:
			p1, p2, p3 := operation.Points[0], operation.Points[1], operation.Points[2]
			b.content.CubicTo(p1.X, p1.Y, p2.X, p2.Y, p3.X, p3.Y)
			pPrev = p3
		case path.PathSegmentClose:
			b.content.ClosePath()
		}
	}
}

// setSolidFill is set_solid_fill in krilla src/content.rs.
func setSolidFill(content *pdfwriter.Content, colorSpace contentColorSpace, color Color) {
	if colorSpace.named == "" {
		// krilla panics here for a separation color, which the base port
		// leaves out (D62).
		comps := color.toPDFColor()
		switch color.(type) {
		case ColorRGB:
			content.SetFillRGB(comps[0], comps[1], comps[2])
		case ColorLuma:
			content.SetFillGray(comps[0])
		case ColorCMYK:
			content.SetFillCMYK(comps[0], comps[1], comps[2], comps[3])
		}
	} else {
		content.SetFillColorSpace(pdfwriter.ColorSpaceOperandFromName(pdfwriter.Name(colorSpace.named)))
		content.SetFillColor(color.toPDFColor()...)
	}
}
