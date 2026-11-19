// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfsvg

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/svg"
	ot "github.com/xo/okage/typesetting/font/opentype"
)

// This file ports krilla-svg src/text.rs.

// renderText is render in krilla-svg src/text.rs. It takes the suffix of
// its module, because other modules have a function of the same name (D10).
//
// Render a text into a surface.
func renderText(text *svg.Text, surface *pdf.Surface, pc *processContext) {
	for _, span := range text.Layouted() {
		if !span.Visible {
			continue
		}

		if span.Overline != nil {
			renderPath(span.Overline, surface, pc)
		}

		if span.Underline != nil {
			renderPath(span.Underline, surface, pc)
		}

		for i := range span.PositionedGlyphs {
			glyph := &span.PositionedGlyphs[i]
			// Ignore glyph if font can't be fetched.
			font := pc.fonts.retrieve(&span, glyph.Font)
			if font == nil {
				continue
			}

			upem := font.UnitsPerEm()

			// The text transform contains the scale transform
			// `font_size / upem`, we need to invert that so we only get the
			// raw transform to account for the glyph position, and the font
			// size is being taken care of by krilla.
			transform := glyph.Transform().PreConcat(path.TransformFromScale(
				upem/span.FontSize.Get(),
				upem/span.FontSize.Get(),
			))

			inverted, ok := transform.Invert()
			if !ok {
				continue
			}

			// We need to apply the inverse transform to fill/stroke because
			// we don't want the paint to be affected by the transform
			// applied to the glyph. See docs of `convertPaint`.
			var fill *pdf.Fill
			if span.Fill != nil {
				fill = convertFill(span.Fill, surface, pc, inverted)
			}

			var stroke *pdf.Stroke
			if span.Stroke != nil {
				stroke = convertStroke(span.Stroke, surface, pc, inverted)
			}

			drawOp := func(s *pdf.Surface, fill *pdf.Fill, stroke *pdf.Stroke, font *pdf.Font, embedText bool) {
				s.SetFill(fill)
				s.SetStroke(stroke)

				s.DrawGlyphs(
					path.PointFromXY(0.0, 0.0),
					[]pdf.Glyph{{
						GlyphID: pdf.GlyphID(glyph.ID),
						// Don't care about those, since we render only one
						// glyph.
						XAdvance:  0.0,
						XOffset:   0.0,
						YOffset:   0.0,
						YAdvance:  0.0,
						TextRange: pdf.TextRange{Start: 0, End: len(glyph.Text)},
					}},
					font,
					glyph.Text,
					span.FontSize.Get(),
					!embedText,
				)
			}

			surface.PushTransform(transform)

			switch {
			case fill == nil && stroke == nil:
				// Emulate invisible glyph by drawing it with an opacity of
				// zero.
				drawOp(
					surface,
					&pdf.Fill{
						Paint:   pdf.NewColorRGB(0, 0, 0),
						Opacity: pdf.NormalizedF32Zero,
						Rule:    pdf.FillRuleNonZero,
					},
					nil,
					font,
					pc.svgSettings.embedText,
				)
			case span.PaintOrder == svg.PaintOrderFillAndStroke || fill == nil || stroke == nil:
				drawOp(surface, fill, stroke, font, pc.svgSettings.embedText)
			default:
				// Paint order stroke and fill, and we have BOTH, a fill and
				// stroke.

				// We always draw the text outlined in this case, so that
				// text won't be embedded twice.
				drawOp(surface, nil, stroke, font, false)

				drawOp(surface, fill, nil, font, pc.svgSettings.embedText)
			}

			surface.Pop()
		}

		if span.LineThrough != nil {
			renderPath(span.LineThrough, surface, pc)
		}
	}
}

// fonts is Fonts in krilla-svg src/text.rs. Upstream holds a mutable
// borrow of the font database, because fontdb shares the data of a face
// through it. The database of svg does not need it.
//
// Manages the krilla fonts used by an SVG.
type fonts struct {
	db            *svg.FontDatabase
	fonts         map[string]*pdf.Font
	supportedAxes map[svg.FontID][][4]byte
}

// newFonts is Fonts::new in krilla-svg src/text.rs.
func newFonts(db *svg.FontDatabase) *fonts {
	return &fonts{
		db:            db,
		fonts:         map[string]*pdf.Font{},
		supportedAxes: map[svg.FontID][][4]byte{},
	}
}

// fontInstanceKey is the key of the type FontInstance of upstream in the
// map of the fonts: the ID of a face and its variations. It writes the bits
// of each value, as the derived Hash of FontVariation does.
//
// Identifies a font at specific variation coordinates.
func fontInstanceKey(id svg.FontID, variations []svg.FontVariation) string {
	b := binary.LittleEndian.AppendUint32(nil, uint32(id))
	for _, v := range variations {
		b = append(b, v.Tag[:]...)
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v.Value))
	}
	return string(b)
}

// retrieve is Fonts::retrieve in krilla-svg src/text.rs. It returns nil
// where upstream returns None.
//
// Retrieves the font identified by `id` from the cache or loads it.
func (f *fonts) retrieve(span *svg.Span, id svg.FontID) *pdf.Font {
	variations := f.resolveVariations(span, id)
	key := fontInstanceKey(id, variations)
	if font, ok := f.fonts[key]; ok {
		return font
	}
	var font *pdf.Font
	if fontData, index, ok := f.db.FaceData(id); ok {
		coords := make([]pdf.VariationCoord, len(variations))
		for i, v := range variations {
			coords[i] = pdf.VariationCoord{Tag: pdf.Tag(v.Tag), Value: v.Value}
		}
		if ft, err := pdf.NewFont(fontData, int(index), pdf.WithVariations(coords...)); err == nil {
			font = ft
		}
	}

	f.fonts[key] = font
	return font
}

// resolveVariations is Fonts::resolve_variations in krilla-svg
// src/text.rs.
//
// Resolves which variations are applicable for the given font (filtering
// out undefined axis values and taking into account optical sizing).
func (f *fonts) resolveVariations(span *svg.Span, id svg.FontID) []svg.FontVariation {
	supportedAxes, ok := f.supportedAxes[id]
	if !ok {
		if data, index, ok := f.db.FaceData(id); ok {
			supportedAxes = fontAxisTags(data, index)
		}
		f.supportedAxes[id] = supportedAxes
	}
	isSupported := func(tag [4]byte) bool {
		return slices.Contains(supportedAxes, tag)
	}

	var variations []svg.FontVariation
	for _, v := range span.Variations {
		if isSupported(v.Tag) {
			variations = append(variations, v)
		}
	}

	// Set the font size for optical sizing if desired, supported, and not
	// manually set.
	opsz := [4]byte{'o', 'p', 's', 'z'}
	if span.FontOpticalSizing == svg.FontOpticalSizingAuto &&
		isSupported(opsz) &&
		!slices.ContainsFunc(variations, func(v svg.FontVariation) bool { return v.Tag == opsz }) {
		variations = append(variations, svg.FontVariation{
			Tag:   opsz,
			Value: span.FontSize.Get(),
		})
	}

	return variations
}

// fontAxisTags is FontRef::from_index(data, index) with font.axes() of
// skrifa, which resolveVariations calls. It returns the tag of each axis of
// the table fvar, or none for a font that does not parse or has no valid
// table fvar.
func fontAxisTags(data []byte, index uint32) [][4]byte {
	lds, err := ot.NewLoaders(bytes.NewReader(data))
	if err != nil || uint64(index) >= uint64(len(lds)) {
		return nil
	}
	fvar, err := lds[index].RawTable(ot.MustNewTag("fvar"))
	if err != nil || len(fvar) < 16 {
		return nil
	}
	axesOffset := int(binary.BigEndian.Uint16(fvar[4:]))
	axisCount := int(binary.BigEndian.Uint16(fvar[8:]))
	axisSize := int(binary.BigEndian.Uint16(fvar[10:]))
	if axisSize < 20 || axesOffset+axisCount*axisSize > len(fvar) {
		return nil
	}
	tags := make([][4]byte, axisCount)
	for i := range tags {
		tags[i] = [4]byte(fvar[axesOffset+i*axisSize:])
	}
	return tags
}
