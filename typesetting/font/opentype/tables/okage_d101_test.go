// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package tables

// okage: this file is not in go-text. It tests the deltas of 32 bits of
// D101.

import (
	"reflect"
	"testing"
)

// TestItemVariationDataLongWords checks the deltas of an
// ItemVariationData with the flag LONG_WORDS. A row then holds the word
// deltas as int32 and the other deltas as int16. go-text returned an
// error for such a subtable.
func TestItemVariationDataLongWords(t *testing.T) {
	src := []byte{
		0, 2, // itemCount
		0x80, 1, // wordDeltaCount: LONG_WORDS and 1 word delta
		0, 3, // regionIndexCount
		0, 0, 0, 1, 0, 2, // regionIndexes
		// The row 0: 46875, -2 and 300.
		0, 0, 0xB7, 0x1B, 0xFF, 0xFE, 0x01, 0x2C,
		// The row 1: -100000, 32767 and -32768.
		0xFF, 0xFE, 0x79, 0x60, 0x7F, 0xFF, 0x80, 0x00,
	}
	ivd, _, err := ParseItemVariationData(src)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int32{{46875, -2, 300}, {-100000, 32767, -32768}}
	if !reflect.DeepEqual(ivd.DeltaSets, want) {
		t.Errorf("the deltas are %v, want %v", ivd.DeltaSets, want)
	}

	// The same deltas without LONG_WORDS have 2 bytes and 1 byte, as in
	// go-text.
	src = []byte{
		0, 1, 0, 1, 0, 2, 0, 0, 0, 1,
		0x80, 0x00, 0xFE,
	}
	ivd, _, err = ParseItemVariationData(src)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]int32{{-32768, -2}}; !reflect.DeepEqual(ivd.DeltaSets, want) {
		t.Errorf("the deltas are %v, want %v", ivd.DeltaSets, want)
	}

	// A row that the data does not hold is an error.
	if _, _, err := ParseItemVariationData(src[:12]); err == nil {
		t.Error("a short row gives no error")
	}
}
