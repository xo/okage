// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text (D78).
//
// It reads the table MATH, version 1.0 of the OpenType specification:
// https://learn.microsoft.com/typography/opentype/spec/math
// okage does not run the generator binarygen of go-text (D52), so the parser
// is written by hand, in the form of the generated Parse functions. It
// compares every offset, count and coverage index with the length of the
// table, and returns an error for a malformed table. It never panics.

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// MATH is the table MATH, which holds the data for the layout of math.
type MATH struct {
	MajorVersion uint16 // 1
	MinorVersion uint16 // 0

	MathConstants MathConstants
	MathGlyphInfo MathGlyphInfo
	MathVariants  MathVariants
}

// MathValueRecord is a value in font units with an optional device table.
type MathValueRecord struct {
	Value int16 // in font units

	// DeviceOffset is the offset of the device table, from the start of the
	// table that holds the record. It is 0 when the record has none.
	DeviceOffset Offset16

	// Device is the header of the device table. It is zero when the record
	// has none.
	Device MathDevice
}

// MathDevice is the header of a device table or of a VariationIndex table.
// For the format 0x8000, First and Second are the outer and the inner index
// of a delta set in the item variation store of GDEF. For the formats 1, 2
// and 3, they are the first and the last size in pixels per em. The deltas
// of these sizes are not read, because okage does not hint (D78).
type MathDevice struct {
	First, Second uint16
	Format        uint16
}

// VariationIndex returns the index of the delta set of a VariationIndex
// table. It returns false for another format.
func (d MathDevice) VariationIndex() (VariationStoreIndex, bool) {
	if d.Format != 0x8000 {
		return VariationStoreIndex{}, false
	}
	return VariationStoreIndex{DeltaSetOuter: d.First, DeltaSetInner: d.Second}, true
}

// MathConstantsRecordCount is the number of MathValueRecord of the table
// MathConstants: MathLeading to RadicalKernAfterDegree.
const MathConstantsRecordCount = 51

// MathConstants holds the 56 constants of the table MathConstants, in the
// order of the specification.
type MathConstants struct {
	ScriptPercentScaleDown       int16
	ScriptScriptPercentScaleDown int16
	DelimitedSubFormulaMinHeight uint16
	DisplayOperatorMinHeight     uint16

	// Records holds the constants from MathLeading to
	// RadicalKernAfterDegree. Their device offsets are from the start of
	// the table MathConstants.
	Records [MathConstantsRecordCount]MathValueRecord

	RadicalDegreeBottomRaisePercent int16
}

// MathGlyphInfo holds the values of the table MATH for single glyphs. Each
// subtable is empty when the table has none.
type MathGlyphInfo struct {
	MathItalicsCorrectionInfo MathValueTable
	MathTopAccentAttachment   MathValueTable

	// ExtendedShapeCoverage holds the extended shapes. It is nil when the
	// table has none.
	ExtendedShapeCoverage Coverage

	MathKernInfo MathKernInfo
}

// MathValueTable is a coverage with one MathValueRecord for each covered
// glyph, as the tables MathItalicsCorrectionInfo and
// MathTopAccentAttachment. The device offsets are from the start of the
// table.
type MathValueTable struct {
	Coverage Coverage // nil when the table is absent
	Values   []MathValueRecord
}

// Value returns the record of glyph, or false when the table does not cover
// it.
func (t MathValueTable) Value(glyph GlyphID) (MathValueRecord, bool) {
	if t.Coverage == nil {
		return MathValueRecord{}, false
	}
	i, ok := t.Coverage.Index(glyph)
	if !ok || i >= len(t.Values) {
		return MathValueRecord{}, false
	}
	return t.Values[i], true
}

// MathKernInfo holds the math kerning of the glyphs.
type MathKernInfo struct {
	Coverage Coverage // nil when the table is absent
	Records  []MathKernInfoRecord
}

// Record returns the kerning of glyph, or false when the table does not
// cover it.
func (t MathKernInfo) Record(glyph GlyphID) (MathKernInfoRecord, bool) {
	if t.Coverage == nil {
		return MathKernInfoRecord{}, false
	}
	i, ok := t.Coverage.Index(glyph)
	if !ok || i >= len(t.Records) {
		return MathKernInfoRecord{}, false
	}
	return t.Records[i], true
}

// MathKernInfoRecord holds the kerning of the four corners of a glyph, in
// the order top right, top left, bottom right and bottom left.
type MathKernInfoRecord [4]MathKern

// MathKern is the kerning of one corner of a glyph. A kern value applies
// below each correction height, and the last kern value applies above the
// last height. So len(KernValues) is len(CorrectionHeights) + 1, or 0 when
// the corner has no kerning. The device offsets are from the start of the
// table MathKern.
type MathKern struct {
	CorrectionHeights []MathValueRecord
	KernValues        []MathValueRecord
}

// MathVariants holds the size variants and the assemblies of the glyphs.
type MathVariants struct {
	MinConnectorOverlap uint16

	VertGlyphCoverage       Coverage // nil when the table has none
	HorizGlyphCoverage      Coverage // nil when the table has none
	VertGlyphConstructions  []MathGlyphConstruction
	HorizGlyphConstructions []MathGlyphConstruction
}

// Construction returns the construction of glyph in a direction, or false
// when the table has none.
func (t MathVariants) Construction(glyph GlyphID, vertical bool) (MathGlyphConstruction, bool) {
	cov, list := t.HorizGlyphCoverage, t.HorizGlyphConstructions
	if vertical {
		cov, list = t.VertGlyphCoverage, t.VertGlyphConstructions
	}
	if cov == nil {
		return MathGlyphConstruction{}, false
	}
	i, ok := cov.Index(glyph)
	if !ok || i >= len(list) {
		return MathGlyphConstruction{}, false
	}
	return list[i], true
}

// MathGlyphConstruction holds the size variants and the assembly of one
// glyph in one direction.
type MathGlyphConstruction struct {
	GlyphAssembly *GlyphAssembly // nil when the glyph has none
	Variants      []MathGlyphVariantRecord
}

// MathGlyphVariantRecord is one size variant of a glyph.
type MathGlyphVariantRecord struct {
	VariantGlyph       GlyphID
	AdvanceMeasurement uint16 // in font units, in the direction of the construction
}

// GlyphAssembly is the recipe that builds a glyph of any size from parts.
// The device offset of ItalicsCorrection is from the start of the table
// GlyphAssembly.
type GlyphAssembly struct {
	ItalicsCorrection MathValueRecord
	Parts             []GlyphPart // bottom to top, or left to right
}

// GlyphPart is one part of a glyph assembly.
type GlyphPart struct {
	GlyphID              GlyphID
	StartConnectorLength uint16
	EndConnectorLength   uint16
	FullAdvance          uint16
	PartFlags            uint16
}

// Extender reports whether the part can be repeated.
func (p GlyphPart) Extender() bool { return p.PartFlags&1 != 0 }

// errMathBudget is the error of a table whose offsets make the parser
// decode much more data than the table holds.
var errMathBudget = errors.New("reading MATH: the subtables overlap too much")

// mathParser reads one table MATH. Every offset is an absolute offset in
// src. Two offsets can point to the same subtable, or into the middle of
// another subtable. The parser reads a shared subtable once, and it stops
// with an error when the arrays that it decodes exceed budget bytes, so a
// small table cannot ask for a large allocation.
type mathParser struct {
	src           []byte
	budget        int
	kerns         map[int]MathKern
	constructions map[int]MathGlyphConstruction
	assemblies    map[int]*GlyphAssembly
}

// ParseMATH reads the table MATH. It returns the number of bytes of the
// header, as the generated Parse functions do.
func ParseMATH(src []byte) (MATH, int, error) {
	var item MATH
	if L := len(src); L < 10 {
		return item, 0, fmt.Errorf("reading MATH: EOF: expected length: 10, got %d", L)
	}
	item.MajorVersion = binary.BigEndian.Uint16(src[0:])
	item.MinorVersion = binary.BigEndian.Uint16(src[2:])
	if item.MajorVersion != 1 {
		return item, 0, fmt.Errorf("reading MATH: unsupported version %d.%d", item.MajorVersion, item.MinorVersion)
	}
	p := mathParser{
		src:           src,
		budget:        4*len(src) + 4096,
		kerns:         map[int]MathKern{},
		constructions: map[int]MathGlyphConstruction{},
		assemblies:    map[int]*GlyphAssembly{},
	}
	var err error
	if off := int(binary.BigEndian.Uint16(src[4:])); off != 0 {
		if item.MathConstants, err = p.constants(off); err != nil {
			return item, 0, err
		}
	}
	if off := int(binary.BigEndian.Uint16(src[6:])); off != 0 {
		if item.MathGlyphInfo, err = p.glyphInfo(off); err != nil {
			return item, 0, err
		}
	}
	if off := int(binary.BigEndian.Uint16(src[8:])); off != 0 {
		if item.MathVariants, err = p.variants(off); err != nil {
			return item, 0, err
		}
	}
	return item, 10, nil
}

// need returns an error when src holds fewer than n bytes at off.
func (p *mathParser) need(table string, off, n int) error {
	if off < 0 || n < 0 || off > len(p.src) || len(p.src)-off < n {
		return fmt.Errorf("reading %s: EOF: expected length: %d, got %d", table, off+n, len(p.src))
	}
	return nil
}

// spend takes n bytes from the budget.
func (p *mathParser) spend(n int) error {
	if n > p.budget {
		return errMathBudget
	}
	p.budget -= n
	return nil
}

func (p *mathParser) u16(off int) uint16 { return binary.BigEndian.Uint16(p.src[off:]) }

// offset returns the absolute position of the offset at off, from base, or
// 0 for a null offset.
func (p *mathParser) offset(base, off int) int {
	o := int(p.u16(off))
	if o == 0 {
		return 0
	}
	return base + o
}

// record reads the MathValueRecord at off, whose device offset is from the
// start of the table at base. The caller has checked the four bytes of the
// record.
func (p *mathParser) record(table string, base, off int) (MathValueRecord, error) {
	rec := MathValueRecord{
		Value:        int16(p.u16(off)),
		DeviceOffset: Offset16(p.u16(off + 2)),
	}
	if rec.DeviceOffset == 0 {
		return rec, nil
	}
	dev := base + int(rec.DeviceOffset)
	if err := p.need(table+" device table", dev, 6); err != nil {
		return rec, err
	}
	rec.Device = MathDevice{First: p.u16(dev), Second: p.u16(dev + 2), Format: p.u16(dev + 4)}
	return rec, nil
}

// records reads count records from off, whose device offsets are from the
// start of the table at base.
func (p *mathParser) records(table string, base, off, count int) ([]MathValueRecord, error) {
	if err := p.need(table, off, 4*count); err != nil {
		return nil, err
	}
	if err := p.spend(4 * count); err != nil {
		return nil, err
	}
	out := make([]MathValueRecord, count) // allocation guarded by the previous checks
	for i := range out {
		var err error
		if out[i], err = p.record(table, base, off+4*i); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// coverage reads the coverage table at off.
func (p *mathParser) coverage(table string, off int) (Coverage, error) {
	if err := p.need(table, off, 0); err != nil {
		return nil, err
	}
	cov, _, err := ParseCoverage(p.src[off:])
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", table, err)
	}
	return cov, nil
}

func (p *mathParser) constants(off int) (MathConstants, error) {
	const size = 8 + 4*MathConstantsRecordCount + 2
	var item MathConstants
	if err := p.need("MathConstants", off, size); err != nil {
		return item, err
	}
	item.ScriptPercentScaleDown = int16(p.u16(off))
	item.ScriptScriptPercentScaleDown = int16(p.u16(off + 2))
	item.DelimitedSubFormulaMinHeight = p.u16(off + 4)
	item.DisplayOperatorMinHeight = p.u16(off + 6)
	for i := range item.Records {
		var err error
		if item.Records[i], err = p.record("MathConstants", off, off+8+4*i); err != nil {
			return item, err
		}
	}
	item.RadicalDegreeBottomRaisePercent = int16(p.u16(off + size - 2))
	return item, nil
}

func (p *mathParser) glyphInfo(off int) (MathGlyphInfo, error) {
	var item MathGlyphInfo
	if err := p.need("MathGlyphInfo", off, 8); err != nil {
		return item, err
	}
	var err error
	if sub := p.offset(off, off); sub != 0 {
		if item.MathItalicsCorrectionInfo, err = p.valueTable("MathItalicsCorrectionInfo", sub); err != nil {
			return item, err
		}
	}
	if sub := p.offset(off, off+2); sub != 0 {
		if item.MathTopAccentAttachment, err = p.valueTable("MathTopAccentAttachment", sub); err != nil {
			return item, err
		}
	}
	if sub := p.offset(off, off+4); sub != 0 {
		if item.ExtendedShapeCoverage, err = p.coverage("ExtendedShapeCoverage", sub); err != nil {
			return item, err
		}
	}
	if sub := p.offset(off, off+6); sub != 0 {
		if item.MathKernInfo, err = p.kernInfo(sub); err != nil {
			return item, err
		}
	}
	return item, nil
}

// valueTable reads a MathItalicsCorrectionInfo or a MathTopAccentAttachment.
func (p *mathParser) valueTable(table string, off int) (MathValueTable, error) {
	var item MathValueTable
	if err := p.need(table, off, 4); err != nil {
		return item, err
	}
	cov := p.offset(off, off)
	if cov == 0 {
		return item, nil // a table without a coverage covers no glyph
	}
	var err error
	if item.Coverage, err = p.coverage(table, cov); err != nil {
		return item, err
	}
	item.Values, err = p.records(table, off, off+4, int(p.u16(off+2)))
	return item, err
}

func (p *mathParser) kernInfo(off int) (MathKernInfo, error) {
	var item MathKernInfo
	if err := p.need("MathKernInfo", off, 4); err != nil {
		return item, err
	}
	cov := p.offset(off, off)
	if cov == 0 {
		return item, nil // a table without a coverage covers no glyph
	}
	var err error
	if item.Coverage, err = p.coverage("MathKernInfo", cov); err != nil {
		return item, err
	}
	count := int(p.u16(off + 2))
	if err := p.need("MathKernInfo", off+4, 8*count); err != nil {
		return item, err
	}
	if err := p.spend(8 * count); err != nil {
		return item, err
	}
	item.Records = make([]MathKernInfoRecord, count) // allocation guarded by the previous checks
	for i := range item.Records {
		for c := range item.Records[i] {
			sub := p.offset(off, off+4+8*i+2*c)
			if sub == 0 {
				continue
			}
			if item.Records[i][c], err = p.kern(sub); err != nil {
				return item, err
			}
		}
	}
	return item, nil
}

func (p *mathParser) kern(off int) (MathKern, error) {
	if k, ok := p.kerns[off]; ok {
		return k, nil
	}
	var item MathKern
	if err := p.need("MathKern", off, 2); err != nil {
		return item, err
	}
	count := int(p.u16(off))
	all, err := p.records("MathKern", off, off+2, 2*count+1)
	if err != nil {
		return item, err
	}
	item.CorrectionHeights, item.KernValues = all[:count:count], all[count:]
	p.kerns[off] = item
	return item, nil
}

func (p *mathParser) variants(off int) (MathVariants, error) {
	var item MathVariants
	if err := p.need("MathVariants", off, 10); err != nil {
		return item, err
	}
	item.MinConnectorOverlap = p.u16(off)
	vertCov, horizCov := p.offset(off, off+2), p.offset(off, off+4)
	vertCount, horizCount := int(p.u16(off+6)), int(p.u16(off+8))
	if err := p.need("MathVariants", off+10, 2*(vertCount+horizCount)); err != nil {
		return item, err
	}
	var err error
	if vertCov != 0 {
		if item.VertGlyphCoverage, err = p.coverage("MathVariants", vertCov); err != nil {
			return item, err
		}
	} else {
		vertCount = 0
	}
	if horizCov != 0 {
		if item.HorizGlyphCoverage, err = p.coverage("MathVariants", horizCov); err != nil {
			return item, err
		}
	} else {
		horizCount = 0
	}
	if item.VertGlyphConstructions, err = p.constructionList(off, off+10, vertCount); err != nil {
		return item, err
	}
	item.HorizGlyphConstructions, err = p.constructionList(off, off+10+2*int(p.u16(off+6)), horizCount)
	return item, err
}

// constructionList reads count offsets to MathGlyphConstruction tables at
// off, from the start of the table MathVariants at base.
func (p *mathParser) constructionList(base, off, count int) ([]MathGlyphConstruction, error) {
	if err := p.spend(2 * count); err != nil {
		return nil, err
	}
	out := make([]MathGlyphConstruction, count) // allocation guarded by the checks of the caller
	for i := range out {
		sub := p.offset(base, off+2*i)
		if sub == 0 {
			continue
		}
		var err error
		if out[i], err = p.construction(sub); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (p *mathParser) construction(off int) (MathGlyphConstruction, error) {
	if c, ok := p.constructions[off]; ok {
		return c, nil
	}
	var item MathGlyphConstruction
	if err := p.need("MathGlyphConstruction", off, 4); err != nil {
		return item, err
	}
	var err error
	if sub := p.offset(off, off); sub != 0 {
		if item.GlyphAssembly, err = p.assembly(sub); err != nil {
			return item, err
		}
	}
	count := int(p.u16(off + 2))
	if err := p.need("MathGlyphConstruction", off+4, 4*count); err != nil {
		return item, err
	}
	if err := p.spend(4 * count); err != nil {
		return item, err
	}
	item.Variants = make([]MathGlyphVariantRecord, count) // allocation guarded by the previous checks
	for i := range item.Variants {
		item.Variants[i] = MathGlyphVariantRecord{
			VariantGlyph:       p.u16(off + 4 + 4*i),
			AdvanceMeasurement: p.u16(off + 6 + 4*i),
		}
	}
	p.constructions[off] = item
	return item, nil
}

func (p *mathParser) assembly(off int) (*GlyphAssembly, error) {
	if a, ok := p.assemblies[off]; ok {
		return a, nil
	}
	if err := p.need("GlyphAssembly", off, 6); err != nil {
		return nil, err
	}
	item := new(GlyphAssembly)
	var err error
	if item.ItalicsCorrection, err = p.record("GlyphAssembly", off, off); err != nil {
		return nil, err
	}
	count := int(p.u16(off + 4))
	if err := p.need("GlyphAssembly", off+6, 10*count); err != nil {
		return nil, err
	}
	if err := p.spend(10 * count); err != nil {
		return nil, err
	}
	item.Parts = make([]GlyphPart, count) // allocation guarded by the previous checks
	for i := range item.Parts {
		at := off + 6 + 10*i
		item.Parts[i] = GlyphPart{
			GlyphID:              p.u16(at),
			StartConnectorLength: p.u16(at + 2),
			EndConnectorLength:   p.u16(at + 4),
			FullAdvance:          p.u16(at + 6),
			PartFlags:            p.u16(at + 8),
		}
	}
	p.assemblies[off] = item
	return item, nil
}
