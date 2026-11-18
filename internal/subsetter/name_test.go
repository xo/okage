// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"
)

// This file ports the tests of subsetter src/name.rs.

// TestParseRespectsStorageOffset is parse_respects_storage_offset in
// subsetter src/name.rs.
func TestParseRespectsStorageOffset(t *testing.T) {
	var data []byte
	data = binary.BigEndian.AppendUint16(data, 0)      // version
	data = binary.BigEndian.AppendUint16(data, 1)      // count
	data = binary.BigEndian.AppendUint16(data, 22)     // storageOffset
	data = binary.BigEndian.AppendUint16(data, 3)      // nameRecord[0].platformID
	data = binary.BigEndian.AppendUint16(data, 1)      // nameRecord[0].encodingID
	data = binary.BigEndian.AppendUint16(data, 0x0409) // nameRecord[0].languageID
	data = binary.BigEndian.AppendUint16(data, 0)      // nameRecord[0].nameID
	data = binary.BigEndian.AppendUint16(data, 4)      // nameRecord[0].length
	data = binary.BigEndian.AppendUint16(data, 0)      // nameRecord[0].stringOffset
	data = append(data, "BAD!"...)                     // padding before storageOffset
	data = append(data, 0, 'O', 0, 'K')                // storage

	table, ok := parseTableName(data)
	if !ok {
		t.Fatal("the table does not parse")
	}
	subset, ok := subsetTableName(table)
	if !ok {
		t.Fatal("the table does not subset")
	}

	if want := []byte{0, 'O', 0, 'K'}; !bytes.Equal(subset.storage, want) {
		t.Errorf("the storage is % x, want % x", subset.storage, want)
	}
}

// TestSubsetTableName checks that subsetTableName keeps the Unicode records
// of the kept name IDs, and writes each string once.
func TestSubsetTableName(t *testing.T) {
	table := tableName{
		names: []nameRecord{
			{platformID: 3, encodingID: 1, nameID: 1, length: 2, stringOffset: 0},
			{platformID: 1, encodingID: 0, nameID: 1, length: 2, stringOffset: 2},
			{platformID: 0, encodingID: 3, nameID: 4, length: 2, stringOffset: 0},
			{platformID: 3, encodingID: 1, nameID: 10, length: 2, stringOffset: 2},
			{platformID: 3, encodingID: 10, nameID: 2, length: 2, stringOffset: 2},
		},
		storage: []byte("abcd"),
	}
	subset, ok := subsetTableName(table)
	if !ok {
		t.Fatal("the table does not subset")
	}
	if len(subset.names) != 3 || string(subset.storage) != "abcd" {
		t.Fatalf("got %d records and the storage %q", len(subset.names), subset.storage)
	}
	if got := []uint16{subset.names[0].stringOffset, subset.names[1].stringOffset, subset.names[2].stringOffset}; !slices.Equal(got, []uint16{0, 0, 2}) {
		t.Errorf("the offsets are %v", got)
	}
	w := newWriter()
	if !subset.write(w) {
		t.Fatal("the table does not write")
	}
	if got := w.finish(); len(got) != 6+3*nameRecordSize+4 {
		t.Errorf("the table has %d bytes", len(got))
	}

	// An offset and a length whose sum wraps in a u16 is out of the storage.
	table.names[0].stringOffset = 0xFFFF
	if _, ok := subsetTableName(table); ok {
		t.Error("a string out of the storage subsets")
	}
}
