// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

import (
	"encoding/binary"

	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// Kernx represents a 'kern' or 'kerx' kerning table.
// It supports both Microsoft and Apple formats.
type Kernx []KernSubtable

// aatMaxSubtables is the largest number of subtables of a table kerx, and
// of all the chains of a table morx, as HB_AAT_MAX_SUBTABLES of
// hb-limits.hh of HarfBuzz 15.1.0. tables has the same limit for the table
// kern.
//
// okage: HarfBuzz refuses a table with more subtables, and so does font
// (D73).
const aatMaxSubtables = 1024

func newKernxFromKerx(kerx tables.Kerx) Kernx {
	// okage: a table with more than aatMaxSubtables subtables is refused,
	// as in HarfBuzz 15.1.0 (D73).
	if len(kerx.Tables) == 0 || len(kerx.Tables) > aatMaxSubtables {
		return nil
	}
	out := make(Kernx, len(kerx.Tables))
	for i, ta := range kerx.Tables {
		out[i] = newKerxSubtable(ta)
	}
	return out
}

// newKernxFromKern converts the table kern of a font of nGlyphs glyphs.
//
// okage: a subtable of format 3 takes the number of glyphs of the font
// (D73).
func newKernxFromKern(kern tables.Kern, nGlyphs int) Kernx {
	if len(kern.Tables) == 0 {
		return nil
	}
	out := make(Kernx, len(kern.Tables))
	for i, ta := range kern.Tables {
		out[i] = newKernSubtable(ta)
		if kd, ok := out[i].Data.(Kern3); ok {
			out[i].Data = kd.withGlyphs(nGlyphs)
		}
	}
	return out
}

// KernSubtable represents a 'kern' or 'kerx' subtable.
type KernSubtable struct {
	Data interface{ isKernSubtable() }

	// high bit of the Coverage field, following 'kerx' conventions
	coverage byte

	// IsExtended is [true] for AAT `kerx` subtables, false for 'kern' subtables
	IsExtended bool

	// 0 for scalar values
	TupleCount int
}

func newKernSubtable(table tables.KernSubtable) (out KernSubtable) {
	out.IsExtended = false
	switch table := table.(type) {
	case tables.OTKernSubtableHeader:
		// synthesize a coverage flag following kerx conventions
		const (
			Horizontal  = 0x01
			CrossStream = 0x04
		)
		if table.Coverage&Horizontal == 0 { // vertical
			out.coverage |= kerxVertical
		}
		if table.Coverage&CrossStream != 0 {
			out.coverage |= kerxCrossStream
		}
	case tables.AATKernSubtableHeader:
		out.coverage = table.Coverage
		out.TupleCount = int(table.TupleCount)
	}
	switch data := table.Data().(type) {
	case tables.KernData0:
		out.Data = newKern0(data)
	case tables.KernData1:
		out.Data = newKern1(data)
	case tables.KernData2:
		out.Data = newKern2(data)
	case tables.KernData3:
		out.Data = Kern3(data)
	}
	return out
}

func newKerxSubtable(table tables.KerxSubtable) (out KernSubtable) {
	out.IsExtended = true
	out.TupleCount = int(table.TupleCount)
	out.coverage = byte(table.Coverage >> 8) // high bit only

	switch data := table.Data.(type) {
	case tables.KerxData0:
		out.Data = newKern0x(data)
	case tables.KerxData1:
		out.Data = newKern1x(data)
	case tables.KerxData2:
		out.Data = newKern2x(data, out.TupleCount)
	case tables.KerxData4:
		out.Data = newKern4(data)
	case tables.KerxData6:
		out.Data = Kern6(data)
	}
	return out
}

func (Kern0) isKernSubtable() {}
func (Kern1) isKernSubtable() {}
func (Kern2) isKernSubtable() {}
func (Kern3) isKernSubtable() {}
func (Kern4) isKernSubtable() {}
func (Kern6) isKernSubtable() {}

var (
	_ SimpleKerns = Kern0(nil)
	_ SimpleKerns = (*Kern2)(nil)
	_ SimpleKerns = (*Kern3)(nil)
	_ SimpleKerns = (*Kern6)(nil)
)

// SimpleKerns store a compact form of the kerning values,
// which is restricted to (one direction) kerning pairs.
// It is only implemented by [Kern0], [Kern2], [Kern3] and [Kern6],
// where [Kern1] and [Kern4] requires a state machine to be interpreted.
type SimpleKerns interface {
	// KernPair return the kern value for the given pair, or zero.
	// The value is expressed in glyph units and
	// is negative when glyphs should be closer.
	KernPair(left, right GID) int16
}

// kernx coverage flags
const (
	kerxBackwards   = 1 << (12 - 8)
	kerxVariation   = 1 << (13 - 8)
	kerxCrossStream = 1 << (14 - 8)
	kerxVertical    = 1 << (15 - 8)
)

// IsHorizontal returns true if the subtable has horizontal kerning values.
func (k KernSubtable) IsHorizontal() bool { return k.coverage&kerxVertical == 0 }

// IsBackwards returns true if state-table based should process the glyphs backwards.
func (k KernSubtable) IsBackwards() bool { return k.coverage&kerxBackwards != 0 }

// IsCrossStream returns true if the subtable has cross-stream kerning values.
func (k KernSubtable) IsCrossStream() bool { return k.coverage&kerxCrossStream != 0 }

// IsVariation returns true if the subtable has variation kerning values.
func (k KernSubtable) IsVariation() bool { return k.coverage&kerxVariation != 0 }

type Kern0 []tables.Kernx0Record

func newKern0(k tables.KernData0) Kern0  { return k.Pairs }
func newKern0x(k tables.KerxData0) Kern0 { return k.Pairs }

func kernPair(records []tables.Kernx0Record, left, right GID) int16 {
	key := uint32(left)<<16 | uint32(right)
	low, high := 0, len(records)
	for low < high {
		mid := low + (high-low)/2 // avoid overflow when computing mid
		p := recordKey(records[mid])
		if key < p {
			high = mid
		} else if key > p {
			low = mid + 1
		} else {
			return records[mid].Value
		}
	}
	return 0
}

func recordKey(kp tables.Kernx0Record) uint32 { return uint32(kp.Left)<<16 | uint32(kp.Right) }

func (kd Kern0) KernPair(left, right GID) int16 { return kernPair(kd, left, right) }

type Kern1 struct {
	// okage: Values holds the values up to the end of the subtable, or up
	// to the end of the table for the last subtable, as HarfBuzz 15.1.0
	// reads them. AATStateEntry.AsKernxIndex() of an entry of Machine can
	// be out of range. In kern, it is tables.Kern1OutOfRange for a list
	// that starts before the value table (D73).
	Values  []int16
	Machine AATStateTable
}

// convert from non extended to extended
func newKern1(k tables.KernData1) Kern1 {
	class := tables.AATLoopkup8{
		AATLoopkup8Data: tables.AATLoopkup8Data{
			FirstGlyph: k.ClassTable.StartGlyph,
			Values:     make([]uint16, len(k.ClassTable.Values)),
		},
	}
	for i, b := range k.ClassTable.Values {
		class.Values[i] = uint16(b)
	}
	states := make([][]uint16, len(k.States))
	for i, row := range k.States {
		v := make([]uint16, len(row))
		for j, b := range row {
			v[j] = uint16(b)
		}
		states[i] = v
	}
	return Kern1{
		Values: k.Values,
		Machine: AATStateTable{
			nClass:  uint32(k.StateSize),
			Class:   class,
			states:  states,
			entries: k.Entries,
		},
	}
}

func newKern1x(k tables.KerxData1) Kern1 {
	return Kern1{Values: k.Values, Machine: newAATStableTable(k.AATStateTableExt)}
}

// Kern2 is a subtable of format 2 of kern or kerx.
//
// okage: Kern2 has the exported fields of tables.KerxData2, and records
// whether it comes from kerx, because the two tables read the class values
// in other ways. KerningData starts at the start of the subtable (D73).
type Kern2 struct {
	Left         tables.AATLookup // The classes of the left glyphs.
	Right        tables.AATLookup // The classes of the right glyphs.
	KerningStart tables.Offset32  // Offset from beginning of this subtable to the start of the kerning array.
	KerningData  []byte           // The data of the subtable, from its start.

	extended   bool // true for a subtable of kerx
	tupleCount int  // the tuple count of a subtable of kerx
}

// convert from non extended to extended
func newKern2(k tables.KernData2) Kern2 {
	return Kern2{
		Left:         tables.AATLoopkup8{AATLoopkup8Data: k.Left},
		Right:        tables.AATLoopkup8{AATLoopkup8Data: k.Right},
		KerningStart: tables.Offset32(k.KerningStart),
		KerningData:  k.KerningData,
	}
}

func newKern2x(k tables.KerxData2, tupleCount int) Kern2 {
	return Kern2{
		Left:         k.Left,
		Right:        k.Right,
		KerningStart: k.KerningStart,
		KerningData:  k.KerningData,
		extended:     true,
		tupleCount:   tupleCount,
	}
}

// KernPair returns the kern value for the given pair, or zero.
//
// okage: in kerx, the sum of the classes is a byte offset from the start of
// the kerning array, rounded down to a value of two bytes, as the commit
// 88e5077a80 of HarfBuzz, in 15.1.0, reads it. In kern, the sum is still a
// byte offset from the start of the subtable, and its distance from the
// kerning array is rounded down to a value of two bytes, as in HarfBuzz.
// go-text read the sum in kerx as an offset from the end of the header of
// the subtable, and required it to be after the start of the kerning
// array. In kern, it read a value at an odd distance (D73).
func (kd Kern2) KernPair(left, right GID) int16 {
	l, _ := kd.Left.Class(tables.GlyphID(left))
	r, _ := kd.Right.Class(tables.GlyphID(right))
	sum := uint64(l) + uint64(r)
	start := uint64(kd.KerningStart)
	var index uint64
	if kd.extended {
		index = start + sum/2*2
	} else if sum < start {
		return 0
	} else {
		index = start + (sum-start)/2*2
	}
	if uint64(len(kd.KerningData)) < index+2 {
		return 0
	}
	kernVal := int16(binary.BigEndian.Uint16(kd.KerningData[index:]))
	// okage: in kerx, a subtable with a tuple count reads the value as a
	// byte offset from the start of the subtable to the values of the
	// tuples, as kerxTupleKern of HarfBuzz 15.1.0. go-text ignored the
	// tuple count (D73).
	if kd.extended && kd.tupleCount != 0 {
		return tables.KerxTupleKern(kd.KerningData, int32(kernVal), kd.tupleCount)
	}
	return kernVal
}

type Kern3 tables.KernData3

// withGlyphs gives the subtable with the classes of nGlyphs glyphs, when it
// has classes. A glyph after the glyphs of the subtable has the class 0.
//
// okage: HarfBuzz 15.1.0 gives the class 0 to such a glyph, and the
// subtable applies to each glyph of the font. go-text applied the subtable
// only to its glyphs (D73).
func (kd Kern3) withGlyphs(nGlyphs int) Kern3 {
	if len(kd.LeftClass) == 0 || nGlyphs <= 0 {
		return kd
	}
	resize := func(classes []uint8) []uint8 {
		out := make([]uint8, nGlyphs)
		copy(out, classes)
		return out
	}
	kd.LeftClass, kd.RightClass = resize(kd.LeftClass), resize(kd.RightClass)
	return kd
}

// KernPair returns the kern value for the given pair, or zero.
//
// okage: as in HarfBuzz 15.1.0, a glyph that the subtable does not hold has
// the class 0, and a class or an index out of range gives no kerning.
// go-text refused the table for a class or an index out of range (D73).
func (kd Kern3) KernPair(left, right GID) int16 {
	var lc, rc int
	if int(left) < len(kd.LeftClass) {
		lc = int(kd.LeftClass[left])
	}
	if int(right) < len(kd.RightClass) {
		rc = int(kd.RightClass[right])
	}
	rightCount := int(kd.RightClassCount)
	if rc >= rightCount || (lc+1)*rightCount > len(kd.KernIndex) {
		return 0
	}
	index := int(kd.KernIndex[lc*rightCount+rc])
	if index >= len(kd.Kernings) {
		return 0
	}
	return kd.Kernings[index]
}

type Kern4 struct {
	// okage: AnchorData holds the data of the actions, from the control
	// point table to the end of the subtable, or to the end of the table
	// for the last subtable. Action reads an action from it, as HarfBuzz
	// 15.1.0 does when it applies the subtable. go-text read the actions
	// into Anchors when it parsed the table (D73).
	AnchorData []byte
	Machine    AATStateTable
	flags      uint32
}

func newKern4(k tables.KerxData4) Kern4 {
	return Kern4{
		Machine:    newAATStableTable(k.AATStateTableExt),
		AnchorData: k.AnchorData,
		flags:      k.Flags,
	}
}

// ActionType returns 0, 1, 2 or 3. An action of the type 3 has no data.
func (k Kern4) ActionType() uint8 {
	const ActionType = 0xC0000000 // A two-bit field containing the action type.
	return uint8(k.flags & ActionType >> 30)
}

// Action returns the values of the action at the index: the two points of
// the action types 0 and 1, or the four coordinates of the action type 2.
// It returns false when the data does not hold the action.
//
// okage: HarfBuzz 15.1.0 reads an action in the same way, and skips the
// transition when the action is not in the subtable (D73).
func (k Kern4) Action(index uint16) (values [4]int16, ok bool) {
	n := 2
	if k.ActionType() == 2 {
		n = 4
	}
	start := int(index) * 2 * n
	if start+2*n > len(k.AnchorData) {
		return values, false
	}
	for i := range n {
		values[i] = int16(binary.BigEndian.Uint16(k.AnchorData[start+2*i:]))
	}
	return values, true
}

type Kern6 tables.KerxData6

// KernPair returns the kern value for the given pair, or zero.
//
// okage: tables.KerxData6.Kerning reads the value as HarfBuzz 15.1.0 does.
// KernPair keeps its low 16 bits, as go-text did (D73).
func (kd Kern6) KernPair(left, right GID) int16 {
	return int16(tables.KerxData6(kd).Kerning(tables.GlyphID(left), tables.GlyphID(right)))
}

// --------------------------------------- state machine ---------------------------------------

// AATStateTable supports both regular and extended AAT state machines
type AATStateTable struct {
	nClass  uint32
	Class   tables.AATLookup
	states  [][]uint16             // each sub array has length stateSize
	entries []tables.AATStateEntry // length is the maximum state + 1
}

func newAATStableTable(k tables.AATStateTableExt) AATStateTable {
	return AATStateTable{
		nClass:  k.StateSize,
		Class:   k.Class,
		states:  k.States,
		entries: k.Entries,
	}
}

// NClasses returns the number of classes of the state table.
//
// okage: harfbuzz reads it to collect the glyphs whose class can start an
// action, as HarfBuzz 15.1.0 does (D73).
func (st *AATStateTable) NClasses() int { return int(st.nClass) }

// GetClass return the class for the given glyph, with the correct default value.
func (st *AATStateTable) GetClass(glyph GID) uint16 {
	if glyph == 0xFFFF { // deleted glyph
		return 2 // class deleted
	}
	c, ok := st.Class.Class(tables.GlyphID(glyph))
	if !ok {
		return 1 // class out of bounds
	}
	return c // class for a state table can't be uint32
}

// GetEntry return the entry for the given state and class,
// and handle invalid values (by returning an empty entry).
func (st *AATStateTable) GetEntry(state, class uint16) tables.AATStateEntry {
	if uint32(class) >= st.nClass {
		class = 1 // class out of bounds
	}
	if int(state) >= len(st.states) {
		return tables.AATStateEntry{}
	}
	entry := st.states[state][class] // access check when parsing
	return st.entries[entry]         // access check when parsing
}
