// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"math"
	"slices"
)

// This file ports subsetter src/name.rs.

// subsetName is subset in subsetter src/name.rs.
func subsetName(ctx *context) error {
	name, ok := ctx.expectTable(tagNAME)
	if !ok {
		return errMalformedFont
	}
	r := newReader(name)

	version, ok := r.readU16()
	if !ok {
		return errMalformedFont
	}

	// From my personal experiments, version 1 isn't used at all, so we don't
	// bother subsetting it.
	if version != 0 {
		ctx.push(tagNAME, name)
		return nil
	}

	table, ok := parseTableName(name)
	if !ok {
		return errMalformedFont
	}
	subsettedTable, ok := subsetTableName(table)
	if !ok {
		return errSubset
	}

	w := newWriter()
	if !subsettedTable.write(w) {
		// Upstream unwraps the conversion of the count to a u16 and panics.
		// subsetTableName keeps at most the records of a table, whose count
		// is a u16, so this does not happen.
		return errOverflow
	}

	ctx.push(tagNAME, w.finish())
	return nil
}

// keptNameIDs is the list of the name IDs that subset_table in subsetter
// src/name.rs keeps.
var keptNameIDs = []uint16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14}

// subsetTableName is subset_table in subsetter src/name.rs.
func subsetTableName(table tableName) (tableName, bool) {
	var names []nameRecord
	for _, record := range table.names {
		if record.isUnicode() && slices.Contains(keptNameIDs, record.nameID) {
			names = append(names, record)
		}
	}

	var storage []byte
	var curStorageOffset uint16

	nameDeduplicator := map[string]uint16{}

	for i := range names {
		record := &names[i]
		// As upstream in a release build, the end wraps in a u16.
		start := int(record.stringOffset)
		end := int(record.stringOffset + record.length)
		if end < start || end > len(table.storage) {
			return tableName{}, false
		}
		name := table.storage[start:end]
		offset, ok := nameDeduplicator[string(name)]
		if !ok {
			storage = append(storage, name...)
			offset = curStorageOffset
			curStorageOffset += record.length
			nameDeduplicator[string(name)] = offset
		}

		record.stringOffset = offset
	}

	return tableName{names: names, storage: storage}, true
}

// write is Writeable::write for Table in subsetter src/name.rs. It returns
// false where upstream panics, for more than 65535 records.
func (t tableName) write(w *writer) bool {
	if len(t.names) > math.MaxUint16 {
		return false
	}
	count := uint16(len(t.names))

	// version
	w.writeU16(0)
	// count
	w.writeU16(count)
	// storage offset
	w.writeU16(2*3 + count*nameRecordSize)
	for _, name := range t.names {
		name.write(w)
	}
	w.extend(t.storage)
	return true
}

// write is Writeable::write for &NameRecord in subsetter src/name.rs.
func (n nameRecord) write(w *writer) {
	w.writeU16(n.platformID)
	w.writeU16(n.encodingID)
	w.writeU16(n.languageID)
	w.writeU16(n.nameID)
	w.writeU16(n.length)
	w.writeU16(n.stringOffset)
}

// tableName is Table in subsetter src/name.rs.
type tableName struct {
	names   []nameRecord
	storage []byte
}

// parseTableName is Table::parse in subsetter src/name.rs. The parsing logic
// was adapted from ttf-parser.
func parseTableName(data []byte) (tableName, bool) {
	r := newReader(data)

	version, ok := r.readU16()
	if !ok || version != 0 {
		return tableName{}, false
	}

	count, ok := r.readU16()
	if !ok {
		return tableName{}, false
	}
	storageOffset, ok := r.readU16()
	if !ok {
		return tableName{}, false
	}

	// Upstream reserves count records first. The port grows the slice as it
	// reads, so that a count larger than the data does not allocate.
	var names []nameRecord

	for range count {
		record, ok := readNameRecord(&r)
		if !ok {
			return tableName{}, false
		}
		names = append(names, record)
	}

	if int(storageOffset) > len(data) {
		return tableName{}, false
	}
	storage := data[storageOffset:]

	return tableName{names: names, storage: storage}, true
}

// nameRecordSize is Readable::SIZE for NameRecord in subsetter src/name.rs.
const nameRecordSize = 2 * 6

// readNameRecord is Readable::read for NameRecord in subsetter src/name.rs.
func readNameRecord(r *reader) (nameRecord, bool) {
	var n nameRecord
	for _, p := range []*uint16{&n.platformID, &n.encodingID, &n.languageID, &n.nameID, &n.length, &n.stringOffset} {
		v, ok := r.readU16()
		if !ok {
			return nameRecord{}, false
		}
		*p = v
	}
	return n, true
}

// nameRecord is NameRecord in subsetter src/name.rs.
type nameRecord struct {
	platformID   uint16
	encodingID   uint16
	languageID   uint16
	nameID       uint16
	length       uint16
	stringOffset uint16
}

// isUnicode is NameRecord::is_unicode in subsetter src/name.rs.
func (n nameRecord) isUnicode() bool {
	return n.platformID == 0 ||
		(n.platformID == 3 && slices.Contains([]uint16{0, 1, 10}, n.encodingID))
}
