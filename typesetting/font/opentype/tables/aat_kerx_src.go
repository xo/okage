// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Kerx is the extended kerning table
// See https://developer.apple.com/fonts/TrueType-Reference-Manual/RM06/Chap6kerx.html
type Kerx struct {
	version uint16         // The version number of the extended kerning table (currently 2, 3, or 4).
	padding uint16         // Unused; set to zero.
	nTables uint32         // The number of subtables included in the extended kerning table.
	Tables  []KerxSubtable `arrayCount:"ComputedField-nTables"`
}

// extended versions

// binarygen: argument=valuesCount int
type KerxSubtable struct {
	length     uint32 // The length of this subtable in bytes, including this header.
	Coverage   uint16 // Circumstances under which this table is used.
	padding    byte   // unused
	version    kerxSTVersion
	TupleCount uint32   // The tuple count. This value is only used with variation fonts and should be 0 for all other fonts. The subtable's tupleCount will be ignored if the 'kerx' table version is less than 4.
	Data       KerxData `unionField:"version" arguments:"tupleCount=.TupleCount, valuesCount=valuesCount"`
}

// check and return the subtable length
func (ks *KerxSubtable) parseEnd(src []byte, _ int) (int, error) {
	if L := len(src); L < int(ks.length) {
		return 0, fmt.Errorf("EOF: expected length: %d, got %d", ks.length, L)
	}
	// okage: HarfBuzz 15.1.0 refuses a subtable shorter than its header
	// (D73).
	if ks.length < 12 {
		return 0, fmt.Errorf("invalid kerx subtable length: %d", ks.length)
	}
	ks.readKerx1Values(src[:ks.length])
	ks.readKerx2Data(src[:ks.length])
	ks.readKerx4Data(src[:ks.length])
	return int(ks.length), nil
}

// readKerx2Data sets the kerning data of a subtable of format 2 to src,
// which starts at the start of the subtable.
//
// okage: HarfBuzz 15.1.0 reads a value of the kerning array up to the end
// of the subtable, and up to the end of the table for the last subtable.
// ParseKerx sets the data of the last subtable again, up to the end of the
// table. go-text kept the data from the end of the header of the subtable
// to the end of the table (D73).
func (ks *KerxSubtable) readKerx2Data(src []byte) {
	if kd, ok := ks.Data.(KerxData2); ok {
		kd.KerningData = src
		ks.Data = kd
	}
}

// readKerx1Values reads the values of a subtable of format 1, from its value
// table to the end of src. src starts at the start of the subtable.
//
// okage: HarfBuzz 15.1.0 reads the values of a list of actions up to the
// end of the subtable, and up to the end of the table for the last
// subtable, as the length of the last subtable is ignored. It checks each
// value when it reads it, so a value table that is too short is not an
// error of the parse. ParseKerx reads the values of the last subtable
// again, up to the end of the table (D73).
func (ks *KerxSubtable) readKerx1Values(src []byte) {
	kd, ok := ks.Data.(KerxData1)
	if !ok {
		return
	}
	kd.Values = nil
	// The offset of the value table starts at the state table, after the
	// header of 12 bytes.
	if start := 12 + uint64(kd.valueTable); start < uint64(len(src)) {
		values := src[start:]
		kd.Values = make([]int16, len(values)/2)
		for i := range kd.Values {
			kd.Values[i] = int16(binary.BigEndian.Uint16(values[2*i:]))
		}
	}
	ks.Data = kd
}

type kerxSTVersion byte

const (
	kerxSTVersion0 kerxSTVersion = iota
	kerxSTVersion1
	kerxSTVersion2
	_
	kerxSTVersion4
	_
	kerxSTVersion6
)

type KerxData interface {
	isKerxData()
}

func (KerxData0) isKerxData() {}
func (KerxData1) isKerxData() {}
func (KerxData2) isKerxData() {}
func (KerxData4) isKerxData() {}
func (KerxData6) isKerxData() {}

// binarygen: argument=tupleCount int
type KerxData0 struct {
	nPairs        uint32
	searchRange   uint32
	entrySelector uint32
	rangeShift    uint32
	Pairs         []Kernx0Record `arrayCount:"ComputedField-nPairs"`
}

// okage: go-text read each value of a subtable with a tuple count here, as
// an offset from the end of the header of the subtable, and refused the
// table when the value was not in the subtable. ParseKerx now reads it
// with KerxSubtable.readKerx0Tuples (D73).
func (kd *KerxData0) parseEnd(src []byte, _ int) (int, error) {
	return len(src), nil
}

// readKerx0Tuples reads the value of each pair of a subtable of format 0
// with a tuple count, with KerxTupleKern. src starts at the start of the
// subtable, and ends at the end of the subtable, or at the end of the table
// for the last subtable.
//
// okage: HarfBuzz 15.1.0 reads a value as a byte offset from the start of
// the subtable to the values of the tuples, and gives 0 when they are not
// in the subtable, or not in the table for the last subtable (D73).
func (ks *KerxSubtable) readKerx0Tuples(src []byte) {
	kd, ok := ks.Data.(KerxData0)
	if !ok || ks.TupleCount == 0 {
		return
	}
	for i, pair := range kd.Pairs {
		kd.Pairs[i].Value = KerxTupleKern(src, int32(pair.Value), int(ks.TupleCount))
	}
}

// KerxTupleKern gives the first value of the tuples at the byte offset
// value in src, or 0 when src does not hold tupleCount values there. A
// negative value is an offset after the end of src.
//
// okage: this is kerxTupleKern of HarfBuzz 15.1.0, for a subtable of kerx
// with a tuple count (D73).
func KerxTupleKern(src []byte, value int32, tupleCount int) int16 {
	offset := uint64(uint32(value))
	if uint64(len(src)) < offset+2*uint64(tupleCount) {
		return 0
	}
	return int16(binary.BigEndian.Uint16(src[offset:]))
}

type Kernx0Record struct {
	Left, Right GlyphID
	Value       int16
}

// Kernx1 state entry flags
const (
	Kerx1Push        = 0x8000 // If set, push this glyph on the kerning stack.
	Kerx1DontAdvance = 0x4000 // If set, don't advance to the next glyph before going to the new state.
	Kerx1Reset       = 0x2000 // If set, reset the kerning data (clear the stack)
	Kern1Offset      = 0x3FFF // Byte offset from beginning of subtable to the  value table for the glyphs on the kerning stack.
)

// binarygen: argument=tupleCount int
// binarygen: argument=valuesCount int
type KerxData1 struct {
	AATStateTableExt `arguments:"valuesCount=valuesCount, entryDataSize=2"`
	valueTable       Offset32
	Values           []int16 `isOpaque:""`
}

// okage: go-text read the values of format 1 of kern and kerx with
// parseKernx1Values, up to the first odd value after the largest index of
// an entry. KerxSubtable.readKerx1Values and readKern1Values replace it,
// and read the values once the length of the subtable is known (D73).
func (kx *KerxData1) parseValues(_ []byte, _, _ int) error {
	return nil
}

type KerxData2 struct {
	rowWidth     uint32    // The number of bytes in each row of the kerning value array
	Left         AATLookup `offsetSize:"Offset32" offsetRelativeTo:"Parent"` // Offset from beginning of this subtable to the left-hand offset table.
	Right        AATLookup `offsetSize:"Offset32" offsetRelativeTo:"Parent"` // Offset from beginning of this subtable to right-hand offset table.
	KerningStart Offset32  // Offset from beginning of this subtable to the start of the kerning array.
	// okage: KerningData starts at the start of the subtable, as the data
	// of KernData2 does. KerxSubtable.readKerx2Data sets it (D73).
	KerningData []byte `subsliceStart:"AtStart" arrayCount:"ToEnd"`
}

// binarygen: argument=valuesCount int
type KerxData4 struct {
	AATStateTableExt `arguments:"valuesCount=valuesCount,entryDataSize=2"`
	Flags            uint32
	// okage: AnchorData holds the bytes from the control point table to the
	// end of the subtable, or to the end of the table for the last
	// subtable, as HarfBuzz 15.1.0 reads them. It is nil when the table
	// starts after that end. KerxSubtable.readKerx4Data sets it. go-text
	// read the actions of the largest index of an entry into Anchors, and
	// refused the table when they were not in the subtable, or when the
	// action type was 3 (D73).
	AnchorData []byte `isOpaque:""`
}

// okage: KerxSubtable.readKerx4Data reads the actions (D73).
func (kd *KerxData4) parseAnchors(_ []byte, _ int) error {
	return nil
}

// readKerx4Data sets the data of the actions of a subtable of format 4.
// src starts at the start of the subtable.
//
// okage: HarfBuzz 15.1.0 reads an action when it applies the subtable, and
// skips the action when it is not in the subtable, or not in the table for
// the last subtable. ParseKerx sets the data of the last subtable again, up
// to the end of the table (D73).
func (ks *KerxSubtable) readKerx4Data(src []byte) {
	kd, ok := ks.Data.(KerxData4)
	if !ok {
		return
	}
	const Offset = 0x00FFFFFF // Masks the offset in bytes from the beginning of the subtable to the beginning of the control point table.
	kd.AnchorData = nil
	// The offset starts at the state table, after the header of 12 bytes.
	if start := 12 + int(kd.Flags&Offset); start <= len(src) {
		kd.AnchorData = src[start:]
	}
	ks.Data = kd
}

// ActionType returns 0 to 3, according to the anchor format :
//   - 0 : KerxAnchorControls
//   - 1 : KerxAnchorAnchors
//   - 2 : KerxAnchorCoordinates
//   - 3 : no action data. okage: HarfBuzz 15.1.0 accepts it, and still
//     attaches the glyph (D73).
func (kd KerxData4) ActionType() uint8 {
	const ActionType = 0xC0000000 // A two-bit field containing the action type.
	return uint8((kd.Flags & ActionType) >> 30)
}

type KerxAnchors interface {
	isKerxAnchors()
}

func (KerxAnchorControls) isKerxAnchors()    {}
func (KerxAnchorAnchors) isKerxAnchors()     {}
func (KerxAnchorCoordinates) isKerxAnchors() {}

type KerxAnchorControls struct {
	Anchors []KAControl
}
type KerxAnchorAnchors struct {
	Anchors []KAAnchor
}
type KerxAnchorCoordinates struct {
	Anchors []KACoordinates
}

type KAControl struct {
	Mark, Current uint16
}

type KAAnchor struct {
	Mark, Current uint16
}

type KACoordinates struct {
	MarkX, MarkY, CurrentX, CurrentY int16
}

// binarygen: argument=tupleCount int
// binarygen: argument=valuesCount int
type KerxData6 struct {
	flags                  uint32         // Flags for this subtable. See below.
	rowCount               uint16         // The number of rows in the kerning value array
	columnCount            uint16         // The number of columns in the kerning value array
	rowIndexTableOffset    uint32         // Offset from beginning of this subtable to the row index lookup table.
	columnIndexTableOffset uint32         // Offset from beginning of this subtable to column index offset table.
	kerningArrayOffset     uint32         // Offset from beginning of this subtable to the start of the kerning array.
	kerningVectorOffset    uint32         // Offset from beginning of this subtable to the start of the kerning vectors. This value is only present if the tupleCount for this subtable is 1 or more.
	Row                    AatLookupMixed `isOpaque:"" offsetRelativeTo:"Parent"` // Values are pre-multiplied by `columnCount`
	Column                 AatLookupMixed `isOpaque:"" offsetRelativeTo:"Parent"`
	// okage: data holds the subtable, up to its end, or up to the end of
	// the table for the last subtable, and tupleCount is the tuple count of
	// the subtable. Kerning reads a value from them, as HarfBuzz 15.1.0
	// does. go-text read rowCount × columnCount values into Kernings, and
	// refused the table when they were not in the subtable (D73).
	data       []byte `isOpaque:"" offsetRelativeTo:"Parent"`
	tupleCount int
}

func (kd *KerxData6) parseRow(_, parentSrc []byte, _, valuesCount int) error {
	isExtended := kd.flags&1 != 0
	if L := len(parentSrc); L < int(kd.rowIndexTableOffset) {
		return fmt.Errorf("EOF: expected length: %d, got %d", kd.rowIndexTableOffset, L)
	}
	var err error
	if isExtended {
		kd.Row, _, err = ParseAATLookupExt(parentSrc[kd.rowIndexTableOffset:], valuesCount)
	} else {
		kd.Row, _, err = ParseAATLookup(parentSrc[kd.rowIndexTableOffset:], valuesCount)
	}
	return err
}

func (kd *KerxData6) parseColumn(_, parentSrc []byte, _, valuesCount int) error {
	isExtended := kd.flags&1 != 0
	if L := len(parentSrc); L < int(kd.columnIndexTableOffset) {
		return fmt.Errorf("EOF: expected length: %d, got %d", kd.columnIndexTableOffset, L)
	}
	var err error
	if isExtended {
		kd.Column, _, err = ParseAATLookupExt(parentSrc[kd.columnIndexTableOffset:], valuesCount)
	} else {
		kd.Column, _, err = ParseAATLookup(parentSrc[kd.columnIndexTableOffset:], valuesCount)
	}
	return err
}

// parseData keeps the subtable for Kerning. parentSrc starts at the start
// of the subtable, and ends at the end of the subtable, or at the end of
// the table for the last subtable.
//
// okage: HarfBuzz 15.1.0 refuses the table when the kerning array starts
// after the end of the subtable, or when the vector of the tuples does, for
// a subtable with a tuple count (D73).
func (kd *KerxData6) parseData(_, parentSrc []byte, tupleCount, _ int) error {
	if L := len(parentSrc); L < int(kd.kerningArrayOffset) {
		return fmt.Errorf("EOF: expected length: %d, got %d", kd.kerningArrayOffset, L)
	}
	if L := len(parentSrc); tupleCount != 0 && L < int(kd.kerningVectorOffset) {
		return fmt.Errorf("EOF: expected length: %d, got %d", kd.kerningVectorOffset, L)
	}
	kd.data = parentSrc
	kd.tupleCount = tupleCount
	return nil
}

// Kerning returns the kerning value for the pair, or 0.
//
// okage: Kerning follows KerxSubTableFormat6::get_kerning of HarfBuzz
// 15.1.0. The value is at the sum of the row and the column, in the
// kerning array, which has values of 32 bits for long values. The counts
// of the rows and the columns do not bound it, and the value is 0 when it
// is not in the subtable. With a tuple count, the value is a byte offset
// from the vector of the tuples, as for KerxTupleKern (D73).
func (kd KerxData6) Kerning(left, right GlyphID) int32 {
	l, r := uint64(kd.Row.ClassUint32(left)), uint64(kd.Column.ClassUint32(right))
	offset, size := l+r, uint64(2)
	if kd.flags&1 != 0 {
		size = 4
		// HarfBuzz gives 0 when the sum or the byte offset overflows 32 bits.
		if offset*size > math.MaxUint32 {
			return 0
		}
	}
	pos := uint64(kd.kerningArrayOffset) + offset*size
	if uint64(len(kd.data)) < pos+size {
		return 0
	}
	var v int32
	if size == 4 {
		v = int32(binary.BigEndian.Uint32(kd.data[pos:]))
	} else {
		v = int32(int16(binary.BigEndian.Uint16(kd.data[pos:])))
	}
	if kd.tupleCount == 0 {
		return v
	}
	return int32(KerxTupleKern(kd.data[kd.kerningVectorOffset:], v, kd.tupleCount))
}

//lint:ignore U1000 this type is required so that the code generator add a ParseAATLookupExt function
type dummy struct {
	A AATLookupExt
}
