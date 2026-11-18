// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"fmt"
	"math"
	"slices"
)

// This file ports subsetter src/cff/mod.rs. The files cff_*.go port the
// other files of src/cff, one Go file for each Rust file, such as
// cff_dict_top_dict.go for src/cff/dict/top_dict.rs. Unit 3 of phase 13
// ports them, and the function init of this file sets the seam
// subsetCFFTable of lib.go.

func init() {
	subsetCFFTable = subsetCFF
}

// fontKindCFF is FontKind in subsetter src/cff/mod.rs. It holds the
// metadata of a CID-keyed font when cid is not nil, and the metadata of a
// SID-keyed font when it is nil.
type fontKindCFF struct {
	sid sidMetadata
	cid *cidMetadata
}

// tableCFF is Table in subsetter src/cff/mod.rs.
type tableCFF struct {
	names       []byte
	topDictData topDictData
	strings     index
	globalSubrs index
	charStrings index
	fontKind    fontKindCFF
}

// deferredOffset is DeferredOffset in subsetter src/cff/mod.rs. An offset
// that needs to be written after the whole font has been written. location
// indicates where in the buffer the offset needs to be written to and value
// indicates the value of the offset.
type deferredOffset struct {
	location int
	value    integerNumber
}

// dummyValue is DUMMY_VALUE in subsetter src/cff/mod.rs.
const dummyValue integerNumber = 0

// dummyOffset is DUMMY_OFFSET in subsetter src/cff/mod.rs. This represents
// a not-existing offset.
var dummyOffset = deferredOffset{location: 0, value: dummyValue}

// updateValue is DeferredOffset::update_value in subsetter
// src/cff/mod.rs.
func (o *deferredOffset) updateValue(value int) error {
	if value > math.MaxInt32 {
		return errOverflow
	}
	o.value = integerNumber(value)
	return nil
}

// adjustLocation is DeferredOffset::adjust_location in subsetter
// src/cff/mod.rs. It adjusts the location of an offset, if it has been set
// (i.e. it's not a dummy offset).
func (o *deferredOffset) adjustLocation(delta int) {
	if *o != dummyOffset {
		o.location += delta
	}
}

// updateLocation is DeferredOffset::update_location in subsetter
// src/cff/mod.rs.
func (o *deferredOffset) updateLocation(location int) {
	o.location = location
}

// writeInto is DeferredOffset::write_into in subsetter src/cff/mod.rs. It
// writes the deferred offset into a buffer.
func (o deferredOffset) writeInto(buffer []byte) error {
	w := newWriterWithCapacity(5)
	// Always write using 5 bytes, to prevent its size from changing.
	o.value.writeAs5Bytes(w)
	encoded := w.finish()
	if o.location > len(buffer)-5 {
		return errSubset
	}

	copy(buffer[o.location:o.location+5], encoded)

	return nil
}

// offsetsCFF is Offsets in subsetter src/cff/mod.rs. It keeps track of the
// offsets that need to be written in the font.
type offsetsCFF struct {
	// charsetOffset is the offset of the charset data.
	charsetOffset deferredOffset
	// charStringsOffset is the offset of the charstrings data.
	charStringsOffset deferredOffset
	// privateDictsLens holds the lengths of the private dicts (not strictly
	// an offset, but we still use it for simplicity).
	privateDictsLens []deferredOffset
	// privateDictsOffsets holds the offsets of the private dicts.
	privateDictsOffsets []deferredOffset
	// fdArrayOffset is the offset of the fd array.
	fdArrayOffset deferredOffset
	// fdSelectOffset is the offset of the fd select.
	fdSelectOffset deferredOffset
}

// newOffsetsCID is Offsets::new_cid in subsetter src/cff/mod.rs.
func newOffsetsCID(numFontDicts uint8) *offsetsCFF {
	return &offsetsCFF{
		charStringsOffset:   dummyOffset,
		charsetOffset:       dummyOffset,
		privateDictsLens:    slices.Repeat([]deferredOffset{dummyOffset}, int(numFontDicts)),
		privateDictsOffsets: slices.Repeat([]deferredOffset{dummyOffset}, int(numFontDicts)),
		fdSelectOffset:      dummyOffset,
		fdArrayOffset:       dummyOffset,
	}
}

// newOffsetsSID is Offsets::new_sid in subsetter src/cff/mod.rs.
func newOffsetsSID() *offsetsCFF {
	return newOffsetsCID(1)
}

// subsetCFF is subset in subsetter src/cff/mod.rs.
func subsetCFF(ctx *context) error {
	table, err := parseTableCFF(ctx)
	if err != nil {
		return err
	}

	// Note: The charstrings are already in the new order that they need be
	// written in.
	charStrings, fdRemapper, err := subsetCharstrings(&table, ctx.mapper)
	if err != nil {
		return err
	}

	sidRemapper, ok := getSIDRemapper(&table, &fdRemapper)
	if !ok {
		return errSubset
	}

	var offsets *offsetsCFF
	if table.fontKind.cid == nil {
		offsets = newOffsetsSID()
	} else {
		offsets = newOffsetsCID(fdRemapper.len())
	}

	w := newWriter()
	// HEADER
	// We always use OffSize 4 (but as far as I can tell this field is unused
	// anyway).
	w.extend([]byte{1, 0, 4, 4})
	// Name INDEX
	w.extend(table.names)
	// Top DICT INDEX
	if err := rewriteTopDictIndex(&table.topDictData, offsets, sidRemapper, w); err != nil {
		return err
	}
	// String INDEX
	var strings [][]byte
	for s := range sidRemapper.sortedStrings() {
		strings = append(strings, s)
	}
	index, err := createIndex(strings)
	if err != nil {
		return err
	}
	w.extend(index.data)
	// Global Subr INDEX
	// We desubroutinized, so no global subroutines and thus empty index.
	defaultOwnedIndex().write(w)

	// Charsets
	if err := offsets.charsetOffset.updateValue(w.len()); err != nil {
		return err
	}
	rewriteCharset(ctx.mapper, w)

	// Private dicts.
	if cid := table.fontKind.cid; cid == nil {
		// Since we convert SID-keyed to CID-keyed, we write one private dict
		// with index 0.
		if err := rewritePrivateDict(offsets, table.fontKind.sid.privateDictData, w, 0); err != nil {
			return err
		}
	} else if err := rewriteCIDPrivateDicts(&fdRemapper, offsets, cid, w); err != nil {
		return err
	}

	// FDSelect
	if err := offsets.fdSelectOffset.updateValue(w.len()); err != nil {
		return err
	}
	if cid := table.fontKind.cid; cid == nil {
		generateFDIndex(ctx.mapper, w)
	} else if err := rewriteFDIndex(ctx.mapper, cid.fdSelect, &fdRemapper, w); err != nil {
		return err
	}

	// FDArray
	if err := offsets.fdArrayOffset.updateValue(w.len()); err != nil {
		return err
	}
	if cid := table.fontKind.cid; cid == nil {
		if err := generateFontDictIndex(offsets, w); err != nil {
			return err
		}
	} else if err := rewriteFontDictIndex(&fdRemapper, sidRemapper, offsets, cid, w, !table.topDictData.hasFontMatrix); err != nil {
		return err
	}

	// Charstrings INDEX
	if err := offsets.charStringsOffset.updateValue(w.len()); err != nil {
		return err
	}
	index, err = createIndex(charStrings)
	if err != nil {
		return err
	}
	w.extend(index.data)

	subsettedFont := w.finish()

	// Rewrite the dummy offsets.
	if err := updateOffsets(offsets, subsettedFont); err != nil {
		return err
	}

	ctx.push(tagCFF, subsettedFont)

	return nil
}

// updateOffsets is update_offsets in subsetter src/cff/mod.rs.
func updateOffsets(offsets *offsetsCFF, buffer []byte) error {
	write := func(offset deferredOffset) error {
		if offset != dummyOffset {
			return offset.writeInto(buffer)
		}
		return nil
	}

	// Private dicts offset have already been written correctly, so no need
	// to write them here.

	for _, o := range []deferredOffset{
		offsets.charsetOffset,
		offsets.charStringsOffset,
		offsets.fdSelectOffset,
		offsets.fdArrayOffset,
	} {
		if err := write(o); err != nil {
			return err
		}
	}

	return nil
}

// subsetCharstrings is subset_charstrings in subsetter src/cff/mod.rs. It
// creates the list of bytes that constitute the programs of the
// charstrings, sorted in the new glyph order.
//
// Upstream decompiles each charstring into a program, and compiles the
// programs after the loop. The port compiles each program when it is
// decompiled, which gives the same bytes and needs the memory of one program
// only.
func subsetCharstrings(table *tableCFF, remapper *GlyphRemapper) ([][]byte, fontDictRemapper, error) {
	gsubrs := newSubroutineContainer(collectIndex(table.globalSubrs))

	var lsubrs subroutineCollection
	if cid := table.fontKind.cid; cid != nil {
		subroutines := make([][][]byte, 0, len(cid.fontDicts))
		for _, fontDict := range cid.fontDicts {
			subroutines = append(subroutines, collectIndex(fontDict.localSubrs))
		}
		lsubrs = newSubroutineCollection(subroutines)
	} else {
		lsubrs = newSubroutineCollection([][][]byte{collectIndex(table.fontKind.sid.localSubrs)})
	}

	var usedFDs [256]bool
	var charStrings [][]byte

	for oldGID := range remapper.RemappedGids() {
		var fdIndex uint8
		if cid := table.fontKind.cid; cid != nil {
			var ok bool
			fdIndex, ok = cid.fdSelect.fontDictIndex(oldGID)
			if !ok {
				return nil, fontDictRemapper{}, errMalformedFont
			}
			usedFDs[fdIndex] = true
		}

		lsubrHandler, ok := lsubrs.getHandler(fdIndex)
		if !ok {
			return nil, fontDictRemapper{}, errMalformedFont
		}
		decompiler := newDecompiler(gsubrs.getHandler(), lsubrHandler)
		charstring, ok := table.charStrings.get(uint32(oldGID))
		if !ok {
			return nil, fontDictRemapper{}, errMalformedFont
		}
		program, err := decompiler.decompile(charstring)
		if err != nil {
			return nil, fontDictRemapper{}, err
		}
		charStrings = append(charStrings, program.compile())
	}

	fdRemapper := newRemapper[uint8, uint8]()

	// Upstream keeps the used font DICTs in a BTreeSet, and remaps them in
	// ascending order.
	for fd, used := range usedFDs {
		if !used {
			continue
		}
		// Upstream panics on the 256th font DICT, where the counter of the
		// remapper overflows.
		if _, ok := fdRemapper.tryRemap(uint8(fd)); !ok {
			return nil, fontDictRemapper{}, fmt.Errorf("%w: the glyphs of the subset use 256 font DICTs", errOverflow)
		}
	}

	return charStrings, fdRemapper, nil
}

// collectIndex is index.into_iter().collect::<Vec<_>>() of Rust. As the
// iterator of upstream, it stops at the first item that the INDEX cannot
// give.
func collectIndex(x index) [][]byte {
	var out [][]byte
	for v := range x.all() {
		out = append(out, v)
	}
	return out
}

// getSIDRemapper is get_sid_remapper in subsetter src/cff/mod.rs.
func getSIDRemapper(table *tableCFF, fdRemapper *fontDictRemapper) (*sidRemapper, bool) {
	sidRemapper := newSIDRemapper()
	sidRemapper.remap([]byte("Adobe"))
	sidRemapper.remap([]byte("Identity"))

	remapSID := func(sid stringID) bool {
		if sid.isStandardString() {
			return true
		}
		str, ok := table.strings.get(uint32(sid - standardStringLen))
		if !ok {
			return false
		}
		sidRemapper.remapWithOldSID(sid, str)

		return true
	}

	if table.topDictData.hasCopyright && !remapSID(table.topDictData.copyright) {
		return nil, false
	}

	if table.topDictData.hasFontName && !remapSID(table.topDictData.fontName) {
		return nil, false
	}

	if table.topDictData.hasNotice && !remapSID(table.topDictData.notice) {
		return nil, false
	}

	if cid := table.fontKind.cid; cid != nil {
		for fd := range fdRemapper.sortedIter() {
			if int(fd) >= len(cid.fontDicts) {
				return nil, false
			}
			fontDict := &cid.fontDicts[fd]

			if fontDict.hasFontName && !remapSID(fontDict.fontName) {
				return nil, false
			}
		}
	}

	return sidRemapper, true
}

// parseTableCFF is Table::parse in subsetter src/cff/mod.rs. The parsing
// logic was taken from ttf-parser.
func parseTableCFF(ctx *context) (tableCFF, error) {
	cff, ok := ctx.expectTable(tagCFF)
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	r := newReader(cff)

	major, ok := r.readU8()
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	if major != 1 {
		return tableCFF{}, errCFF
	}

	r.skip(1) // minor
	headerSize, ok := r.readU8()
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	r.jump(int(headerSize))

	namesStart := r.offset()
	if !skipIndex(&r) {
		return tableCFF{}, errMalformedFont
	}
	if namesStart > r.offset() || r.offset() > len(cff) {
		return tableCFF{}, errMalformedFont
	}
	names := cff[namesStart:r.offset()]
	topDictData, ok := parseTopDictIndex(&r)
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	strings, ok := parseIndex(&r)
	if !ok {
		return tableCFF{}, errMalformedFont
	}
	globalSubrs, ok := parseIndex(&r)
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	if !topDictData.hasCharStrings {
		return tableCFF{}, errMalformedFont
	}
	cr := newReaderAtOffset(cff, topDictData.charStrings)
	charStrings, ok := parseIndex(&cr)
	if !ok {
		return tableCFF{}, errMalformedFont
	}

	numberOfGlyphs := charStrings.len()
	if numberOfGlyphs == 0 || numberOfGlyphs > 0xFFFF {
		return tableCFF{}, errMalformedFont
	}

	var fontKind fontKindCFF
	if topDictData.hasROS {
		cid, ok := parseCIDMetadata(cff, &topDictData, uint16(numberOfGlyphs))
		if !ok {
			return tableCFF{}, errMalformedFont
		}
		fontKind.cid = &cid
	} else {
		fontKind.sid = parseSIDMetadata(cff, &topDictData)
	}

	return tableCFF{
		names:       names,
		topDictData: topDictData,
		strings:     strings,
		globalSubrs: globalSubrs,
		charStrings: charStrings,
		fontKind:    fontKind,
	}, nil
}
