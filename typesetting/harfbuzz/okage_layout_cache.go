package harfbuzz

import "github.com/xo/okage/typesetting/font/opentype/tables"

// okage: D83. This file is okage's own. In HarfBuzz, each of the first 8
// subtables of a lookup can have a small cache of its own
// (external_cache_create), which keeps the coverage or the classes of 256
// glyphs (hb_ot_layout_mapping_cache_t). okage keeps such a cache for each
// PairPos subtable, the kerning: its coverage, and the classes of a
// subtable of format 2. A cache only keeps the result of a lookup in a
// table of the font, so the glyphs do not change.

// maxCachedSubtables is the number of the first subtables of a lookup that
// have a cache, as in HarfBuzz.
const maxCachedSubtables = 8

// mappingCache maps a glyph to a value of 16 bits, as
// hb_ot_layout_mapping_cache_t of HarfBuzz. Each item holds a glyph in its
// high 16 bits and the value of the glyph in its low 16 bits, at the index
// of the low 8 bits of the glyph.
type mappingCache [256]uint32

// clear empties the cache. Each item holds a glyph whose low 8 bits are not
// its index, so no glyph matches it.
func (mc *mappingCache) clear() {
	for i := range mc {
		mc[i] = uint32(i^1) << 16
	}
}

// get returns the value of g, and whether the cache holds it.
func (mc *mappingCache) get(g gID) (uint16, bool) {
	e := mc[uint8(g)]
	return uint16(e), gID(e>>16) == g
}

// set keeps the value of g.
func (mc *mappingCache) set(g gID, v uint16) {
	mc[uint8(g)] = uint32(g)<<16 | uint32(v)
}

// notCovered is the value of a glyph that a coverage does not hold, in a
// cache of a coverage. A coverage index of 0xFFFF or more is not kept.
const notCovered = 0xFFFF

// pairPosCache is the cache of a PairPos subtable: its coverage, and for
// format 2 its first class definition and its second class definition.
type pairPosCache struct {
	coverage      mappingCache
	first, second *mappingCache
}

// newPairPosCache returns an empty cache for a subtable of format 2, or of
// format 1 when classes is false.
func newPairPosCache(classes bool) *pairPosCache {
	pc := new(pairPosCache)
	pc.coverage.clear()
	if classes {
		pc.first, pc.second = new(mappingCache), new(mappingCache)
		pc.first.clear()
		pc.second.clear()
	}
	return pc
}

// index returns cov.Index(g), from the cache when pc holds it. A nil pc
// does the lookup.
func (pc *pairPosCache) index(cov tables.Coverage, g gID) (int, bool) {
	if pc == nil {
		return cov.Index(g)
	}
	if v, ok := pc.coverage.get(g); ok {
		return int(v), v != notCovered
	}
	index, ok := cov.Index(g)
	switch {
	case !ok:
		pc.coverage.set(g, notCovered)
	case 0 <= index && index < notCovered:
		pc.coverage.set(g, uint16(index))
	}
	return index, ok
}

// classes returns the class of g1 in inner.ClassDef1 and the class of g2
// in inner.ClassDef2, from the cache when pc holds them. A nil pc does the
// lookups, and so does a pc of format 1. The class of a glyph that a class
// definition does not hold is 0.
func (pc *pairPosCache) classes(inner *tables.PairPosData2, g1, g2 gID) (class1, class2 uint16) {
	if pc == nil || pc.first == nil {
		class1, _ = inner.ClassDef1.Class(g1)
		class2, _ = inner.ClassDef2.Class(g2)
		return class1, class2
	}
	return cachedClass(inner.ClassDef1, pc.first, g1), cachedClass(inner.ClassDef2, pc.second, g2)
}

// cachedClass returns the class of g in cd, from the cache when it holds
// it.
func cachedClass(cd tables.ClassDef, cache *mappingCache, g gID) uint16 {
	if v, ok := cache.get(g); ok {
		return v
	}
	class, _ := cd.Class(g)
	cache.set(g, class)
	return class
}
