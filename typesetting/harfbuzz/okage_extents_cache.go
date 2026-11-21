package harfbuzz

import (
	"math"

	"github.com/xo/okage/typesetting/font"
)

// okage: D87. This file is okage's own. The shaping package asks for the
// extents of each glyph that it shapes. GlyphExtents scales each of the
// four values with a multiply, a divide and a rounding. extentsCache keeps
// the results for the glyphs of a Font.

// extentsCacheSize is the number of items of an extentsCache. A text in
// Chinese uses a few thousand glyphs.
const extentsCacheSize = 4096

// extentsCache keeps the extents of glyphs after the scale of a Font. The
// item of a glyph is at the index of the low 12 bits of the glyph. An item
// holds the bits of the extents that the face gives, and the extents after
// the scale. It serves a glyph only when the face gives the same bits for
// the glyph, and the scale gives the same result for the same bits. So an
// item gives the right extents for each glyph, also when two glyphs share
// an item, and when the face changes its variations.
//
// The result of the scale also depends on the scales, the slant and the
// embolden of the Font. The cache keeps the values of its items, and it
// empties itself when one of them changes. An item is empty when its gen is
// not the gen of the cache.
type extentsCache struct {
	xScale, yScale          int32
	slantBits, emboldenBits uint32
	gen                     uint32

	items [extentsCacheSize]scaledExtents
}

// scaledExtents is an item of an extentsCache.
type scaledExtents struct {
	gen uint32
	raw [4]uint32
	out GlyphExtents
}

// sync empties the cache when the scales, the slant or the embolden of f
// are not the values of its items.
func (ec *extentsCache) sync(f *Font) {
	slant, embolden := math.Float32bits(f.slant), math.Float32bits(f.xEmbolden)
	if ec.gen != 0 && ec.xScale == f.XScale && ec.yScale == f.YScale &&
		ec.slantBits == slant && ec.emboldenBits == embolden {
		return
	}
	ec.xScale, ec.yScale, ec.slantBits, ec.emboldenBits = f.XScale, f.YScale, slant, embolden
	ec.gen++
	if ec.gen == 0 {
		// The gen went round, so an old item can hold the new gen.
		clear(ec.items[:])
		ec.gen = 1
	}
}

// extentsBits returns the bits of the values of ext.
func extentsBits(ext *font.GlyphExtents) [4]uint32 {
	return [4]uint32{
		math.Float32bits(ext.XBearing), math.Float32bits(ext.YBearing),
		math.Float32bits(ext.Width), math.Float32bits(ext.Height),
	}
}

// item returns the item of glyph, and whether it holds the extents with
// the bits raw.
func (ec *extentsCache) item(glyph GID, raw [4]uint32) (*scaledExtents, bool) {
	it := &ec.items[glyph&(extentsCacheSize-1)]
	return it, it.gen == ec.gen && it.raw == raw
}
