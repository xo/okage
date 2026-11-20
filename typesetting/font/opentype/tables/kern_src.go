// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type KernSubtable interface {
	// Data returns the actual kerning data
	Data() KernData
}

type OTKernSubtableHeader struct {
	version  uint16        // Kern subtable version number
	length   uint16        // Length of the subtable, in bytes (including this header).
	format   kernSTVersion // What type of information is contained in this table.
	Coverage byte          // What type of information is contained in this table.
	data     KernData      `unionField:"format"`
}

// check and return the length
func (st *OTKernSubtableHeader) parseEnd(src []byte) (int, error) {
	if L, E := len(src), int(st.length); L < E {
		return 0, fmt.Errorf("EOF: expected length: %d, got %d", E, L)
	}
	// okage: the values of format 1 go up to the end of the subtable (D73).
	st.data = readKern1Values(st.data, src[6:max(6, int(st.length))])
	return int(st.length), nil
}

type AATKernSubtableHeader struct {
	length     uint32 // The length of this subtable in bytes, including this header.
	Coverage   byte   // Circumstances under which this table is used.
	version    kernSTVersion
	TupleCount uint16   // The tuple count. This value is only used with variation fonts and should be 0 for all other fonts. The subtable's tupleCount will be ignored if the 'kerx' table version is less than 4.
	data       KernData `unionField:"version"`
}

// check and return the length
func (st *AATKernSubtableHeader) parseEnd(src []byte) (int, error) {
	if L, E := len(src), int(st.length); L < E {
		return 0, fmt.Errorf("EOF: expected length: %d, got %d", E, L)
	}
	// okage: the values of format 1 go up to the end of the subtable (D73).
	st.data = readKern1Values(st.data, src[8:max(8, int(st.length))])
	return int(st.length), nil
}

type KernData interface {
	isKernData()
}

func (KernData0) isKernData() {}
func (KernData1) isKernData() {}
func (KernData2) isKernData() {}
func (KernData3) isKernData() {}

type kernSTVersion byte

const (
	kernSTVersion0 kernSTVersion = iota
	kernSTVersion1
	kernSTVersion2
	kernSTVersion3
)

type KernData0 struct {
	nPairs        uint16         //	The number of kerning pairs in this subtable.
	searchRange   uint16         //	The largest power of two less than or equal to the value of nPairs, multiplied by the size in bytes of an entry in the subtable.
	entrySelector uint16         //	This is calculated as log2 of the largest power of two less than or equal to the value of nPairs. This value indicates how many iterations of the search loop have to be made. For example, in a list of eight items, there would be three iterations of the loop.
	rangeShift    uint16         //	The value of nPairs minus the largest power of two less than or equal to nPairs. This is multiplied b
	Pairs         []Kernx0Record `arrayCount:"ComputedField-nPairs"`
}

type KernData1 struct {
	AATStateTable
	valueTable uint16  // Offset in bytes from the beginning of the subtable to the beginning of the kerning table.
	Values     []int16 `isOpaque:""`
}

// Kern1OutOfRange is the index of the values of an entry of a subtable of
// format 1 of kern whose list starts before the value table. The index of
// a list that starts in the value table is at most 0x1FFF.
//
// okage: HarfBuzz 15.1.0 applies such a list with an index out of range,
// and empties the stack (D73).
const Kern1OutOfRange = 0xFFFE

// okage: parseValues resolves the offset of each entry to an index. An
// entry with the offset 0 has no list, and an entry whose list starts
// before the value table has the index Kern1OutOfRange, as in HarfBuzz
// 15.1.0. go-text gave it no list. readKern1Values reads the values once
// the length of the subtable is known (D73).
func (kd *KernData1) parseValues(_ []byte) error {
	valuesOffset := int(kd.valueTable)
	// start by resolving offset -> index
	for i := range kd.Entries {
		entry := &kd.Entries[i]
		offset := int(entry.Flags & Kern1Offset)
		switch {
		case offset == 0:
			binary.BigEndian.PutUint16(entry.data[:], 0xFFFF)
		case offset < valuesOffset:
			binary.BigEndian.PutUint16(entry.data[:], Kern1OutOfRange)
		default:
			index := uint16((offset - valuesOffset) / 2)
			binary.BigEndian.PutUint16(entry.data[:], index)
		}
	}
	return nil
}

// readKern1Values reads the values of a subtable of format 1 from its value
// table to the end of src, which starts at the state table. It gives the
// other formats as they are.
//
// okage: HarfBuzz 15.1.0 requires the values for the whole stack in the
// subtable, and in the table for the last subtable, whose length it
// ignores. An odd value does not end the values. go-text read the values
// up to the first odd value after the largest index of an entry, also
// after the end of the subtable, and refused the table when it was too
// short for that index (D73).
func readKern1Values(data KernData, src []byte) KernData {
	kd, ok := data.(KernData1)
	if !ok {
		return data
	}
	kd.Values = nil
	if start := int(kd.valueTable); start < len(src) {
		values := src[start:]
		kd.Values = make([]int16, len(values)/2)
		for i := range kd.Values {
			kd.Values[i] = int16(binary.BigEndian.Uint16(values[2*i:]))
		}
	}
	return kd
}

type KernData2 struct {
	rowWidth     uint16          // The width, in bytes, of a row in the subtable.
	Left         AATLoopkup8Data `offsetSize:"Offset16" offsetRelativeTo:"Parent"`
	Right        AATLoopkup8Data `offsetSize:"Offset16" offsetRelativeTo:"Parent"`
	KerningStart Offset16        // Offset from beginning of this subtable to the start of the kerning array.
	KerningData  []byte          `isOpaque:"" offsetRelativeTo:"Parent"` // indexed by Left + Right
}

func (kd *KernData2) parseKerningData(_ []byte, parentSrc []byte) error {
	kd.KerningData = parentSrc
	return nil
}

type KernData3 struct {
	glyphCount      uint16  // The number of glyphs in this font.
	kernValueCount  uint8   // The number of kerning values.
	leftClassCount  uint8   // The number of left-hand classes.
	RightClassCount uint8   // The number of right-hand classes.
	flags           uint8   // Set to zero (reserved for future use).
	Kernings        []int16 `arrayCount:"ComputedField-kernValueCount"`
	LeftClass       []uint8 `arrayCount:"ComputedField-glyphCount"`
	RightClass      []uint8 `arrayCount:"ComputedField-glyphCount"`
	KernIndex       []uint8 `arrayCount:"ComputedField-nKernIndex()"`
}

func (kd *KernData3) nKernIndex() int { return int(kd.leftClassCount) * int(kd.RightClassCount) }

// sanitize index and class values
func (kd *KernData3) parseEnd(_ []byte) (int, error) {
	for _, index := range kd.KernIndex {
		if index >= kd.kernValueCount {
			return 0, errors.New("invalid kern subtable format 3 index value")
		}
	}

	for i := range kd.LeftClass {
		if kd.LeftClass[i] >= kd.leftClassCount {
			return 0, errors.New("invalid kern subtable format 3 left class value")
		}
		if kd.RightClass[i] >= kd.RightClassCount {
			return 0, errors.New("invalid kern subtable format 3 right class value")
		}
	}

	return 0, nil
}
