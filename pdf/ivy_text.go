// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/internal/pdfwriter"
)

// This file ports no file of krilla. It holds the invisible text of D106:
// a run of glyphs that a reader can search, select and copy, and that paints
// nothing. The device of ivy draws the outlines of a text as paths, and
// draws its glyphs again as invisible text over the paths.

// DrawInvisibleGlyphs draws the glyphs with their text as invisible text,
// with the text rendering mode 3 (D106). The arguments are those of
// DrawGlyphs. The run embeds the font and adds its text to the ToUnicode
// map, as a visible run does, so that a reader can search, select and copy
// the text. The fill and the stroke of the surface do not apply, and the
// run adds no graphics state of its own.
//
// As DrawGlyphs does, DrawInvisibleGlyphs records the error of CheckGlyphs
// for a glyph that it cannot draw, which Document.Finish returns, and then
// draws nothing.
func (s *Surface) DrawInvisibleGlyphs(start Point, glyphs []Glyph, font *Font, text string, fontSize float32) {
	if err := CheckGlyphs(glyphs, text); err != nil {
		s.sc.setError(err)
		return
	}
	// The color of the text does not show. The context color of the fill
	// keeps the font of a visible run of the same font, and an image font
	// takes black, as in DrawGlyphs (D89).
	contextColor := s.contextColor()
	if font.img != nil {
		contextColor = ColorRGB{}
	}
	s.bd.getMut().drawInvisibleGlyphs(start, s.sc, s.cc, contextColor, glyphs, font, text, fontSize)
}

// drawInvisibleGlyphs writes the glyphs as a text object in the text
// rendering mode 3, with no fill and no stroke properties (D106). It is
// drawGlyphs without the paint.
func (b *contentBuilder) drawInvisibleGlyphs(
	start Point,
	sc *serializeContext,
	cc *chunkContainer,
	contextColor ColorRGB,
	glyphs []Glyph,
	font *Font,
	text string,
	fontSize float32,
) {
	x, y := start.X, start.Y
	b.graphicsStates.saveState()
	// The text paints nothing, but the bbox of an XObject holds it, so that
	// a reader that clips its text to the bbox keeps it.
	if b.bboxImportant && len(glyphs) != 0 {
		b.expandBBox(getGlyphsBBox(glyphs, x, y, fontSize, font))
	}
	b.fillStrokeGlyphRun(x, y, sc, cc, pdfwriter.TextRenderingModeInvisible,
		func(*contentBuilder, *serializeContext, *chunkContainer) {},
		glyphs, font, contextColor, text, fontSize)
	b.graphicsStates.restoreState()
}
