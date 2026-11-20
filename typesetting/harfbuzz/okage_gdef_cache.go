package harfbuzz

import "github.com/xo/okage/typesetting/font/opentype/tables"

// okage: D83. This file is okage's own. HarfBuzz keeps a small cache of the
// glyph properties of the table GDEF (glyph_props_cache of the accelerator
// of GDEF). The shaper reads them for each glyph of each run, and most
// fonts hold the glyph classes in ranges, which a binary search reads. A
// text in Chinese uses a few thousand glyphs, so the cache of okage holds
// 4096 glyphs, where the cache of HarfBuzz holds 256.

// glyphPropsCache maps a glyph to its glyph properties. Each item holds a
// glyph in its high 16 bits and its properties in its low 16 bits, at the
// index of the low 12 bits of the glyph.
type glyphPropsCache [4096]uint32

// clear empties the cache. Each item holds a glyph whose low 12 bits are
// not its index, so no glyph matches it.
func (gc *glyphPropsCache) clear() {
	for i := range gc {
		gc[i] = uint32(i^1) << 16
	}
}

// glyphProps returns the properties of g in the table GDEF of the face of
// f, as GDEF.GlyphProps does, from the cache of f when it holds them.
func (f *Font) glyphProps(g gID) tables.GlyphProps {
	e := &f.glyphPropsCache[int(g)&(len(f.glyphPropsCache)-1)]
	if gID(*e>>16) == g {
		return uint16(*e)
	}
	props := f.face.GDEF.GlyphProps(g)
	*e = uint32(g)<<16 | uint32(props)
	return props
}
