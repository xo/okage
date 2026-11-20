// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"encoding/binary"
	"math"

	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports the part of subsetter src/interjector.rs that lib.rs
// calls to choose the interjector: Interjector, Interjector::is_skrifa,
// SkrifaInterjector::new and SkrifaInterjector::is_default_location. Unit 4
// of phase 13 ports SkrifaInterjector::interject and OutlinePath, and sets
// the seam interjectGlyph of lib.go. doc.go describes the seams.
//
// Upstream reads the font with skrifa, which reads it with read-fonts. The
// port reads the fvar and the avar tables with typesetting (D52, D62).
// fontRefFromIndex finds the tables of the font as read-fonts does. The
// Loader of typesetting cannot do it: it reads every font of a collection,
// where read-fonts reads only the font at the index.

// interjector is Interjector in subsetter src/interjector.rs. Its zero value
// is Interjector::Dummy, and a value with a skrifa interjector is
// Interjector::Skrifa.
type interjector struct {
	skrifa *skrifaInterjector
}

// isSkrifa is Interjector::is_skrifa in subsetter src/interjector.rs.
func (i interjector) isSkrifa() bool {
	return i.skrifa != nil
}

// skrifaInterjector is SkrifaInterjector in subsetter src/interjector.rs. It
// holds the font and the normalized coordinates of its location.
type skrifaInterjector struct {
	// fontRef is the font at the index of the font file.
	fontRef fontRef
	// location holds one normalized coordinate for each axis of the fvar
	// table, in the order of the axes. It is the Location of upstream.
	location []tables.Coord
	// inst holds the outlines and the metrics at the location, which
	// interject reads with typesetting. interjector_skrifa.go makes it on
	// the first call.
	inst *glyphInstance
}

// newSkrifaInterjector is SkrifaInterjector::new in subsetter
// src/interjector.rs. It returns false where upstream returns None: when
// read-fonts cannot read the font at the index.
func newSkrifaInterjector(data []byte, index uint32, location []Variation) (*skrifaInterjector, bool) {
	ref, ok := fontRefFromIndex(data, index)
	if !ok {
		return nil, false
	}
	return &skrifaInterjector{
		fontRef:  ref,
		location: axesLocation(ref, location),
	}, true
}

// isDefaultLocation is SkrifaInterjector::is_default_location in subsetter
// src/interjector.rs. It tells whether each normalized coordinate is 0.
func (s *skrifaInterjector) isDefaultLocation() bool {
	for _, c := range s.location {
		if c != 0 {
			return false
		}
	}
	return true
}

// axesLocation is font_ref.axes().location(location) of
// SkrifaInterjector::new in subsetter src/interjector.rs. It gives one
// normalized coordinate for each axis of the fvar table:
//
//   - A setting whose tag matches no axis is ignored.
//   - A setting is clamped to the range of its axis.
//   - If more than one setting for an axis is given, the last one is used.
//   - An axis without a setting is at its default, which is 0.
//
// skrifa computes the coordinates in the 16.16 fixed-point numbers of
// OpenType, maps them with the avar table, and rounds them to 2.14 at the
// end, in Fvar::user_to_normalized of read-fonts src/tables/fvar.rs. The
// port does the same with the avar table of version 1 of typesetting
// (D84). typesetting does not read the avar table of version 2, which skrifa
// reads.
func axesLocation(ref fontRef, location []Variation) []tables.Coord {
	raw, ok := ref.table(Tag{'f', 'v', 'a', 'r'})
	if !ok {
		return nil
	}
	fvar, _, err := tables.ParseFvar(raw)
	if err != nil {
		return nil
	}
	axes := fvar.Axis
	fixed := make([]tables.Fixed, len(axes))
	for i, a := range axes {
		for _, v := range location {
			if [4]byte(v.Tag) == tagBytes(a.Tag) {
				fixed[i] = normalizeAxis(a, fixedFromF64(float64(v.Value)))
			}
		}
	}

	if raw, ok := ref.table(Tag{'a', 'v', 'a', 'r'}); ok {
		if avar, _, err := tables.ParseAvar(raw); err == nil {
			for i := range min(len(fixed), len(avar.AxisSegmentMaps)) {
				fixed[i] = avar.AxisSegmentMaps[i].MapFixed(fixed[i])
			}
		}
	}
	coords := make([]tables.Coord, len(axes))
	for i, f := range fixed {
		coords[i] = f.ToCoord()
	}
	return coords
}

// normalizeAxis normalizes a coordinate of the user to the range -1 to 1 of
// the axis, as VariationAxisRecord::normalize of read-fonts
// src/tables/fvar.rs does in the 16.16 format.
func normalizeAxis(a tables.VariationAxisRecord, value tables.Fixed) tables.Fixed {
	minValue, defaultValue := fixedOfFloat1616(a.Minimum), fixedOfFloat1616(a.Default)
	// Make sure max is >= min to avoid potential panic in clamp.
	maxValue := max(fixedOfFloat1616(a.Maximum), minValue)
	value = min(max(value, minValue), maxValue)
	switch {
	case value < defaultValue:
		value = -saturatingSub(defaultValue, value).Div(saturatingSub(defaultValue, minValue))
	case value > defaultValue:
		value = saturatingSub(value, defaultValue).Div(saturatingSub(maxValue, defaultValue))
	default:
		value = 0
	}
	return min(max(value, -tables.FixedOne), tables.FixedOne)
}

// fixedOfFloat1616 gives the bits of a number of the fvar table, which
// typesetting reads as a float32.
func fixedOfFloat1616(v float32) tables.Fixed {
	return tables.Fixed(int32(tables.Float1616ToUint(v)))
}

// fixedFromF64 is Fixed::from_f64 of font-types src/fixed.rs: it rounds
// half away from zero, and saturates as the conversion as i32 of Rust does.
// NaN gives 0.
func fixedFromF64(x float64) tables.Fixed {
	frac := -0.5
	if !math.Signbit(x) {
		frac = 0.5
	}
	v := float64(x*65536) + frac
	switch {
	case v != v:
		return 0
	case v <= math.MinInt32:
		return math.MinInt32
	case v >= math.MaxInt32:
		return math.MaxInt32
	}
	return tables.Fixed(int32(v))
}

// saturatingSub is Fixed::saturating_sub of font-types src/fixed.rs.
func saturatingSub(a, b tables.Fixed) tables.Fixed {
	d := int64(a) - int64(b)
	return tables.Fixed(min(max(d, math.MinInt32), math.MaxInt32))
}

// tagBytes gives the four bytes of a tag of typesetting.
func tagBytes(t ot.Tag) [4]byte {
	return [4]byte{byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t)}
}

// fontRef is FontRef of read-fonts src/lib.rs: the data of a font file, and
// the table records of the font at an index.
type fontRef struct {
	data    []byte
	records []tableRecord
	sorted  bool
}

// fontRefFromIndex is FontRef::from_index of read-fonts src/lib.rs. It
// returns false where read-fonts returns an error:
//
//   - The file has fewer than 12 bytes.
//   - The file is a collection, and the index is not less than the number
//     of fonts, or the array of the offsets of the fonts is out of the file.
//   - The file is one font, and the index is not 0.
//   - The table directory of the font has fewer than 12 bytes, or its
//     version is not 0x00010000, "OTTO" or "true".
//
// As read-fonts, it gives a font with no table when the table records are
// out of the file.
func fontRefFromIndex(data []byte, index uint32) (fontRef, bool) {
	if len(data) < 12 {
		return fontRef{}, false
	}
	dir := data
	if string(data[:4]) == "ttcf" {
		numFonts := uint64(binary.BigEndian.Uint32(data[8:]))
		if uint64(index) >= numFonts || 12+4*numFonts > uint64(len(data)) {
			return fontRef{}, false
		}
		offset := binary.BigEndian.Uint32(data[12+4*int(index):])
		if uint64(offset) > uint64(len(data)) {
			return fontRef{}, false
		}
		dir = data[offset:]
	} else if index != 0 {
		return fontRef{}, false
	}
	if len(dir) < 12 {
		return fontRef{}, false
	}
	switch binary.BigEndian.Uint32(dir) {
	case 0x00010000, 0x4F54544F, 0x74727565:
	default:
		return fontRef{}, false
	}

	ref := fontRef{data: data, sorted: true}
	numTables := int(binary.BigEndian.Uint16(dir[4:]))
	if 12+16*numTables <= len(dir) {
		r := newReaderAt(dir, 12)
		for range numTables {
			record, _ := readTableRecord(&r)
			ref.records = append(ref.records, record)
		}
	}
	var last Tag
	for _, record := range ref.records {
		if record.tag.compare(last) <= 0 {
			ref.sorted = false
			break
		}
		last = record.tag
	}
	return ref, true
}

// table is FontRef::table_data of read-fonts src/lib.rs. It searches the
// records with a binary search when their tags are sorted, and in order when
// they are not. It returns false when the font has no such table, when the
// offset of the table is 0, or when the table is out of the file.
func (f fontRef) table(tag Tag) ([]byte, bool) {
	i, ok := -1, false
	if f.sorted {
		i, ok = binarySearchRecords(f.records, tag)
	} else {
		for j, record := range f.records {
			if record.tag == tag {
				i, ok = j, true
				break
			}
		}
	}
	if !ok {
		return nil, false
	}
	record := f.records[i]
	start := uint64(record.offset)
	end := start + uint64(record.length)
	if start == 0 || end > uint64(len(f.data)) {
		return nil, false
	}
	return f.data[start:end], true
}
