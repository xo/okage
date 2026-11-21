// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Ankr is the anchor point table
// See - https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6ankr.html
type Ankr struct {
	version uint16 // Version number (set to zero)
	flags   uint16 // Flags (currently unused; set to zero)
	// Offset to the table's lookup table; currently this is always 0x0000000C
	// The lookup table returns uint16 offset from the beginning of the glyph data table, not indices.
	lookupTable AATLookup `offsetSize:"Offset32"`
	// Offset to the glyph data table
	glyphDataTable []byte `offsetSize:"Offset32" arrayCount:"ToEnd"`
}

// GetAnchor return the i-th anchor for `glyph`, or {0,0} if not found.
//
// okage: a font without a table ankr gives {0,0}, as in HarfBuzz 15.1.0.
// go-text panicked on its nil lookup table (D73).
func (ank Ankr) GetAnchor(glyph GlyphID, index int) (anchor AnkrAnchor) {
	if ank.lookupTable == nil {
		return anchor
	}
	offset, ok := ank.lookupTable.Class(glyph)
	if !ok || int(offset)+4 >= len(ank.glyphDataTable) {
		return anchor
	}

	count := int(binary.BigEndian.Uint32(ank.glyphDataTable[offset:]))
	if index >= count {
		return anchor // invalid index
	}

	indexStart := int(offset) + 4 + 4*index
	if len(ank.glyphDataTable) < indexStart+4 {
		return anchor // invalid table
	}
	anchor.X = int16(binary.BigEndian.Uint16(ank.glyphDataTable[indexStart:]))
	anchor.Y = int16(binary.BigEndian.Uint16(ank.glyphDataTable[indexStart+2:]))
	return anchor
}

// AnkrAnchor is a point within the coordinate space of a given glyph
// independent of the control points used to render the glyph
type AnkrAnchor struct {
	X, Y int16
}

// checkAnchors checks that the glyph data holds the anchors of each value
// of the lookup table.
//
// okage: this is the sanitizer of ankr of HarfBuzz 15.1.0. It refuses the
// table when the anchors of a value are not in the table, and when the
// lookup has the format 10. go-text checked the anchors of a glyph only
// when it read them (D73).
func (ank Ankr) checkAnchors() error {
	check := func(offset uint16) error {
		start := int(offset)
		if len(ank.glyphDataTable) < start+4 {
			return fmt.Errorf("EOF: expected length: %d, got %d", start+4, len(ank.glyphDataTable))
		}
		count := uint64(binary.BigEndian.Uint32(ank.glyphDataTable[start:]))
		if uint64(len(ank.glyphDataTable)) < uint64(start)+4+4*count {
			return fmt.Errorf("EOF: expected length: %d, got %d", uint64(start)+4+4*count, len(ank.glyphDataTable))
		}
		return nil
	}
	var values []uint16
	switch lk := ank.lookupTable.(type) {
	case nil:
		return nil
	case AATLoopkup0:
		values = lk.Values
	case AATLoopkup2:
		for _, r := range lk.Records {
			values = append(values, r.Value)
		}
	case AATLoopkup4:
		for _, r := range lk.Records {
			values = append(values, r.Values...)
		}
	case AATLoopkup6:
		for _, r := range lk.Records {
			values = append(values, r.Value)
		}
	case AATLoopkup8:
		values = lk.Values
	case AATLoopkup10:
		return errors.New("unsupported lookup format 10")
	}
	for _, v := range values {
		if err := check(v); err != nil {
			return err
		}
	}
	return nil
}
