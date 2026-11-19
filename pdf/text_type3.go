// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"strconv"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports krilla src/text/type3.rs.

// coloredGlyph is ColoredGlyph in krilla src/text/type3.rs.
type coloredGlyph struct {
	glyphID GlyphID
	// contextColor is the color of the glyph. Some formats like COLR and SVG
	// allow drawing the same glyph with a different context color, so we
	// need to store that as well.
	contextColor ColorRGB
}

// type3Font is Type3Font in krilla src/text/type3.rs.
type type3Font struct {
	f           *Font
	glyphs      []coloredGlyph
	widths      []float32
	cmapEntries map[uint8]cmapEntry
	glyphSet    map[coloredGlyph]bool
	index       int
}

// newType3Font is Type3Font::new in krilla src/text/type3.rs.
func newType3Font(font *Font, index int) *type3Font {
	return &type3Font{
		f:           font,
		cmapEntries: map[uint8]cmapEntry{},
		glyphSet:    map[coloredGlyph]bool{},
		index:       index,
	}
}

// unitsPerEm is Type3Font::unit_per_em in krilla src/text/type3.rs, and
// PdfFont::units_per_em.
//
// Unlike CID fonts, the units per em of type 3 fonts does not have to be
// 1000.
func (t *type3Font) unitsPerEm() float32 {
	return t.f.UnitsPerEm()
}

// isFull is Type3Font::is_full in krilla src/text/type3.rs.
func (t *type3Font) isFull() bool {
	return len(t.glyphs) == 256
}

// covers is Type3Font::covers in krilla src/text/type3.rs.
func (t *type3Font) covers(glyph coloredGlyph) bool {
	return t.glyphSet[glyph]
}

// getType3GID is Type3Font::get_gid in krilla src/text/type3.rs.
func (t *type3Font) getType3GID(glyph coloredGlyph) (uint8, bool) {
	for i, g := range t.glyphs {
		if g == glyph {
			if i > 0xFF {
				return 0, false
			}
			return uint8(i), true
		}
	}
	return 0, false
}

// getGID is PdfFont::get_gid for Type3Font in krilla src/text/mod.rs.
func (t *type3Font) getGID(glyph coloredGlyph) (pdfGlyph, bool) {
	gid, ok := t.getType3GID(glyph)
	return pdfGlyph{isType3: true, type3: gid}, ok
}

// addGlyph is Type3Font::add_glyph in krilla src/text/type3.rs. The caller
// adds a glyph only to a font that is not full, which krilla asserts.
func (t *type3Font) addGlyph(glyph coloredGlyph) uint8 {
	if pos, ok := t.getType3GID(glyph); ok {
		return pos
	}
	t.glyphSet[glyph] = true
	t.glyphs = append(t.glyphs, glyph)
	w, _ := t.f.advanceWidth(glyph.glyphID)
	t.widths = append(t.widths, w)
	return uint8(len(t.glyphs) - 1)
}

// getCodepoints is Type3Font::get_codepoints in krilla src/text/type3.rs,
// and PdfFont::get_codepoints.
func (t *type3Font) getCodepoints(g pdfGlyph) (string, bool) {
	if !g.isType3 {
		return "", false
	}
	e, ok := t.cmapEntries[g.type3]
	return e.text, ok
}

// setCodepoints is Type3Font::set_codepoints in krilla src/text/type3.rs,
// and PdfFont::set_codepoints.
func (t *type3Font) setCodepoints(g pdfGlyph, text string, location Location) {
	if !g.isType3 {
		return
	}
	t.cmapEntries[g.type3] = cmapEntry{text: text, location: location}
}

// font is Type3Font::font in krilla src/text/type3.rs, and PdfFont::font.
func (t *type3Font) font() *Font {
	return t.f
}

// forceFill is PdfFont::force_fill for Type3Font in krilla src/text/mod.rs.
func (*type3Font) forceFill() bool {
	return true
}

// identifier is Type3Font::identifier in krilla src/text/type3.rs.
func (t *type3Font) identifier() fontIdentifier {
	return type3Identifier(t.f, t.index)
}

// serialize is Type3Font::serialize in krilla src/text/type3.rs.
func (t *type3Font) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	streamChunk := sc.newChunk()

	rdBuilder := newResourceDictionaryBuilder()
	fontBBox, _ := path.RectFromXYWH(0.0, 0.0, 1.0, 1.0)

	glyphStreams := make([]pdfwriter.Ref, len(t.glyphs))
	for index, glyph := range t.glyphs {
		streamSurface := newStreamBuilder(sc, cc, path.IdentityTransform())
		surface := streamSurface.Surface()

		// In case this returns false, the surface is guaranteed to be empty.
		drawnColorGlyph := drawColorGlyph(t.f, glyph.contextColor, glyph.glyphID, path.IdentityTransform(), surface)

		if !drawnColorGlyph {
			// If this code path is reached, either we are dealing with a
			// glyph that has no outline (like .notdef), or it means we tried
			// to create a Type3 font from a font that does not have any
			// (valid) color/bitmap/svg table. Therefore, as a last resort we
			// draw the outline glyph as a black glyph. PDF does have the
			// concept of shape glyphs which take the color or where they are
			// drawn from. The problem with those is that they seemingly have
			// very bad viewer support. For example, filled glyphs with
			// gradients become broken in some viewers, and stroking does not
			// work at all consistently. Because of this, we don't bother
			// trying to implement it for now.
			//
			// If this is the case (i.e. no color glyph was drawn, either
			// because no table exists or an error occurred, the surface is
			// guaranteed to be empty. So we can just safely draw the outline
			// glyph instead without having to worry about the surface being
			// "dirty".
			if p, ok := glyphPath(t.f, glyph.glyphID); ok {
				fill := DefaultFill()
				surface.SetFill(&fill)
				surface.SetStroke(nil)
				surface.DrawPath(p)
			}
		}

		surface.Finish()
		stream := streamSurface.Finish()
		content := sc.newContent()

		// I considered writing into the stream directly instead of creating
		// an XObject and showing that, but it seems like many viewers don't
		// like that, and emojis look messed up. Using XObjects seems like
		// the best choice here.
		content.StartColorGlyph(t.widths[index])
		xObject := newXObject(stream, false, false, nil)
		if !xObject.isEmpty() {
			rectExpand(&fontBBox, xObject.bbox())
			xName := rdBuilder.registerResource(registerResourceable(sc, cc, xObject))
			content.XObject(pdfwriter.Name(xName))
		}

		contentStream := content.Finish()

		fontStream := newFilterStreamFromContentStream(contentStream.AsSlice(), sc.serializeSettings()).
			finish(sc.serializeSettings())

		streamRef := sc.newRef()
		s := streamChunk.Stream(streamRef, fontStream.encodedData())
		fontStream.writeFilters(s.Dict)
		s.Finish()

		glyphStreams[index] = streamRef
	}

	resourceDictionary := rdBuilder.finish()
	chunk := cc.nonStream.fonts

	var descriptorRef pdfwriter.Ref
	hasDescriptor := sc.serializeSettings().pdfVersion() >= PDF15
	if hasDescriptor {
		descriptorRef = sc.newRef()
	}
	cmapRef := sc.newRef()

	var flags pdfwriter.FontFlags
	if name, ok := t.f.postscriptName(); ok && containsSerif(name) {
		flags |= pdfwriter.FontFlagsSerif
	}
	if t.f.isMonospaced() {
		flags |= pdfwriter.FontFlagsFixedPitch
	}
	if t.f.italicAngle() != 0.0 {
		flags |= pdfwriter.FontFlagsItalic
	}
	flags |= pdfwriter.FontFlagsSymbolic
	flags |= pdfwriter.FontFlagsSmallCap

	italicAngle := t.f.italicAngle()
	ascender := fontBBox.Bottom()
	descender := fontBBox.Top()

	baseFont := type3BaseFontName(t.f)

	if hasDescriptor {
		// Write the font descriptor (contains metrics about the font).
		fontDescriptor := chunk.FontDescriptor(descriptorRef)
		fontDescriptor.
			Name(pdfwriter.Name(baseFont)).
			Flags(flags).
			BBox(rectToPDFRect(fontBBox)).
			ItalicAngle(italicAngle).
			Ascent(ascender).
			Descent(descender).
			// Adobe recommends these for tagged PDF for 1.5+ (descriptors
			// for Type3 fonts are only written for 1.5+ in the first place,
			// so no additional checks needed) so we write them as well.
			// Unfortunately we have no way of determining the actual family
			// name, so we just take the next best thing.
			Family(pdfwriter.Str(baseFont)).
			Stretch(fontStretch(t.f.stretch())).
			Weight(fontWeight(t.f.weight()))

		fontDescriptor.Finish()
	}

	type3 := chunk.Type3Font(rootRef)
	resourceDictionary.toPDFResources(type3, sc, cc.nonStream.resourceDictionaries)

	// See https://github.com/typst/typst/issues/5067 as to why we write
	// this.
	type3.Name(pdfwriter.Name(baseFont))
	type3.BBox(rectToPDFRect(fontBBox))
	type3.ToUnicode(cmapRef)
	type3.Matrix(transformToPDF(path.TransformFromScale(
		1.0/t.f.UnitsPerEm(),
		1.0/t.f.UnitsPerEm(),
	)))
	type3.FirstChar(0)
	type3.LastChar(uint8(len(t.glyphs) - 1))
	type3.Widths(t.widths...)
	if hasDescriptor {
		type3.FontDescriptor(descriptorRef)
	}

	charProcs := type3.CharProcs()
	for gid, ref := range glyphStreams {
		charProcs.Pair(pdfwriter.Name("g"+strconv.Itoa(gid)), ref)
	}
	charProcs.Finish()

	names := make([]pdfwriter.Name, len(t.glyphs))
	for gid := range names {
		names[gid] = pdfwriter.Name("g" + strconv.Itoa(gid))
	}

	type3.EncodingCustom().Differences().Consecutive(0, names...)

	type3.Finish()

	cmap := pdfwriter.NewUnicodeCmap[uint8](cmapName, systemInfo)
	for g := range t.glyphs {
		var entry *cmapEntry
		if e, ok := t.cmapEntries[uint8(g)]; ok {
			entry = &e
		}
		writeCmapEntry(t.f, entry, sc, cmap, uint8(g))
	}

	cmapBuf := cmap.Finish()
	cmapStream := newFilterStreamFromContentStream(cmapBuf.AsSlice(), sc.serializeSettings()).finish(sc.serializeSettings())
	cmapWriter := streamChunk.Cmap(cmapRef, cmapStream.encodedData())
	cmapStream.writeFilters(cmapWriter.Dict)
	cmapWriter.WritingMode(pdfwriter.WModeHorizontal)
	cmapWriter.Finish()
	cc.streams.fonts = append(cc.streams.fonts, streamChunk)
}

// fontStretch is the match of the stretch of a font descriptor in
// Type3Font::serialize in krilla src/text/type3.rs. A ratio that is not one
// of the constants of Stretch of skrifa falls back to normal.
func fontStretch(ratio float32) pdfwriter.FontStretch {
	switch ratio {
	case 0.5:
		return pdfwriter.FontStretchUltraCondensed
	case 0.625:
		return pdfwriter.FontStretchExtraCondensed
	case 0.75:
		return pdfwriter.FontStretchCondensed
	case 0.875:
		return pdfwriter.FontStretchSemiCondensed
	case 1.125:
		return pdfwriter.FontStretchSemiExpanded
	case 1.25:
		return pdfwriter.FontStretchExpanded
	case 1.5:
		return pdfwriter.FontStretchExtraExpanded
	case 2.0:
		return pdfwriter.FontStretchUltraExpanded
	}
	return pdfwriter.FontStretchNormal
}

// fontWeight is the match of the weight of a font descriptor in
// Type3Font::serialize in krilla src/text/type3.rs.
func fontWeight(weight float32) uint16 {
	switch w := wide.TruncI32(weight); {
	case w >= 150 && w < 250:
		return 200
	case w >= 250 && w < 350:
		return 300
	case w >= 350 && w < 450:
		return 400
	case w >= 450 && w < 550:
		return 500
	case w >= 550 && w < 650:
		return 600
	case w >= 650 && w < 750:
		return 700
	case w >= 750 && w < 850:
		return 800
	case w < 150:
		return 100
	}
	return 900
}

// type3FontMapper is Type3FontMapper in krilla src/text/type3.rs.
//
// A font can have multiple Type3 fonts, if more than 256 glyphs are used.
// We use this struct to keep track of them.
type type3FontMapper struct {
	font  *Font
	fonts []*type3Font
}

// newType3FontMapper is Type3FontMapper::new in krilla src/text/type3.rs.
func newType3FontMapper(font *Font) *type3FontMapper {
	return &type3FontMapper{font: font}
}

// idFromGlyph is Type3FontMapper::id_from_glyph in krilla
// src/text/type3.rs.
//
// Given a requested glyph coverage, find the corresponding font identifier
// of the font that contains it.
func (m *type3FontMapper) idFromGlyph(glyph coloredGlyph) (fontIdentifier, bool) {
	for _, f := range m.fonts {
		if f.covers(glyph) {
			return f.identifier(), true
		}
	}
	return fontIdentifier{}, false
}

// fontFromID is Type3FontMapper::font_from_id and font_mut_from_id in
// krilla src/text/type3.rs.
//
// Given a font identifier, return the corresponding Type3 font if it is
// part of that type 3 font mapper.
func (m *type3FontMapper) fontFromID(id fontIdentifier) (*type3Font, bool) {
	for _, f := range m.fonts {
		if f.identifier() == id {
			return f, true
		}
	}
	return nil, false
}

// addGlyph is Type3FontMapper::add_glyph in krilla src/text/type3.rs.
func (m *type3FontMapper) addGlyph(glyph coloredGlyph) (fontIdentifier, uint8) {
	// If the glyph has already been added, return the font identifier of
	// the type 3 font as well as the Type3 gid in that font.
	if id, ok := m.idFromGlyph(glyph); ok {
		f, _ := m.fontFromID(id)
		gid, _ := f.getType3GID(glyph)
		return id, gid
	}

	if n := len(m.fonts); n != 0 && !m.fonts[n-1].isFull() {
		// Otherwise, insert it into the last Type3 font.
		last := m.fonts[n-1]
		return last.identifier(), last.addGlyph(glyph)
	}
	// If no Type3 font exists yet, or the last one is full, create a new
	// one.
	font := newType3Font(m.font, len(m.fonts))
	id := font.identifier()
	gid := font.addGlyph(glyph)
	m.fonts = append(m.fonts, font)
	return id, gid
}

// type3BaseFontName is base_font_name in krilla src/text/type3.rs.
func type3BaseFontName(font *Font) string {
	const restLen = 1 + 1 + len(identityH)
	postscriptName, ok := font.postscriptName()
	if !ok {
		postscriptName = "unknown"
	}
	const maxLen = 127 - restLen
	return postscriptName[:min(len(postscriptName), maxLen)]
}
