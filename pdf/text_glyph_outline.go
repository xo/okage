// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	tsfont "github.com/xo/okage/typesetting/font"

	"github.com/xo/okage/path"
)

// This file ports krilla src/text/glyph/outline.rs. skrifa draws an outline
// with the calls of the trait OutlinePen. typesetting gives the segments of
// the outline, which the package glyphoutline passes to the pen in the order
// of skrifa, for svg and pdf.

// glyphPath is glyph_path in krilla src/text/glyph/outline.rs. It draws the
// outline of a glyph at the location of the font, unhinted and unscaled. It
// returns false where upstream returns None: for a glyph without an outline,
// and for an outline without a segment.
func glyphPath(font *Font, glyph GlyphID) (*Path, bool) {
	// okage: a glyph of an image font has no outline (D89).
	if font.img != nil {
		return nil, false
	}
	b := newOutlineBuilder()
	font.drawSkrifaOutline(glyph, b)
	return b.finish()
}

// drawOutlineGlyph is draw_glyph in krilla src/text/glyph/outline.rs.
//
// Draw an outline-based glyph on a surface.
func drawOutlineGlyph(font *Font, glyph GlyphID, baseTransform Transform, surface *Surface) bool {
	p, ok := glyphPath(font, glyph)
	if !ok {
		return false
	}
	p, ok = p.Transform(baseTransform)
	if !ok {
		return false
	}
	surface.DrawPath(p)
	return true
}

// outlineBuilder is OutlineBuilder in krilla src/text/glyph/outline.rs.
//
// A wrapper struct for implementing the OutlinePen trait.
type outlineBuilder struct {
	b *path.PathBuilder
}

// newOutlineBuilder is OutlineBuilder::new in krilla
// src/text/glyph/outline.rs.
func newOutlineBuilder() *outlineBuilder {
	return &outlineBuilder{b: path.NewPathBuilder()}
}

// finish is OutlineBuilder::finish in krilla src/text/glyph/outline.rs.
func (o *outlineBuilder) finish() (*Path, bool) {
	return o.b.Finish()
}

// MoveTo is OutlinePen::move_to for OutlineBuilder.
func (o *outlineBuilder) MoveTo(x, y float32) { o.b.MoveTo(x, y) }

// LineTo is OutlinePen::line_to for OutlineBuilder.
func (o *outlineBuilder) LineTo(x, y float32) { o.b.LineTo(x, y) }

// QuadTo is OutlinePen::quad_to for OutlineBuilder.
func (o *outlineBuilder) QuadTo(cx0, cy0, x, y float32) { o.b.QuadTo(cx0, cy0, x, y) }

// CurveTo is OutlinePen::curve_to for OutlineBuilder.
func (o *outlineBuilder) CurveTo(cx0, cy0, cx1, cy1, x, y float32) {
	o.b.CubicTo(cx0, cy0, cx1, cy1, x, y)
}

// Close is OutlinePen::close for OutlineBuilder.
func (o *outlineBuilder) Close() { o.b.Close() }

// drawSkrifaOutline is OutlineGlyph::draw of skrifa with unhinted settings
// at the size unscaled and the location of the font, as krilla calls it. It
// draws the outline of the glyph into the builder, and returns false when
// the font has no outline for the glyph. glyphoutline puts the segments of
// typesetting in the order of skrifa.
func (f *Font) drawSkrifaOutline(glyph GlyphID, pen *outlineBuilder) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if uint64(glyph) > 0xFFFF {
		return false
	}
	return f.outline.Draw(f.face, tsfont.GID(glyph), pen)
}
