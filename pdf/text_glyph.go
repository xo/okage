// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/path"
)

// This file ports krilla src/text/glyph/mod.rs.
//
// The trait Glyph of upstream lets a caller give glyphs of its own type.
// Its one implementation, KrillaGlyph, is the struct Glyph of the port, which
// DrawGlyphs takes.

// GlyphID is GlyphId in krilla src/text/glyph/mod.rs.
//
// A glyph ID.
type GlyphID uint32

// TextRange is the Range<usize> of a cluster in krilla: the start and the
// end of its bytes in the text.
type TextRange struct {
	Start int
	End   int
}

// Glyph is KrillaGlyph in krilla src/text/glyph/mod.rs. Its methods are the
// methods of the trait Glyph of upstream for KrillaGlyph.
//
// A glyph type that implements Glyph.
//
// XAdvance, XOffset, YOffset and YAdvance need to be normalized, i.e.
// divided by the units per em!
type Glyph struct {
	// GlyphID is the glyph ID of the glyph.
	GlyphID GlyphID
	// TextRange is the range in the original text that corresponds to the
	// cluster of the glyph.
	TextRange TextRange
	// XAdvance is the advance of the glyph.
	XAdvance float32
	// XOffset is the x offset of the glyph.
	XOffset float32
	// YOffset is the y offset of the glyph.
	YOffset float32
	// YAdvance is the y advance of the glyph.
	YAdvance float32
	// Location is the location of the glyph. If set, the surface sets it
	// with SetLocation before it processes the glyph. Zero is None.
	Location Location
}

// xAdvance is Glyph::x_advance for KrillaGlyph: the advance in the x
// direction of the glyph, at the given font size.
func (g *Glyph) xAdvance(size float32) float32 {
	return float32(g.XAdvance * size)
}

// xOffset is Glyph::x_offset for KrillaGlyph: the offset in the x direction
// of the glyph, at the given font size.
func (g *Glyph) xOffset(size float32) float32 {
	return float32(g.XOffset * size)
}

// yOffset is Glyph::y_offset for KrillaGlyph: the offset in the y direction
// of the glyph, at the given font size.
func (g *Glyph) yOffset(size float32) float32 {
	return float32(g.YOffset * size)
}

// yAdvance is Glyph::y_advance for KrillaGlyph: the advance in the y
// direction of the glyph, at the given font size.
func (g *Glyph) yAdvance(size float32) float32 {
	return float32(g.YAdvance * size)
}

// drawColorGlyph is draw_color_glyph in krilla src/text/glyph/mod.rs. It
// returns false where upstream returns None.
//
// Draw a color glyph to a surface.
func drawColorGlyph(font *Font, contextColor ColorRGB, glyph GlyphID, baseTransform Transform, surface *Surface) bool {
	surface.PushTransform(baseTransform)
	surface.PushTransform(path.TransformFromScale(1.0, -1.0))

	// Note: See the comment in krilla-svg in `render_svg_glyph` for why we
	// prefer COLR over SVG. For Noto Color Emoji specifically, using SVG
	// will be slow when rendering multiple glyphs.
	drawn := drawColrGlyph(font, contextColor, glyph, surface) ||
		drawSVGGlyph(font, contextColor, glyph, surface) ||
		drawBitmapGlyph(font, glyph, surface)

	surface.Pop()
	surface.Pop()

	return drawn
}

// shouldOutline is should_outline in krilla src/text/glyph/mod.rs. It
// reports whether the glyph goes into the CID font of its font.
func shouldOutline(font *Font, glyph GlyphID) bool {
	hasSVG := hasSVGData(font, glyph)
	hasColr := hasColrData(font, glyph)
	hasBitmap := hasBitmapData(font, glyph)
	return font.info.canBeCIDFont() && !hasSVG && !hasColr && !hasBitmap
}

// drawGlyph is draw_glyph in krilla src/text/glyph/mod.rs.
//
// Draw a color glyph or outline glyph to a surface.
func drawGlyph(font *Font, contextColor ColorRGB, glyph GlyphID, baseTransform Transform, surface *Surface) bool {
	return drawColorGlyph(font, contextColor, glyph, baseTransform, surface) ||
		drawOutlineGlyph(font, glyph, baseTransform, surface)
}

// pdfGlyph is PDFGlyph in krilla src/text/glyph/mod.rs. isType3 tells the
// case: PDFGlyph::Type3 with type3, or PDFGlyph::Cid with cid.
//
// A glyph that belongs either to a CID font or a Type 3 font.
type pdfGlyph struct {
	isType3 bool
	type3   uint8
	cid     uint16
}

// encodeInto is PDFGlyph::encode_into in krilla src/text/glyph/mod.rs.
//
// Encode the glyph into a content stream.
func (g pdfGlyph) encodeInto(b []byte) []byte {
	if g.isType3 {
		return append(b, g.type3)
	}
	return append(b, byte(g.cid>>8), byte(g.cid&0xff))
}
