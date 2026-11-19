// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/internal/siphasher"
)

// This file ports krilla src/text/mod.rs, with the methods of
// SerializeContext of src/serialize.rs that keep the fonts of a document:
// register_font_container, register_font_identifier and serialize_fonts.
//
// krilla has extensive support for OpenType fonts. It supports CFF-based as
// well as TTF-based OpenType fonts. In addition to that, krilla also
// supports all major tables used in color fonts, including the SVG, COLR,
// sbix and CBDT/EBDT (only PNG) tables.
//
// You do not need to take care of choosing the right table for drawing
// glyphs: All you need to do is to provide the Font object with an
// appropriate index.

// pdfUnitsPerEm is PDF_UNITS_PER_EM in krilla src/text/mod.rs.
const pdfUnitsPerEm float32 = 1000.0

// fontIdentifierKind is the case of a fontIdentifier.
type fontIdentifierKind uint8

// The cases of FontIdentifier in krilla src/text/mod.rs. The case Standard
// is for the standard fonts of the forms, which the base port leaves out.
const (
	fontIdentifierCID fontIdentifierKind = iota
	fontIdentifierType3
)

// fontIdentifier is FontIdentifier in krilla src/text/mod.rs, with
// CIDIdentifier and Type3Identifier. The font is its hash, so that the
// identifier is comparable. type3 is the Type3ID of a Type 3 font.
//
// A font identifier for a PDF font.
type fontIdentifier struct {
	kind  fontIdentifierKind
	font  siphasher.Hash128
	type3 int
}

// typeID is the name of fontIdentifier for sipHash.
func (fontIdentifier) typeID() string { return "FontIdentifier" }

// hash is the derived Hash of FontIdentifier in krilla src/text/mod.rs.
func (i fontIdentifier) hash(h *hasher) {
	h.usize(int(i.kind))
	h.u128(i.font)
	if i.kind == fontIdentifierType3 {
		h.usize(i.type3)
	}
}

// cidIdentifier is the FontIdentifier::Cid of a font.
func cidIdentifier(f *Font) fontIdentifier {
	return fontIdentifier{kind: fontIdentifierCID, font: f.hash}
}

// type3Identifier is the FontIdentifier::Type3 of a Type 3 font of a font.
func type3Identifier(f *Font, index int) fontIdentifier {
	return fontIdentifier{kind: fontIdentifierType3, font: f.hash, type3: index}
}

// fontGlyph is the pair (FontIdentifier, PDFGlyph) of the caches of
// FontContainer in krilla src/text/mod.rs.
type fontGlyph struct {
	id    fontIdentifier
	glyph pdfGlyph
}

// fontContainer is FontContainer in krilla src/text/mod.rs.
//
// A container that holds all PDF fonts belonging to an OTF font.
type fontContainer struct {
	font        *Font
	type3Mapper *type3FontMapper
	cidFont     *cidFont
	cidCache    map[uint32]fontGlyph
	type3Cache  map[coloredGlyph]fontGlyph
}

// newFontContainer is FontContainer::new in krilla src/text/mod.rs.
func newFontContainer(font *Font) *fontContainer {
	return &fontContainer{
		font:        font,
		type3Mapper: newType3FontMapper(font),
		cidFont:     newCIDFont(font),
		cidCache:    map[uint32]fontGlyph{},
		type3Cache:  map[coloredGlyph]fontGlyph{},
	}
}

// fontIdentifier is FontContainer::font_identifier in krilla
// src/text/mod.rs.
func (c *fontContainer) fontIdentifier(glyph coloredGlyph) (fontIdentifier, bool) {
	if e, ok := c.cidCache[uint32(glyph.glyphID)]; ok {
		return e.id, true
	}
	if e, ok := c.type3Cache[glyph]; ok {
		return e.id, true
	}
	return fontIdentifier{}, false
}

// getFromIdentifier is FontContainer::get_from_identifier and
// get_from_identifier_mut in krilla src/text/mod.rs.
func (c *fontContainer) getFromIdentifier(id fontIdentifier) (pdfFont, bool) {
	if c.cidFont.identifier() == id {
		return c.cidFont, true
	}
	// If the identifier doesn't match either of CID or Type3, this will
	// return false.
	if t3, ok := c.type3Mapper.fontFromID(id); ok {
		return t3, true
	}
	return nil, false
}

// addGlyph is FontContainer::add_glyph in krilla src/text/mod.rs.
func (c *fontContainer) addGlyph(glyph coloredGlyph) fontGlyph {
	if e, ok := c.cidCache[uint32(glyph.glyphID)]; ok {
		// We already know whether this glyph uses a CID or Type3 glyph.
		return e
	}
	if e, ok := c.type3Cache[glyph]; ok {
		return e
	}
	if shouldOutline(c.font, glyph.glyphID) {
		cid := c.cidFont.addGlyph(glyph.glyphID)
		res := fontGlyph{id: c.cidFont.identifier(), glyph: pdfGlyph{cid: cid}}
		c.cidCache[uint32(glyph.glyphID)] = res
		return res
	}
	id, gid := c.type3Mapper.addGlyph(glyph)
	res := fontGlyph{id: id, glyph: pdfGlyph{isType3: true, type3: gid}}
	c.type3Cache[glyph] = res
	return res
}

// pdfFont is the trait PdfFont in krilla src/text/mod.rs, which cidFont and
// type3Font implement. krilla panics when a glyph of the other kind of font
// reaches a method. The two methods with a glyph then return false or do
// nothing, which no caller of the port can reach.
type pdfFont interface {
	unitsPerEm() float32
	font() *Font
	getCodepoints(g pdfGlyph) (string, bool)
	setCodepoints(g pdfGlyph, text string, location Location)
	getGID(glyph coloredGlyph) (pdfGlyph, bool)
	forceFill() bool
}

// registerFontContainer is SerializeContext::register_font_container in
// krilla src/serialize.rs. The containers keep the order in which the
// document uses their fonts, as the IndexMap of upstream does.
func (sc *serializeContext) registerFontContainer(font *Font) *fontContainer {
	if c, ok := sc.globalObjects.fontIndex[font.hash]; ok {
		return c
	}
	if sc.globalObjects.fontIndex == nil {
		sc.globalObjects.fontIndex = map[siphasher.Hash128]*fontContainer{}
	}
	c := newFontContainer(font)
	sc.globalObjects.fontIndex[font.hash] = c
	sc.globalObjects.fontMap = append(sc.globalObjects.fontMap, c)
	return c
}

// registerFontIdentifier is SerializeContext::register_font_identifier in
// krilla src/serialize.rs. It gives the reference of the font object of a
// PDF font, which it allocates on the first call.
func (sc *serializeContext) registerFontIdentifier(f fontIdentifier) fontResource {
	hash := sipHash(f)
	if ref, ok := sc.cachedMappings[hash]; ok {
		return fontResource{ref}
	}
	rootRef := sc.newRef()
	sc.cachedMappings[hash] = rootRef
	return fontResource{rootRef}
}

// serializeFonts is SerializeContext::serialize_fonts in krilla
// src/serialize.rs. The standard fonts are for the forms, which the base
// port leaves out (D62).
func (sc *serializeContext) serializeFonts(cc *chunkContainer) error {
	fonts := sc.globalObjects.fontMap
	sc.globalObjects.fontMap = nil
	sc.globalObjects.fontIndex = nil
	for _, c := range fonts {
		for _, t3 := range c.type3Mapper.fonts {
			f := sc.registerFontIdentifier(t3.identifier())
			t3.serialize(sc, cc, f.getRef())
		}

		if !c.cidFont.isEmpty() {
			f := sc.registerFontIdentifier(c.cidFont.identifier())
			if err := c.cidFont.serialize(sc, cc, f.getRef()); err != nil {
				return err
			}
		}
	}
	return nil
}
