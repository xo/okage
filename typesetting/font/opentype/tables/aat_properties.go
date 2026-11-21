// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import (
	"sort"
)

// This file implements routines used to simplify acces to the tables
// data.

func (lk AATLoopkup0) Class(g GlyphID) (uint16, bool) {
	if int(g) >= len(lk.Values) {
		return 0, false
	}
	return lk.Values[g], true
}

func (lk AATLoopkup2) Class(g GlyphID) (uint16, bool) {
	// 'adapted' from golang/x/image/font/sfnt
	c := lk.Records
	num := len(c)
	if num == 0 {
		return 0, false
	}

	// classRange is an array of startGlyphID, endGlyphID and target class ID.
	// Ranges are non-overlapping.
	// E.g. 130, 135, 1   137, 137, 5   etc

	idx := sort.Search(num, func(i int) bool { return g <= c[i].FirstGlyph })
	// idx either points to a matching start, or to the next range (or idx==num)
	// e.g. with the range example from above: 130 points to 130-135 range, 133 points to 137-137 range

	// check if gi is the start of a range, but only if sort.Search returned a valid result
	if idx < num {
		if class := c[idx]; g == c[idx].FirstGlyph {
			return class.Value, true
		}
	}
	// check if gi is in previous range
	if idx > 0 {
		idx--
		if class := c[idx]; g >= class.FirstGlyph && g <= class.LastGlyph {
			return class.Value, true
		}
	}

	return 0, false
}

func (lk AATLoopkup4) Class(g GlyphID) (uint16, bool) {
	// binary search
	for i, j := 0, len(lk.Records); i < j; {
		h := i + (j-i)/2
		entry := lk.Records[h]
		if g < entry.FirstGlyph {
			j = h
		} else if entry.LastGlyph < g {
			i = h + 1
		} else {
			return entry.Values[g-entry.FirstGlyph], true
		}
	}
	return 0, false
}

func (lk AATLoopkup6) Class(g GlyphID) (uint16, bool) {
	// binary search
	for i, j := 0, len(lk.Records); i < j; {
		h := i + (j-i)/2
		entry := lk.Records[h]
		if g < entry.Glyph {
			j = h
		} else if entry.Glyph < g {
			i = h + 1
		} else {
			return entry.Value, true
		}
	}
	return 0, false
}

func (lk AATLoopkup8Data) Class(g GlyphID) (uint16, bool) {
	if g < lk.FirstGlyph || g >= lk.FirstGlyph+GlyphID(len(lk.Values)) {
		return 0, false
	}
	return lk.Values[g-lk.FirstGlyph], true
}

// Class returns (0, false) for each glyph.
//
// okage: HarfBuzz 15.1.0 finds no value in a lookup of format 10 for a
// class table, a substitution of morx or the classes of kerx format 2.
// Only the rows and the columns of kerx format 6 read its values, with
// ClassUint32. go-text gave the values in each place (D73).
func (lk AATLoopkup10) Class(GlyphID) (uint16, bool) { return 0, false }

// value gives the value of the glyph, or (0, false) for a glyph that the
// lookup does not cover.
func (lk AATLoopkup10) value(g GlyphID) (uint16, bool) {
	if g < lk.FirstGlyph || g >= lk.FirstGlyph+GlyphID(len(lk.Values)) {
		return 0, false
	}
	return lk.Values[g-lk.FirstGlyph], true
}

func (lk AATLoopkupExt0) Class(g GlyphID) (uint32, bool) {
	if int(g) >= len(lk.Values) {
		return 0, false
	}
	return lk.Values[g], true
}

func (lk AATLoopkupExt2) Class(g GlyphID) (uint32, bool) {
	// 'adapted' from golang/x/image/font/sfnt
	c := lk.Records
	num := len(c)
	if num == 0 {
		return 0, false
	}

	// classRange is an array of startGlyphID, endGlyphID and target class ID.
	// Ranges are non-overlapping.
	// E.g. 130, 135, 1   137, 137, 5   etc

	idx := sort.Search(num, func(i int) bool { return g <= c[i].FirstGlyph })
	// idx either points to a matching start, or to the next range (or idx==num)
	// e.g. with the range example from above: 130 points to 130-135 range, 133 points to 137-137 range

	// check if gi is the start of a range, but only if sort.Search returned a valid result
	if idx < num {
		if class := c[idx]; g == c[idx].FirstGlyph {
			return class.Value, true
		}
	}
	// check if gi is in previous range
	if idx > 0 {
		idx--
		if class := c[idx]; g >= class.FirstGlyph && g <= class.LastGlyph {
			return class.Value, true
		}
	}

	return 0, false
}

func (lk AATLoopkupExt4) Class(g GlyphID) (uint32, bool) {
	// binary search
	for i, j := 0, len(lk.Records); i < j; {
		h := i + (j-i)/2
		entry := lk.Records[h]
		if g < entry.FirstGlyph {
			j = h
		} else if entry.LastGlyph < g {
			i = h + 1
		} else {
			return entry.Values[g-entry.FirstGlyph], true
		}
	}
	return 0, false
}

func (lk AATLoopkupExt6) Class(g GlyphID) (uint32, bool) {
	// binary search
	for i, j := 0, len(lk.Records); i < j; {
		h := i + (j-i)/2
		entry := lk.Records[h]
		if g < entry.Glyph {
			j = h
		} else if entry.Glyph < g {
			i = h + 1
		} else {
			return entry.Value, true
		}
	}
	return 0, false
}

func (lk AATLoopkupExt8) Class(g GlyphID) (uint32, bool) {
	if g < lk.FirstGlyph || g >= lk.FirstGlyph+GlyphID(len(lk.Values)) {
		return 0, false
	}
	return lk.Values[g-lk.FirstGlyph], true
}

// Class returns (0, false) for each glyph, as AATLoopkup10.Class.
//
// okage: as in HarfBuzz 15.1.0 (D73).
func (lk AATLoopkupExt10) Class(GlyphID) (uint32, bool) { return 0, false }

// value gives the value of the glyph, or (0, false) for a glyph that the
// lookup does not cover.
func (lk AATLoopkupExt10) value(g GlyphID) (uint32, bool) {
	if g < lk.FirstGlyph || g >= lk.FirstGlyph+GlyphID(len(lk.Values)) {
		return 0, false
	}
	return lk.Values[g-lk.FirstGlyph], true
}

type AatLookupMixed interface {
	// Returns 0 if not supported
	ClassUint32(GlyphID) uint32

	// Coverage returns the glyphs covered by this lookup as
	// a list of inclusive ranges
	Coverage() [][2]GlyphID
}

func (lk AATLoopkup0) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return uint32(v)
}

func (lk AATLoopkup2) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return uint32(v)
}

func (lk AATLoopkup4) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return uint32(v)
}

func (lk AATLoopkup6) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return uint32(v)
}

func (lk AATLoopkup8) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return uint32(v)
}

// ClassUint32 returns the value of the glyph, or 0.
//
// okage: the rows and the columns of kerx format 6 read the values of a
// lookup of format 10, as in HarfBuzz 15.1.0 (D73).
func (lk AATLoopkup10) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.value(g)
	return uint32(v)
}

func (lk AATLoopkupExt0) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return v
}

func (lk AATLoopkupExt2) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return v
}

func (lk AATLoopkupExt4) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return v
}

func (lk AATLoopkupExt6) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return v
}

func (lk AATLoopkupExt8) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.Class(g)
	return v
}

// ClassUint32 returns the value of the glyph, or 0, as
// AATLoopkup10.ClassUint32.
//
// okage: as in HarfBuzz 15.1.0 (D73).
func (lk AATLoopkupExt10) ClassUint32(g GlyphID) uint32 {
	v, _ := lk.value(g)
	return v
}

// GetFeature performs a binary seach into the names, using `Feature` as key,
// returning `nil` if not found.
func (ft Feat) GetFeature(feature uint16) *FeatureName {
	for i, j := 0, len(ft.Names); i < j; {
		h := i + (j-i)/2
		entry := ft.Names[h].Feature
		if feature < entry {
			j = h
		} else if entry < feature {
			i = h + 1
		} else {
			return &ft.Names[h]
		}
	}
	return nil
}

// IsExclusive returns true if the feature settings are mutually exclusive.
func (feature *FeatureName) IsExclusive() bool {
	const Exclusive = 0x8000
	return feature.FeatureFlags&Exclusive != 0
}
