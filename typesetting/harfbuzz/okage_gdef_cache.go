package harfbuzz

import "github.com/xo/okage/typesetting/font/opentype/tables"

// okage: D83. This file is okage's own. HarfBuzz keeps a small cache of the
// glyph properties of the table GDEF (glyph_props_cache of the accelerator
// of GDEF). The shaper reads them for each glyph of each run, and most
// fonts hold the glyph classes in ranges, which a binary search reads.

// glyphProps returns the properties of g in the table GDEF of the face of
// f, as GDEF.GlyphProps does, from the cache of f when it holds them.
func (f *Font) glyphProps(g gID) tables.GlyphProps {
	if v, ok := f.glyphPropsCache.get(g); ok {
		return v
	}
	props := f.face.GDEF.GlyphProps(g)
	f.glyphPropsCache.set(g, props)
	return props
}
