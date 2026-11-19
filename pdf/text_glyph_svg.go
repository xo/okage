// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports krilla src/text/glyph/svg.rs, with the type
// RenderSvgGlyphFn and the field render_svg_glyph_fn of SerializeSettings of
// src/serialize.rs, which become SVGGlyphFunc and WithSVGGlyphs.

// SVGGlyphFunc is RenderSvgGlyphFn in krilla src/serialize.rs. It draws the
// SVG document of a glyph onto a surface, in the units of the font, and
// returns false where upstream returns None. data is the document of the
// glyph in the table SVG, which can be compressed with gzip. width and
// height are the units per em of the font. pdfsvg gives RenderGlyph, which
// draws the document with the package svg.
type SVGGlyphFunc func(data []byte, contextColor ColorRGB, glyph GlyphID, width, height float32, s *Surface) bool

// WithSVGGlyphs is the field render_svg_glyph_fn of SerializeSettings in
// krilla src/serialize.rs. The default is nil, the default function of
// upstream, which draws no SVG glyph. The surface then draws the bitmap or
// the outline of such a glyph.
//
// A function that should be used to render SVG glyphs. It is recommended
// that you use the function of the package pdfsvg.
func WithSVGGlyphs(fn SVGGlyphFunc) Option {
	return func(s *serializeSettings) {
		s.renderSVGGlyphFn = fn
	}
}

// svgGlyphData is svg().glyph_data(glyph) of read-fonts: the document of
// the table SVG that holds the glyph. It returns false for a font without a
// valid table SVG, or a glyph that the table does not hold.
func svgGlyphData(font *Font, glyph GlyphID) ([]byte, bool) {
	font.svgOnce.Do(func() {
		if raw := font.table("SVG "); raw != nil {
			if t, _, err := tables.ParseSVG(raw); err == nil {
				font.svg = &t
			}
		}
	})
	if font.svg == nil || glyph > 0xFFFF {
		return nil, false
	}
	list := font.svg.SVGDocumentList
	// read-fonts searches the records with a binary search.
	records := list.DocumentRecords
	lo, hi := 0, len(records)
	for lo < hi {
		mid := lo + (hi-lo)/2
		r := records[mid]
		switch {
		case uint32(glyph) < uint32(r.StartGlyphID):
			hi = mid
		case uint32(glyph) > uint32(r.EndGlyphID):
			lo = mid + 1
		default:
			start := int(r.SvgDocOffset)
			end := start + int(r.SvgDocLength)
			if start < 0 || end > len(list.SVGRawData) || start > end {
				return nil, false
			}
			return list.SVGRawData[start:end], true
		}
	}
	return nil, false
}

// hasSVGData is has_svg_data in krilla src/text/glyph/svg.rs.
func hasSVGData(font *Font, glyph GlyphID) bool {
	_, ok := svgGlyphData(font, glyph)
	return ok
}

// drawSVGGlyph is draw_glyph in krilla src/text/glyph/svg.rs. It returns
// false where upstream returns None, and when the document has no function
// of WithSVGGlyphs, as the default function of upstream does.
//
// Draw an SVG-based glyph on a surface.
func drawSVGGlyph(font *Font, contextColor ColorRGB, glyph GlyphID, surface *Surface) bool {
	data, ok := svgGlyphData(font, glyph)
	if !ok {
		return false
	}

	upem := font.UnitsPerEm()

	oldFill := surface.GetFill()
	oldStroke := surface.GetStroke()

	fn := surface.sc.serializeSettings().renderSVGGlyphFn
	result := false
	if fn != nil {
		result = fn(data, contextColor, glyph, upem, upem, surface)
	}

	surface.SetFill(oldFill)
	surface.SetStroke(oldStroke)

	return result
}
