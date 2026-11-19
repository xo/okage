// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file ports krilla src/surface.rs, drawing onto a surface, with the
// methods of Page and StreamBuilder that make a surface.
//
// The port leaves out these parts of upstream:
//
//   - The tags and the forms: start_tagged, end_tagged, start_variable_text
//     and end_variable_text, the field page_identifier of Surface, the
//     number of marked content IDs that finish_fn takes, and
//     has_active_marked_content of Builders. The base port has no tags and no
//     forms (D62).
//   - draw_pdf_page, for an embedded PDF file (D62).
//
// pdfsvg.Draw draws an SVG tree. It calls the methods of Surface, as
// krilla-svg implements the trait SurfaceExt.

// Surface is Surface in krilla src/surface.rs.
//
// A surface represents a drawing area for defining graphical content. The
// origin of the coordinate axis is in the top-left corner.
//
// You cannot directly create an instance of a Surface yourself. Instead,
// there are two ways of getting access to a surface, which you can then use
// to draw on:
//
//   - The first way, and also the most common one you will use, is by
//     creating a new document, adding a page to it and then invoking the
//     method Page.Surface. The surface returned as part of that represents
//     the drawing area of the page.
//   - The second way is by calling the method Surface.StreamBuilder, to
//     create a sub-drawing context. See the documentation of Stream for more
//     information.
//
// The surface uses a push and pop based mechanism for applying certain
// actions. For example, the method PushTransform concatenates a new
// transform to the current transform matrix. The method PushClipPath
// intersects the current drawing area with a clip path. Once you call such
// a push method, the action that it invokes will be in place until you call
// the method Pop, which will then revert the last push operation. This
// allows you to, for example, define a clipping path area or a mask to use
// only for certain objects.
//
// For each push method you invoke, there must be a corresponding call of
// Pop that reverts it. krilla panics when the pushes and the pops differ.
// The port records ErrPDFPushPop, which Document.Finish returns.
//
// Rust drops a surface at the end of its scope. In Go, call Finish when the
// drawing is done. Page.Finish finishes the surface of the page. Finish the
// surface of a StreamBuilder before StreamBuilder.Finish. Do not use a
// surface after Finish.
type Surface struct {
	sc               *serializeContext
	cc               *chunkContainer
	fill             *Fill
	stroke           *Stroke
	bd               builders
	pushInstructions []pushInstruction
	// finishFn gives the stream of the surface to the code that made it.
	finishFn func(stream *Stream)
	finished bool
}

// newSurface is Surface::new in krilla src/surface.rs.
func newSurface(sc *serializeContext, cc *chunkContainer, rootBuilder *contentBuilder, baseTransform Transform, finishFn func(stream *Stream)) *Surface {
	return &Surface{
		sc:       sc,
		cc:       cc,
		bd:       newBuilders(rootBuilder, baseTransform),
		finishFn: finishFn,
	}
}

// Surface is Page::surface in krilla src/page.rs.
//
// Get the surface of the page to draw on. Calling this multiple times on the
// same page will reset any previous drawings.
func (p *Page) Surface() *Surface {
	if p.finishSurface != nil {
		p.finishSurface()
	}
	rootBuilder := newContentBuilder(p.rootTransform(), false, p.sc)
	s := newSurface(p.sc, p.cc, rootBuilder, path.IdentityTransform(), func(stream *Stream) {
		p.pageStream = stream
		p.finishSurface = nil
	})
	p.finishSurface = s.Finish
	return s
}

// Surface is StreamBuilder::surface in krilla src/stream.rs.
//
// Get the surface of the stream builder.
func (b *StreamBuilder) Surface() *Surface {
	contentBuilder := newContentBuilder(path.IdentityTransform(), true, b.sc)
	return b.surfaceWithContentBuilder(contentBuilder)
}

// SurfaceWithBBox is StreamBuilder::surface_with_bbox in krilla
// src/stream.rs.
//
// Get the surface of the stream builder. Unlike Surface, the returned
// surface has its y-axis inverted similarly to the surface of a page.
func (b *StreamBuilder) SurfaceWithBBox(bbox Rect) *Surface {
	transform := path.TransformFromRow(1, 0, 0, -1, 0, bbox.Height())
	contentBuilder := newContentBuilder(transform, false, b.sc)
	contentBuilder.expandBBox(bbox)
	return b.surfaceWithContentBuilder(contentBuilder)
}

// surfaceWithContentBuilder is StreamBuilder::surface_with_content_builder
// in krilla src/stream.rs.
func (b *StreamBuilder) surfaceWithContentBuilder(contentBuilder *contentBuilder) *Surface {
	return newSurface(b.sc, b.cc, contentBuilder, b.baseTransform, func(stream *Stream) {
		b.stream = stream
	})
}

// StreamBuilder is Surface::stream_builder in krilla src/surface.rs.
//
// Return a StreamBuilder to allow drawing on a sub-context.
func (s *Surface) StreamBuilder() *StreamBuilder {
	baseTransform := s.CTM()
	return newStreamBuilder(s.sc, s.cc, baseTransform)
}

// SetFill is Surface::set_fill in krilla src/surface.rs. The surface keeps a
// copy of the fill, and nil is None.
//
// Set the fill that should be used for the next drawing operation.
//
// You can set it to nil if you want to disable filling (though if there is
// no active stroke, then krilla will choose to fill with black by default).
func (s *Surface) SetFill(fill *Fill) {
	s.fill = nil
	if fill != nil {
		f := *fill
		s.fill = &f
	}
}

// GetFill is Surface::get_fill in krilla src/surface.rs. It returns a copy of
// the fill, or nil.
//
// Get the currently active fill.
func (s *Surface) GetFill() *Fill {
	if s.fill == nil {
		return nil
	}
	f := *s.fill
	return &f
}

// SetStroke is Surface::set_stroke in krilla src/surface.rs. The surface
// keeps a copy of the stroke, and nil is None.
//
// Set the stroke that should be used for drawing operations.
//
// You can set it to nil if you want to disable stroking.
func (s *Surface) SetStroke(stroke *Stroke) {
	s.stroke = nil
	if stroke != nil {
		k := *stroke
		s.stroke = &k
	}
}

// GetStroke is Surface::get_stroke in krilla src/surface.rs. It returns a
// copy of the stroke, or nil.
//
// Get the currently active stroke.
func (s *Surface) GetStroke() *Stroke {
	if s.stroke == nil {
		return nil
	}
	k := *s.stroke
	return &k
}

// DrawPath is Surface::draw_path in krilla src/surface.rs.
//
// Draw a path using the currently active fill and/or stroke.
func (s *Surface) DrawPath(p *Path) {
	// okage: a mesh with opacities fills through a soft mask of its own
	// (D68).
	if s.drawMeshFill(p) {
		return
	}
	if s.fill != nil || s.stroke != nil {
		if s.hasComplexFillOrStroke() {
			s.bd.getMut().drawPath(p, s.fill, nil, s.sc, s.cc)
			s.bd.getMut().drawPath(p, nil, s.stroke, s.sc, s.cc)
		} else {
			s.bd.getMut().drawPath(p, s.fill, s.stroke, s.sc, s.cc)
		}
	} else {
		// Draw with black by default.
		fill := DefaultFill()
		s.bd.getMut().drawPath(p, &fill, nil, s.sc, s.cc)
	}
}

// outlineGlyphs is Surface::outline_glyphs in krilla src/surface.rs.
func (s *Surface) outlineGlyphs(glyphs []Glyph, contextColor ColorRGB, start Point, font *Font, fontSize float32) {
	curX, curY := start.X, start.Y

	for i := range glyphs {
		glyph := &glyphs[i]
		baseTransform := path.TransformFromTranslate(
			curX+glyph.xOffset(fontSize),
			curY-glyph.yOffset(fontSize),
		)
		baseTransform = baseTransform.PreConcat(path.TransformFromScale(
			fontSize/font.UnitsPerEm(),
			-fontSize/font.UnitsPerEm(),
		))
		drawGlyph(font, contextColor, glyph.GlyphID, baseTransform, s)

		curX += glyph.xAdvance(fontSize)
		curY -= glyph.yAdvance(fontSize)
	}
}

// DrawGlyphs is Surface::draw_glyphs in krilla src/surface.rs. text holds
// the characters of the clusters of the glyphs, which the TextRange of each
// glyph gives. A font draws the text that a reader can copy or search, from
// the text of each cluster (D62, question 27). When outlined is true, the
// glyphs are drawn as paths.
//
// krilla panics for a glyph whose range is outside the text or does not
// fall on the bounds of its characters, and for a glyph ID above 65535 in a
// CID font. Unless outlined is true, the port then records
// ErrPDFInvalidGlyph, which Document.Finish returns, and draws nothing.
//
// Draw a sequence of glyphs using the currently active fill and/or stroke.
//
// This is a very low-level method, which gives you full control over how to
// place the glyphs that make up the text. This means that you must have
// your own text processing logic for dealing with bidirectional text, font
// fallback, text layouting, etc.
func (s *Surface) DrawGlyphs(start Point, glyphs []Glyph, font *Font, text string, fontSize float32, outlined bool) {
	contextColor := s.contextColor()
	if outlined {
		s.outlineGlyphs(glyphs, contextColor, start, font, fontSize)
		return
	}
	if err := checkGlyphs(glyphs, text); err != nil {
		s.sc.setError(err)
		return
	}
	switch {
	case s.fill != nil && s.stroke != nil:
		if s.hasComplexFillOrStroke() {
			// We need to draw fill and stroke separately. In order to avoid
			// embedding text twice, we fill the text as normal text and then
			// stroke it as a path.
			s.bd.getMut().drawGlyphs(start, s.sc, s.cc, s.fill, nil, contextColor, glyphs, font, text, fontSize)
			s.outlineGlyphs(glyphs, contextColor, start, font, fontSize)
		} else {
			s.bd.getMut().drawGlyphs(start, s.sc, s.cc, s.fill, s.stroke, contextColor, glyphs, font, text, fontSize)
		}
	case s.stroke != nil:
		s.bd.getMut().drawGlyphs(start, s.sc, s.cc, nil, s.stroke, contextColor, glyphs, font, text, fontSize)
	case s.fill != nil:
		s.bd.getMut().drawGlyphs(start, s.sc, s.cc, s.fill, nil, contextColor, glyphs, font, text, fontSize)
	default:
		// Draw with black by default.
		fill := DefaultFill()
		s.bd.getMut().drawGlyphs(start, s.sc, s.cc, &fill, nil, contextColor, glyphs, font, text, fontSize)
	}
}

// checkGlyphs checks the glyphs of DrawGlyphs where krilla panics. It is not
// in krilla.
func checkGlyphs(glyphs []Glyph, text string) error {
	boundary := func(i int) bool {
		return i == len(text) || (i < len(text) && !isUTF8Continuation(text[i]))
	}
	for i := range glyphs {
		g := &glyphs[i]
		r := g.TextRange
		if r.Start < 0 || r.Start > r.End || r.End > len(text) || !boundary(r.Start) || !boundary(r.End) {
			return fmt.Errorf("%w: the range %d to %d of the glyph %d is not a range of the text of %d bytes", okage.ErrPDFInvalidGlyph, r.Start, r.End, g.GlyphID, len(text))
		}
		if g.GlyphID > 0xFFFF {
			return fmt.Errorf("%w: the glyph ID %d is above 65535", okage.ErrPDFInvalidGlyph, g.GlyphID)
		}
	}
	return nil
}

// isUTF8Continuation reports whether b continues a character of UTF-8, as
// str::is_char_boundary of Rust tests it.
func isUTF8Continuation(b byte) bool {
	return b&0xC0 == 0x80
}

// DrawText is Surface::draw_text in krilla src/surface.rs. typesetting
// shapes the text (D52, D62).
//
// Draw some text using the currently active fill and/or stroke.
//
// This is a high-level method which allows you to just provide some text,
// which will then be rendered into a single line. However, this approach
// has restrictions:
//
//   - It will not perform BIDI resolution and only supports a single
//     script, meaning that you must ensure that your text does not contain
//     multiple scripts.
//   - It will only use the single font you provided to draw the text, no
//     font fallback will be performed.
//
// If you need more advanced control over how your text looks, you can use
// the method DrawGlyphs.
func (s *Surface) DrawText(start Point, font *Font, fontSize float32, text string, outlined bool, direction TextDirection) {
	glyphs := naiveShape(text, font, direction)

	s.DrawGlyphs(start, glyphs, font, text, fontSize, outlined)
}

// contextColor is Surface::context_color in krilla src/surface.rs.
func (s *Surface) contextColor() ColorRGB {
	if s.fill != nil {
		if c, ok := paintAsRGB(s.fill.Paint); ok {
			return c
		}
	}
	if s.stroke != nil {
		if c, ok := paintAsRGB(s.stroke.Paint); ok {
			return c
		}
	}
	return ColorRGB{}
}

// SetLocation is Surface::set_location in krilla src/surface.rs.
//
// Set the location that should be assumed for subsequent operations.
func (s *Surface) SetLocation(location Location) {
	s.sc.setLocation(location)
}

// ResetLocation is Surface::reset_location in krilla src/surface.rs.
//
// Reset the location that should be assumed for subsequent operations.
func (s *Surface) ResetLocation() {
	s.sc.resetLocation()
}

// CTM is Surface::ctm in krilla src/surface.rs.
//
// Return the current transformation matrix, relative to the outermost
// surface, usually the page.
//
// This also accounts for the transforms that the current content stream
// will be drawn with, for example inside of PushMask, PushOpacity or
// PushIsolated. It is useful for determining at which resolution content
// should be rasterized.
//
// Inside of a surface created with Surface.StreamBuilder, it is assumed that
// the stream will be drawn with the transformation matrix the builder was
// created at, which is not accurate if the stream is drawn somewhere else,
// for example as a pattern.
func (s *Surface) CTM() Transform {
	return s.bd.ctm()
}

// PushTransform is Surface::push_transform in krilla src/surface.rs.
//
// Concatenate a new transform to the current transformation matrix.
func (s *Surface) PushTransform(transform Transform) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionTransform{})
	s.bd.getMut().saveGraphicsState()
	s.bd.getMut().concatTransform(transform)
}

// PushBlendMode is Surface::push_blend_mode in krilla src/surface.rs.
//
// Push a new blend mode.
func (s *Surface) PushBlendMode(blendMode BlendMode) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionBlendMode{})
	s.bd.getMut().saveGraphicsState()
	s.bd.getMut().setBlendMode(blendMode.toPDF())
}

// PushClipPath is Surface::push_clip_path in krilla src/surface.rs.
//
// Push a new clip path.
func (s *Surface) PushClipPath(p *Path, clipRule FillRule) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionClipPath{})
	s.bd.getMut().pushClipPath(p, clipRule, s.sc)
}

// PushMask is Surface::push_mask in krilla src/surface.rs.
//
// Push a new mask.
func (s *Surface) PushMask(mask *Mask) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionMask{mask: mask})
	s.pushSubBuilder()
}

// PushOpacity is Surface::push_opacity in krilla src/surface.rs.
//
// Push a new opacity, meaning that each subsequent graphics object will be
// rendered with a base opacity.
//
// This stacks, meaning that if you do PushOpacity(0.5) twice, the resulting
// base opacity will be 0.25.
func (s *Surface) PushOpacity(opacity NormalizedF32) {
	s.pushInstructions = append(s.pushInstructions, pushInstructionOpacity{opacity: opacity})

	if opacity != NormalizedF32One {
		s.pushSubBuilder()
	}
}

// PushIsolated is Surface::push_isolated in krilla src/surface.rs.
//
// Push a new isolated layer.
func (s *Surface) PushIsolated() {
	s.pushInstructions = append(s.pushInstructions, pushInstructionIsolated{})
	s.pushSubBuilder()
}

// popSubBuilder is Surface::pop_sub_builder in krilla src/surface.rs.
func (s *Surface) popSubBuilder() *Stream {
	builder := s.bd.pop()
	return builder.finish(s.sc)
}

// pushSubBuilder is Surface::push_sub_builder in krilla src/surface.rs.
func (s *Surface) pushSubBuilder() {
	builder := newContentBuilder(path.IdentityTransform(), true, s.sc)
	s.bd.push(builder)
}

// Pop is Surface::pop in krilla src/surface.rs. krilla panics when no push is
// left. The port then records ErrPDFPushPop, which Document.Finish returns,
// and does nothing.
//
// Pop the last push instruction.
func (s *Surface) Pop() {
	n := len(s.pushInstructions)
	if n == 0 {
		s.sc.setError(fmt.Errorf("%w: a pop without a push", okage.ErrPDFPushPop))
		return
	}
	instruction := s.pushInstructions[n-1]
	s.pushInstructions = s.pushInstructions[:n-1]

	switch instruction := instruction.(type) {
	case pushInstructionTransform:
		s.bd.getMut().restoreGraphicsState()
	case pushInstructionOpacity:
		if instruction.opacity != NormalizedF32One {
			stream := s.popSubBuilder()
			s.bd.getMut().drawOpacified(s.sc, s.cc, instruction.opacity, stream)
		}
	case pushInstructionClipPath:
		s.bd.getMut().popClipPath()
	case pushInstructionBlendMode:
		s.bd.getMut().restoreGraphicsState()
	case pushInstructionMask:
		stream := s.popSubBuilder()
		s.bd.getMut().drawMasked(s.sc, s.cc, instruction.mask, stream)
	case pushInstructionIsolated:
		stream := s.popSubBuilder()
		s.bd.getMut().drawIsolated(s.sc, s.cc, stream)
	case okagePushInstruction:
		// okage: the pushes that are not in krilla pop themselves (D68).
		instruction.pop(s)
	}
}

// DrawImage is Surface::draw_image in krilla src/surface.rs. It draws the
// image into the rectangle from 0, 0 to the size. krilla cannot take a
// missing image. The port records ErrPDFImage for a nil image, which
// Document.Finish returns, and draws nothing.
//
// Draw a new bitmap image.
func (s *Surface) DrawImage(img *Image, size Size) {
	if img == nil {
		s.sc.setError(fmt.Errorf("%w: the image is nil", okage.ErrPDFImage))
		return
	}
	s.bd.getMut().drawImage(img, size, s.sc, s.cc)
}

// DrawGraphic is Surface::draw_graphic in krilla src/surface.rs.
//
// Draw a new graphic.
//
// Drawing the same graphic multiple times is very cheap in terms of file
// size.
func (s *Surface) DrawGraphic(graphic Graphic) {
	s.bd.getMut().drawXObject(s.sc, s.cc, graphic.xObject, newExtGState())
}

// drawShading is Surface::draw_shading in krilla src/surface.rs.
func (s *Surface) drawShading(shading shadingFunction) {
	s.bd.getMut().drawShading(shading, s.sc, s.cc)
}

// Finish is Surface::finish and the impl Drop for Surface in krilla
// src/surface.rs. It gives the stream of the surface to its page or its
// StreamBuilder. A second call does nothing.
//
// krilla panics when a push has no pop. The port then records ErrPDFPushPop,
// which Document.Finish returns, and pops the pushes that are left, so that
// the content stream stays valid.
func (s *Surface) Finish() {
	if s.finished {
		return
	}
	s.finished = true

	if n := len(s.pushInstructions); n != 0 {
		s.sc.setError(fmt.Errorf("%w: %d pushes without a pop", okage.ErrPDFPushPop, n))
		for len(s.pushInstructions) != 0 {
			s.Pop()
		}
	}

	rootBuilder := s.bd.rootBuilder
	s.bd.rootBuilder = nil
	s.finishFn(rootBuilder.finish(s.sc))
}

// drawOpacifiedStream is Surface::draw_opacified_stream in krilla
// src/surface.rs.
func (s *Surface) drawOpacifiedStream(opacity NormalizedF32, stream *Stream) {
	s.bd.getMut().drawOpacified(s.sc, s.cc, opacity, stream)
}

// shadingMaskStream is the part of Mask::new_from_shading in krilla
// src/graphics/mask.rs that draws the shading of the opacities of a gradient
// onto the surface of a new stream.
func shadingMaskStream(sc *serializeContext, cc *chunkContainer, shadingTransform Transform, shadingFunction shadingFunction) *Stream {
	builder := newStreamBuilder(sc, cc, path.IdentityTransform())
	surface := builder.Surface()
	surface.PushTransform(shadingTransform)
	surface.drawShading(shadingFunction)
	surface.Pop()
	surface.Finish()
	return builder.Finish()
}

// opacifiedPatternStream is the part of TilingPattern::new in krilla
// src/graphics/tiling_pattern.rs that draws the stream of a pattern with an
// opacity onto the surface of a new stream.
func opacifiedPatternStream(sc *serializeContext, cc *chunkContainer, baseOpacity NormalizedF32, stream *Stream) *Stream {
	builder := newStreamBuilder(sc, cc, path.IdentityTransform())
	surface := builder.Surface()
	surface.drawOpacifiedStream(baseOpacity, stream)
	surface.Finish()
	return builder.Finish()
}

// hasComplexFillOrStroke is Surface::has_complex_fill_or_stroke in krilla
// src/surface.rs.
func (s *Surface) hasComplexFillOrStroke() bool {
	// Things can go wrong and yield inconsistent results in different PDF
	// viewers for some fill/stroke combinations. So in some cases, we need
	// to fill and stroke separately.
	// 1) We have an opacity on the stroke, because they render
	// inconsistently across different viewers.
	// 2) We have a linear gradient with opacities in at least one stop,
	// because then we need to invoke a soft mask, in which case the
	// fill/stroke would both be affected by this.

	anyStopOpacity := func(stops []Stop) bool {
		for _, s := range stops {
			if s.Opacity != NormalizedF32One {
				return true
			}
		}
		return false
	}

	checkPaint := func(paint Paint) bool {
		switch p := paint.(type) {
		case LinearGradient:
			return anyStopOpacity(p.Stops)
		case RadialGradient:
			return anyStopOpacity(p.Stops)
		case SweepGradient:
			return anyStopOpacity(p.Stops)
		case MeshGradient:
			// okage: a mesh with opacities needs a soft mask too (D68).
			return p.hasOpacity()
		default:
			// A color or a pattern.
			return false
		}
	}

	complexStroke := s.stroke != nil && (s.stroke.Opacity != NormalizedF32One || checkPaint(s.stroke.Paint))

	complexFill := s.fill != nil && checkPaint(s.fill.Paint)

	return complexFill || complexStroke
}

// builders is Builders in krilla src/surface.rs.
//
// Holds the different content streams we are currently building. In the
// usual case, this only contains the current page stream as the root
// builder, but the sub builders will be used if we are for example creating
// a mask/pattern, or an XObject.
type builders struct {
	rootBuilder *contentBuilder
	subBuilders []*contentBuilder
	// rootBaseTransform is the transform that the root builder will be drawn
	// with, relative to the outermost surface.
	rootBaseTransform Transform
	// subBaseTransforms holds the transforms that each sub builder will be
	// drawn with, relative to the outermost surface.
	subBaseTransforms []Transform
}

// newBuilders is Builders::new in krilla src/surface.rs.
func newBuilders(rootBuilder *contentBuilder, rootBaseTransform Transform) builders {
	return builders{
		rootBuilder:       rootBuilder,
		rootBaseTransform: rootBaseTransform,
	}
}

// push is Builders::push in krilla src/surface.rs.
//
// Push a new sub builder, whose content will be drawn with the current
// transform.
func (b *builders) push(builder *contentBuilder) {
	b.subBaseTransforms = append(b.subBaseTransforms, b.ctm())
	b.subBuilders = append(b.subBuilders, builder)
}

// pop is Builders::pop in krilla src/surface.rs. The caller pops only after
// a push.
func (b *builders) pop() *contentBuilder {
	b.subBaseTransforms = b.subBaseTransforms[:len(b.subBaseTransforms)-1]
	builder := b.subBuilders[len(b.subBuilders)-1]
	b.subBuilders = b.subBuilders[:len(b.subBuilders)-1]
	return builder
}

// ctm is Builders::ctm in krilla src/surface.rs.
func (b *builders) ctm() Transform {
	base := b.rootBaseTransform
	if n := len(b.subBaseTransforms); n != 0 {
		base = b.subBaseTransforms[n-1]
	}
	return base.PreConcat(b.get().curTransform())
}

// getMut is Builders::get_mut in krilla src/surface.rs.
func (b *builders) getMut() *contentBuilder {
	if n := len(b.subBuilders); n != 0 {
		return b.subBuilders[n-1]
	}
	return b.rootBuilder
}

// get is Builders::get in krilla src/surface.rs.
func (b *builders) get() *contentBuilder {
	return b.getMut()
}

// pushInstruction is PushInstruction in krilla src/surface.rs. Each case of
// upstream is a type.
type pushInstruction interface {
	isPushInstruction()
}

// pushInstructionTransform is PushInstruction::Transform.
type pushInstructionTransform struct{}

// pushInstructionOpacity is PushInstruction::Opacity.
type pushInstructionOpacity struct {
	opacity NormalizedF32
}

// pushInstructionClipPath is PushInstruction::ClipPath.
type pushInstructionClipPath struct{}

// pushInstructionBlendMode is PushInstruction::BlendMode.
type pushInstructionBlendMode struct{}

// pushInstructionMask is PushInstruction::Mask.
type pushInstructionMask struct {
	mask *Mask
}

// pushInstructionIsolated is PushInstruction::Isolated.
type pushInstructionIsolated struct{}

func (pushInstructionTransform) isPushInstruction() {}
func (pushInstructionOpacity) isPushInstruction()   {}
func (pushInstructionClipPath) isPushInstruction()  {}
func (pushInstructionBlendMode) isPushInstruction() {}
func (pushInstructionMask) isPushInstruction()      {}
func (pushInstructionIsolated) isPushInstruction()  {}
