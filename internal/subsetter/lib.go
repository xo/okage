// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"slices"
	"unicode/utf8"

	"github.com/xo/okage"
)

// This file ports subsetter src/lib.rs, without the package comment, which
// is in doc.go. The tables and the functions of the units that are not ported
// yet are reached through the seams at the end of this file.

// Variation is a pair (Tag, f32) of the coordinates of
// subset_with_variations in subsetter src/lib.rs: the tag of an axis and a
// value in the coordinates of the user, such as 700 for the axis wght.
type Variation struct {
	Tag   Tag
	Value float32
}

// Subset is subset in subsetter src/lib.rs.
//
// Subset the font face to include only the necessary glyphs and tables.
//
//   - The data must be in the OpenType font format.
//   - The index is only relevant if the data contains a font collection
//     (.ttc or .otc file). Otherwise, it should be 0.
//
// CFF2 fonts are not supported.
//
// The error wraps a sentinel error of okage, such as
// okage.ErrSubsetMalformedFont.
func Subset(data []byte, index uint32, mapper *GlyphRemapper) ([]byte, error) {
	return subsetInner(data, index, nil, false, mapper)
}

// SubsetWithVariations is subset_with_variations in subsetter src/lib.rs.
//
// Subset the font face to include only the necessary glyphs and tables,
// instantiated to the given variation coordinates.
//
// This does the same as Subset, but allows you to specify variation
// coordinates.
//
// It is important to note that if you pass a CFF2 font, it will be converted
// to a TrueType font.
func SubsetWithVariations(data []byte, index uint32, variationCoordinates []Variation, mapper *GlyphRemapper) ([]byte, error) {
	return subsetInner(data, index, variationCoordinates, true, mapper)
}

// subsetInner is subset_inner in subsetter src/lib.rs.
func subsetInner(data []byte, index uint32, variationCoordinates []Variation, allowCFF2 bool, mapper *GlyphRemapper) ([]byte, error) {
	mapper = mapper.Clone()
	ctx, err := prepareContext(data, index, variationCoordinates, allowCFF2, mapper)
	if err != nil {
		return nil, err
	}
	return subsetTables(ctx)
}

// prepareContext is prepare_context in subsetter src/lib.rs, with the
// feature variable-fonts of upstream, which is on by default.
func prepareContext(data []byte, index uint32, variationCoordinates []Variation, allowCFF2 bool, gidRemapper *GlyphRemapper) (*context, error) {
	face, err := parse(data, index)
	if err != nil {
		return nil, err
	}
	var flavor fontFlavor
	switch {
	case face.hasTable(tagGLYF):
		flavor = fontFlavorTrueType
	case face.hasTable(tagCFF):
		flavor = fontFlavorCFF
	case face.hasTable(tagCFF2):
		// Only works if `variable-fonts` feature is enabled, as we need skrifa
		// so we can convert CFF2 into TrueType.
		if !allowCFF2 {
			return nil, errUnimplemented
		}
		flavor = fontFlavorCFF2
	default:
		return nil, errUnknownKind
	}

	if flavor == fontFlavorTrueType {
		if err := closure(face, gidRemapper); err != nil {
			return nil, err
		}
	}

	// For CFF, we _always_ want to do normal subsetting, since CFF cannot have
	// variations. For TrueType, we prefer normal subsetting in case the
	// requested variation coordinates resolve to the default location. If we
	// do have variations, we use `skrifa` to instance. For CFF2, we _always_
	// use `skrifa` to instance.
	var interj interjector
	if flavor != fontFlavorCFF {
		s, ok := newSkrifaInterjector(data, index, variationCoordinates)
		if !ok {
			return nil, errMalformedFont
		}
		if flavor != fontFlavorTrueType || !s.isDefaultLocation() {
			interj = interjector{skrifa: s}
		}
	}

	return &context{
		face:        face,
		mapper:      gidRemapper,
		interjector: interj,
		flavor:      flavor,
	}, nil
}

// subsetTables is _subset in subsetter src/lib.rs.
func subsetTables(ctx *context) ([]byte, error) {
	// See here for the required tables:
	// https://learn.microsoft.com/en-us/typography/opentype/spec/otff#required-tables
	// but some of those are not strictly needed according to the PDF
	// specification.

	// Of the above tables, we are not including the following ones:
	// - CFF2: Since we don't support CFF2
	// - VORG: PDF doesn't use that table.
	// - CMAP: CID fonts in PDF define their own cmaps, so we don't need to
	//   include them in the font.
	// - GASP: Not mandated by PDF specification, and ghostscript also seems to
	//   exclude them.
	// - OS2: Not mandated by PDF specification, and ghostscript also seems to
	//   exclude them.

	var tags []Tag
	switch ctx.flavor {
	case fontFlavorTrueType:
		// LOCA will be handled by GLYF
		tags = append(tags, tagGLYF)

		// Only copy hinting tables if we don't interject because skrifa will
		// discard hinting information.
		// TODO: Add a test for this.
		if !ctx.interjector.isSkrifa() {
			tags = append(tags, tagCVT, tagFPGM, tagPREP) // won't be subsetted.
		}
	case fontFlavorCFF:
		tags = append(tags, tagCFF)
	case fontFlavorCFF2:
		tags = append(tags, tagCFF2)
	}

	// Required tables.
	// NAME is also not strictly needed, and ghostscript removes it when
	// subsetting. However, it contains copyright information which probably
	// should not be removed... Even though it can free up a lot of space for
	// some fonts.
	tags = append(tags, tagHEAD, tagHMTX, tagMAXP, tagNAME, tagPOST)
	for _, tag := range tags {
		if err := ctx.process(tag); err != nil {
			return nil, err
		}
	}

	return construct(ctx), nil
}

// parse is parse in subsetter src/lib.rs. It parses a font face from
// OpenType data.
func parse(data []byte, index uint32) (*face, error) {
	r := newReader(data)
	kind, ok := readFontKind(&r)
	if !ok {
		return nil, errUnknownKind
	}

	// Parse font collection header if necessary.
	if kind == fontKindCollection {
		// Rust computes the offset in a usize, which does not overflow on a
		// machine of 64 bits.
		at := uint64(12) + 4*uint64(index)
		if at > math.MaxInt {
			return nil, errMalformedFont
		}
		r = newReaderAt(data, int(at))
		offset, ok := r.readU32()
		if !ok || uint64(offset) > uint64(len(data)) {
			return nil, errMalformedFont
		}
		subdata := data[offset:]
		r = newReader(subdata)
		kind, ok = readFontKind(&r)
		if !ok {
			return nil, errMalformedFont
		}

		// Cannot have nested collection
		if kind == fontKindCollection {
			return nil, errMalformedFont
		}
	}

	// Read number of table records.
	count, ok := r.readU16()
	if !ok {
		return nil, errMalformedFont
	}
	for range 3 {
		if _, ok := r.readU16(); !ok {
			return nil, errMalformedFont
		}
	}

	// Read table records. Each record that the loop appends is in data, so
	// the slice grows with the data and not with the count.
	var records []tableRecord
	for range count {
		record, ok := readTableRecord(&r)
		if !ok {
			return nil, errMalformedFont
		}
		records = append(records, record)
	}

	return &face{data: data, records: records}, nil
}

// construct is construct in subsetter src/lib.rs. It constructs a brand-new
// font.
func construct(ctx *context) []byte {
	slices.SortStableFunc(ctx.tables, func(a, b table) int {
		return a.tag.compare(b.tag)
	})

	w := newWriter()
	ctx.flavor.write(w)

	// Write table directory.
	count := uint16(len(ctx.tables))
	// Upstream computes (count as f32).log2().floor() as u16. For a count of
	// 0, the cast gives 0. For any other count, it is the index of the
	// highest bit that is set.
	entrySelector := uint16(0)
	if count > 0 {
		entrySelector = uint16(bits.Len16(count) - 1)
	}
	searchRange := uint16(1) << entrySelector * 16
	rangeShift := count*16 - searchRange
	w.writeU16(count)
	w.writeU16(searchRange)
	w.writeU16(entrySelector)
	w.writeU16(rangeShift)

	// This variable will hold the offset to the checksum adjustment field in
	// the head table, which we'll have to write in the end (after
	// checksumming the whole font).
	checksumAdjustmentOffset := -1

	// Write table records.
	offset := 12 + len(ctx.tables)*16
	for i := range ctx.tables {
		t := &ctx.tables[i]
		if t.tag == tagHEAD {
			// Zero out checksum field in head table. The head table has
			// already been subset and is guaranteed to have the necessary
			// minimum size.
			clear(t.data[8:12])
			checksumAdjustmentOffset = offset + 8
		}

		length := len(t.data)
		tableRecord{
			tag:      t.tag,
			checksum: checksum(t.data),
			offset:   uint32(offset),
			length:   uint32(length),
		}.write(w)

		// Increase offset, plus padding zeros to align to 4 bytes.
		offset += length
		for offset%4 != 0 {
			offset++
		}
	}

	// Write tables.
	for _, t := range ctx.tables {
		// Write data plus padding zeros to align to 4 bytes.
		w.extend(t.data)
		w.align(4)
	}

	// Write checksum adjustment field in head table.
	data := w.finish()
	if i := checksumAdjustmentOffset; i >= 0 {
		sum := checksum(data)
		val := 0xB1B0AFBA - sum
		data[i] = byte(val >> 24)
		data[i+1] = byte(val >> 16)
		data[i+2] = byte(val >> 8)
		data[i+3] = byte(val)
	}

	return data
}

// checksum is checksum in subsetter src/lib.rs. It calculates a checksum
// over the sliced data as a sum of u32s. If the data length is not a multiple
// of four, it is treated as if padded with zero to a length that is a
// multiple of four.
func checksum(data []byte) uint32 {
	var sum uint32
	for len(data) > 0 {
		var bytes [4]byte
		n := copy(bytes[:], data)
		data = data[n:]
		sum += uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
	}
	return sum
}

// table is a pair (Tag, Cow<[u8]>) of the subsetted tables of Context in
// subsetter src/lib.rs. Its data can be a part of the data of the font,
// except the data of the head table, which head.go copies.
type table struct {
	tag  Tag
	data []byte
}

// context is Context in subsetter src/lib.rs. The subsetting context.
type context struct {
	// face is the original face.
	face *face
	// mapper is a map from old gids to new gids, and the reverse.
	mapper *GlyphRemapper
	// flavor is the font flavor.
	flavor fontFlavor
	// tables holds the subsetted tables.
	tables []table
	// interjector is an object that can _interject_ data during the
	// subsetting process. Normally, when subsetting CFF/TrueType fonts, we
	// will simply read the corresponding data from the old font and rewrite
	// it to the new font, for example when writing the `hmtx` table.
	//
	// However, in case we are subsetting with variation coordinates, we
	// instead rely on skrifa to apply the variation coordinates and interject
	// that data during the subsetting process instead of using the data from
	// the old font.
	interjector interjector
	// customMaxpData is custom data that should be used for writing the
	// `maxp` table. Only needed for CFF2, where we need to synthesize a V1
	// table after converting. Nil is None.
	customMaxpData *maxpData
	// customHmtxData is the custom data of the hmtx table, as pairs of an
	// advance and a left side bearing. Nil is None.
	customHmtxData []hmtxMetric
	// longLoca tells whether the long loca format was chosen.
	longLoca bool
}

// expectTable is Context::expect_table in subsetter src/lib.rs. It expects
// a table.
func (ctx *context) expectTable(tag Tag) ([]byte, bool) {
	return ctx.face.table(tag)
}

// process is Context::process in subsetter src/lib.rs. It processes a
// table.
func (ctx *context) process(tag Tag) error {
	data, ok := ctx.face.table(tag)
	if !ok {
		return nil
	}

	switch tag {
	case tagGLYF:
		return subsetGlyf(ctx)
	case tagLOCA:
		panic("handled by glyf")
	case tagCFF:
		if subsetCFFTable == nil {
			return notPorted("the CFF table")
		}
		return subsetCFFTable(ctx)
	case tagCFF2:
		if subsetCFF2Table == nil {
			return notPorted("the CFF2 table")
		}
		return subsetCFF2Table(ctx)
	case tagHEAD:
		return subsetHead(ctx)
	case tagHHEA:
		panic("handled by hmtx")
	case tagHMTX:
		return subsetHmtx(ctx)
	case tagPOST:
		return subsetPost(ctx)
	case tagMAXP:
		return subsetMaxp(ctx)
	case tagNAME:
		return subsetName(ctx)
	}
	ctx.push(tag, data)
	return nil
}

// push is Context::push in subsetter src/lib.rs. It pushes a subsetted
// table. Upstream asserts in a debug build that the table is not there yet.
func (ctx *context) push(tag Tag, data []byte) {
	ctx.tables = append(ctx.tables, table{tag: tag, data: data})
}

// face is Face in subsetter src/lib.rs. A font face with OpenType tables.
type face struct {
	data    []byte
	records []tableRecord
}

// table is Face::table in subsetter src/lib.rs. It returns the data of the
// table with the tag, and false when the font has no such table or the
// record of the table is out of the bounds of the data.
func (f *face) table(tag Tag) ([]byte, bool) {
	i, ok := binarySearchRecords(f.records, tag)
	if !ok {
		return nil, false
	}
	record := f.records[i]
	start := uint64(record.offset)
	end := start + uint64(record.length)
	if end > uint64(len(f.data)) {
		return nil, false
	}
	return f.data[start:end], true
}

// hasTable is face.table(tag).is_some() in subsetter src/lib.rs.
func (f *face) hasTable(tag Tag) bool {
	_, ok := f.table(tag)
	return ok
}

// binarySearchRecords is records.binary_search_by(|record|
// record.tag.cmp(&tag)) in Face::table of subsetter src/lib.rs. It follows
// the algorithm of slice::binary_search_by of the Rust standard library, so
// that a directory whose records are not sorted, or that holds a tag twice,
// gives the record that upstream gives.
func binarySearchRecords(records []tableRecord, tag Tag) (int, bool) {
	size := len(records)
	if size == 0 {
		return 0, false
	}
	base := 0
	for size > 1 {
		half := size / 2
		mid := base + half
		if records[mid].tag.compare(tag) <= 0 {
			base = mid
		}
		size -= half
	}
	if records[base].tag.compare(tag) == 0 {
		return base, true
	}
	return 0, false
}

// fontKind is FontKind in subsetter src/lib.rs. It tells whether the font is
// a font collection or a single font.
type fontKind int

const (
	// fontKindCollection is a font collection.
	fontKindCollection fontKind = iota
	// fontKindSingle is a single font.
	fontKindSingle
)

// readFontKind is Readable::read for FontKind in subsetter src/lib.rs.
func readFontKind(r *reader) (fontKind, bool) {
	v, ok := r.readU32()
	if !ok {
		return 0, false
	}
	switch v {
	// TrueType
	case 0x00010000, 0x74727565:
		return fontKindSingle, true
	// CFF
	case 0x4F54544F:
		return fontKindSingle, true
	case 0x74746366:
		return fontKindCollection, true
	}
	return 0, false
}

// fontFlavor is FontFlavor in subsetter src/lib.rs. The flavor of outlines
// used by the font.
type fontFlavor int

const (
	// fontFlavorTrueType is TrueType fonts using the `glyf` table.
	fontFlavorTrueType fontFlavor = iota
	// fontFlavorCFF is CFF fonts using the `CFF` table.
	fontFlavorCFF
	// fontFlavorCFF2 is CFF2 fonts using the `CFF2` table.
	fontFlavorCFF2
)

// write is Writeable::write for FontFlavor in subsetter src/lib.rs.
func (f fontFlavor) write(w *writer) {
	switch f {
	// Important note: This is the magic for TrueType and not CFF2. However,
	// CFF2 fonts will be converted to TrueType as part of the subsetting
	// process, hence we write the same magic.
	case fontFlavorTrueType, fontFlavorCFF2:
		w.writeU32(0x00010000)
	case fontFlavorCFF:
		w.writeU32(0x4F54544F)
	}
}

// Tag is Tag in subsetter src/lib.rs. A 4-byte OpenType tag.
type Tag [4]byte

// NewTag is Tag::new in subsetter src/lib.rs. It creates a new tag.
func NewTag(tag [4]byte) Tag {
	return Tag(tag)
}

// Get is Tag::get in subsetter src/lib.rs. It returns the value of the tag.
func (t Tag) Get() [4]byte {
	return t
}

// ParseTag is the impl FromStr of Tag in subsetter src/lib.rs. It tries to
// create a new tag from a string, and returns false if the string is not 4
// bytes in size.
func ParseTag(s string) (Tag, bool) {
	if len(s) != 4 {
		return Tag{}, false
	}
	return Tag{s[0], s[1], s[2], s[3]}, true
}

// compare is the derived Ord of Tag in subsetter src/lib.rs. It compares
// the bytes of two tags in order.
func (t Tag) compare(other Tag) int {
	for i := range t {
		if c := cmp.Compare(t[i], other[i]); c != 0 {
			return c
		}
	}
	return 0
}

// The tags of the tables of subsetter src/lib.rs. Go has no constant array,
// so they are variables, which nothing changes. Upstream also declares the
// tags of cmap, OS/2, gasp, VORG and of the bitmap and color tables, and uses
// none of them, so the port leaves them out.
var (
	// General tables.
	tagHEAD = Tag{'h', 'e', 'a', 'd'}
	tagHHEA = Tag{'h', 'h', 'e', 'a'}
	tagHMTX = Tag{'h', 'm', 't', 'x'}
	tagMAXP = Tag{'m', 'a', 'x', 'p'}
	tagNAME = Tag{'n', 'a', 'm', 'e'}
	tagPOST = Tag{'p', 'o', 's', 't'}

	// TrueType.
	tagGLYF = Tag{'g', 'l', 'y', 'f'}
	tagLOCA = Tag{'l', 'o', 'c', 'a'}
	tagPREP = Tag{'p', 'r', 'e', 'p'}
	tagFPGM = Tag{'f', 'p', 'g', 'm'}
	tagCVT  = Tag{'c', 'v', 't', ' '}

	// CFF.
	tagCFF  = Tag{'C', 'F', 'F', ' '}
	tagCFF2 = Tag{'C', 'F', 'F', '2'}
)

// readTag is Readable::read for Tag in subsetter src/lib.rs.
func readTag(r *reader) (Tag, bool) {
	b, ok := r.readBytes(4)
	if !ok {
		return Tag{}, false
	}
	return Tag(b), true
}

// write is Writeable::write for Tag in subsetter src/lib.rs.
func (t Tag) write(w *writer) {
	w.extend(t[:])
}

// String is the impl Display of Tag in subsetter src/lib.rs. It gives the
// tag as text, or "..." when the tag is not UTF-8.
func (t Tag) String() string {
	if !utf8.Valid(t[:]) {
		return "..."
	}
	return string(t[:])
}

// GoString is the impl Debug of Tag in subsetter src/lib.rs.
func (t Tag) GoString() string {
	return fmt.Sprintf("Tag(%s)", t)
}

// tableRecord is TableRecord in subsetter src/lib.rs. It locates a table in
// the font file.
type tableRecord struct {
	tag      Tag
	checksum uint32
	offset   uint32
	length   uint32
}

// readTableRecord is Readable::read for TableRecord in subsetter
// src/lib.rs.
func readTableRecord(r *reader) (tableRecord, bool) {
	var t tableRecord
	var ok bool
	if t.tag, ok = readTag(r); !ok {
		return t, false
	}
	if t.checksum, ok = r.readU32(); !ok {
		return t, false
	}
	if t.offset, ok = r.readU32(); !ok {
		return t, false
	}
	if t.length, ok = r.readU32(); !ok {
		return t, false
	}
	return t, true
}

// write is Writeable::write for TableRecord in subsetter src/lib.rs.
func (t tableRecord) write(w *writer) {
	t.tag.write(w)
	w.writeU32(t.checksum)
	w.writeU32(t.offset)
	w.writeU32(t.length)
}

// f2Dot14 is F2Dot14 in subsetter src/lib.rs. A signed 16-bit fixed-point
// number.
type f2Dot14 uint16

// readF2Dot14 is Readable::read for F2Dot14 in subsetter src/lib.rs.
func readF2Dot14(r *reader) (f2Dot14, bool) {
	v, ok := r.readU16()
	return f2Dot14(v), ok
}

// write is Writeable::write for F2Dot14 in subsetter src/lib.rs.
func (f f2Dot14) write(w *writer) {
	w.writeU16(uint16(f))
}

// The cases of the enum Error of subsetter src/lib.rs. Each one is a
// sentinel error of okage (D21).
const (
	// errUnknownKind is Error::UnknownKind. The file contains an unknown
	// kind of font.
	errUnknownKind = okage.ErrSubsetUnknownKind
	// errMalformedFont is Error::MalformedFont. The font is malformed (or
	// there is a bug in the font parsing logic).
	errMalformedFont = okage.ErrSubsetMalformedFont
	// errUnimplemented is Error::Unimplemented. The font relies on an
	// unimplemented feature, and thus the subsetting process couldn't be
	// completed.
	errUnimplemented = okage.ErrSubsetUnimplemented
	// errSubset is Error::SubsetError. An unexpected error occurred when
	// subsetting the font. Indicates that there is a logical bug in the
	// subsetter.
	errSubset = okage.ErrSubsetFailed
	// errOverflow is Error::OverflowError. An overflow occurred during the
	// computation. Could be either an issue with the font itself, or a bug
	// in the subsetter logic.
	errOverflow = okage.ErrSubsetOverflow
	// errCFF is Error::CFFError. An error occurred while processing the CFF
	// table.
	errCFF = okage.ErrSubsetCFF
)

// The seams of the units of phase 13 that are not ported yet. A seam is nil
// until the file of its unit sets it in a function init. While a seam is nil,
// the code that calls it returns an error that wraps
// okage.ErrSubsetUnimplemented. The package comment in doc.go describes them.
var (
	// subsetCFFTable is cff::subset in subsetter src/cff/mod.rs. cff.go sets
	// it.
	subsetCFFTable func(ctx *context) error
	// subsetCFF2Table is cff2::subset in subsetter src/cff2.rs. cff2.go sets
	// it.
	subsetCFF2Table func(ctx *context) error
	// interjectGlyph is SkrifaInterjector::interject in subsetter
	// src/interjector.rs. The file of unit 4 that ports it sets it. It
	// returns the advance, the left side bearing and the glyph description
	// in the glyf format, and false where upstream returns None.
	interjectGlyph func(s *skrifaInterjector, maxp *maxpData, glyph uint16) (advance uint16, lsb int16, data []byte, ok bool)
)

// notPorted returns the error of a seam that is nil.
func notPorted(what string) error {
	return fmt.Errorf("%w: %s is not ported yet", errUnimplemented, what)
}
