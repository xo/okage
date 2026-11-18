// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"slices"
	"testing"
)

// subsetter src/read.rs and src/write.rs have no tests. These tests check
// the reader and the writer, which the tables of each unit read and write.

func TestReader(t *testing.T) {
	data := []byte{1, 0xFF, 0xFE, 0x80, 0, 0, 1, 0xFF, 0xFF, 0xFF, 0xFE, 9}
	r := newReader(data)
	if v, ok := r.readU8(); !ok || v != 1 {
		t.Errorf("readU8 = %d, %v", v, ok)
	}
	if v, ok := r.readI16(); !ok || v != -2 {
		t.Errorf("readI16 = %d, %v", v, ok)
	}
	if v, ok := r.readU32(); !ok || v != 0x80000001 {
		t.Errorf("readU32 = %#x, %v", v, ok)
	}
	if v, ok := r.readI32(); !ok || v != -2 {
		t.Errorf("readI32 = %d, %v", v, ok)
	}
	if r.offset() != 11 || r.atEnd() {
		t.Errorf("the offset is %d", r.offset())
	}
	// A read past the end fails, and skipBytes does not move.
	if _, ok := r.readU16(); ok {
		t.Error("readU16 reads past the end")
	}
	r.skipBytes(2)
	if r.offset() != 11 {
		t.Errorf("skipBytes moves to %d", r.offset())
	}
	r.skip(1)
	if !r.atEnd() {
		t.Error("the reader is not at the end")
	}
	if tail, ok := r.tail(); !ok || len(tail) != 0 {
		t.Errorf("the tail is % x, %v", tail, ok)
	}
	r.jump(20)
	if _, ok := r.tail(); ok || !r.atEnd() {
		t.Error("the tail past the end exists")
	}
	r = newReaderAt(data, -1)
	if _, ok := r.readU8(); ok {
		t.Error("a reader at a negative offset reads")
	}
	if _, ok := r.readBytes(-1); ok {
		t.Error("readBytes reads a negative length")
	}

	r = newReaderAt(data, 1)
	a, ok := readArray16(&r, 3, 2, (*reader).readU16)
	if !ok || a.len() != 3 {
		t.Fatalf("readArray16 = %d, %v", a.len(), ok)
	}
	if v, ok := a.get(1); !ok || v != 0x8000 {
		t.Errorf("get(1) = %#x, %v", v, ok)
	}
	if _, ok := a.get(3); ok {
		t.Error("get(3) exists")
	}
	if got := slices.Collect(a.all()); !slices.Equal(got, []uint16{0xFFFE, 0x8000, 1}) {
		t.Errorf("all gives %#x", got)
	}
	if _, ok := readArray16(&r, 10, 2, (*reader).readU16); ok {
		t.Error("readArray16 reads past the end")
	}
	var empty lazyArray16[uint16]
	if empty.len() != 0 {
		t.Error("the empty array is not empty")
	}
}

func TestWriter(t *testing.T) {
	w := newWriterWithCapacity(4)
	w.writeU8(1)
	w.writeI16(-2)
	w.align(4)
	w.writeU32(0x01020304)
	w.writeI32(-1)
	w.writeU16(0xABCD)
	w.extend([]byte{7})
	want := []byte{1, 0xFF, 0xFE, 0, 1, 2, 3, 4, 0xFF, 0xFF, 0xFF, 0xFF, 0xAB, 0xCD, 7}
	if got := w.finish(); !slices.Equal(got, want) || w.len() != len(want) {
		t.Errorf("got % x, want % x", got, want)
	}
}
