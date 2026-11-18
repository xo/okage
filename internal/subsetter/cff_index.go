// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "iter"

// This file ports subsetter src/cff/index.rs, which was taken from
// ttf-parser. The trait IndexSize of upstream reads the count of an INDEX as
// a u16 or a u32. The CFF code of subsetter reads only INDEXes with a count
// of the type u16, so parseIndex and skipIndex read a u16.

// parseIndex is parse_index::<u16> in subsetter src/cff/index.rs.
func parseIndex(r *reader) (index, bool) {
	count, ok := r.readU16()
	if !ok {
		return index{}, false
	}
	return parseIndexImpl(uint32(count), r)
}

// parseIndexImpl is parse_index_impl in subsetter src/cff/index.rs.
func parseIndexImpl(count uint32, r *reader) (index, bool) {
	if count == 0 || count == ^uint32(0) {
		return defaultIndex(), true
	}

	offsets, ok := readVarOffsets(count, r)
	if !ok {
		return index{}, false
	}

	lastOffset, ok := offsets.last()
	if !ok {
		return defaultIndex(), true
	}
	data, ok := r.readBytes(int(lastOffset))
	if !ok {
		return index{}, false
	}
	return index{data: data, offsets: offsets}, true
}

// readVarOffsets reads the offset size and the count+1 offsets of an INDEX,
// as parse_index_impl and skip_index_impl in subsetter src/cff/index.rs do.
func readVarOffsets(count uint32, r *reader) (varOffsets, bool) {
	offsetSize, ok := readOffsetSize(r)
	if !ok {
		return varOffsets{}, false
	}
	// count+1 does not overflow, because count is not u32::MAX. Rust
	// computes the product with checked_mul in a u32.
	offsetsLen := uint64(count+1) * uint64(offsetSize)
	if offsetsLen > uint64(^uint32(0)) || offsetsLen > uint64(len(r.data)) {
		return varOffsets{}, false
	}
	data, ok := r.readBytes(int(offsetsLen))
	if !ok {
		return varOffsets{}, false
	}
	return varOffsets{data: data, offsetSize: offsetSize}, true
}

// skipIndex is skip_index::<u16> in subsetter src/cff/index.rs.
func skipIndex(r *reader) bool {
	count, ok := r.readU16()
	if !ok {
		return false
	}
	return skipIndexImpl(uint32(count), r)
}

// skipIndexImpl is skip_index_impl in subsetter src/cff/index.rs.
func skipIndexImpl(count uint32, r *reader) bool {
	if count == 0 || count == ^uint32(0) {
		return true
	}

	offsets, ok := readVarOffsets(count, r)
	if !ok {
		return false
	}

	if lastOffset, ok := offsets.last(); ok {
		r.skipBytes(int(lastOffset))
	}

	return true
}

// varOffsets is VarOffsets in subsetter src/cff/index.rs.
type varOffsets struct {
	data       []byte
	offsetSize offsetSize
}

// get is VarOffsets::get in subsetter src/cff/index.rs.
func (o varOffsets) get(index uint32) (uint32, bool) {
	if index >= o.len() {
		return 0, false
	}

	start := int(index) * int(o.offsetSize)
	r := newReaderAt(o.data, start)
	var n uint32
	var ok bool
	switch o.offsetSize {
	case offsetSize1:
		var v uint8
		v, ok = r.readU8()
		n = uint32(v)
	case offsetSize2:
		var v uint16
		v, ok = r.readU16()
		n = uint32(v)
	case offsetSize3:
		var v u24
		v, ok = readU24(&r)
		n = uint32(v)
	case offsetSize4:
		n, ok = r.readU32()
	}
	if !ok {
		return 0, false
	}

	// Offsets are offset by one byte in the font, so we have to shift them
	// back.
	if n == 0 {
		return 0, false
	}
	return n - 1, true
}

// last is VarOffsets::last in subsetter src/cff/index.rs.
func (o varOffsets) last() (uint32, bool) {
	if o.isEmpty() {
		return 0, false
	}
	return o.get(o.len() - 1)
}

// len is VarOffsets::len in subsetter src/cff/index.rs.
func (o varOffsets) len() uint32 {
	return uint32(len(o.data)) / uint32(o.offsetSize)
}

// isEmpty is VarOffsets::is_empty in subsetter src/cff/index.rs.
func (o varOffsets) isEmpty() bool {
	return o.len() == 0
}

// index is Index in subsetter src/cff/index.rs. Make the empty INDEX with
// defaultIndex.
type index struct {
	data    []byte
	offsets varOffsets
}

// defaultIndex is the impl Default of Index in subsetter src/cff/index.rs.
func defaultIndex() index {
	return index{offsets: varOffsets{offsetSize: offsetSize1}}
}

// all is the impl IntoIterator of Index and IndexIter in subsetter
// src/cff/index.rs. As upstream, it stops at the first item that get does
// not find.
func (x index) all() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for i := range x.len() {
			v, ok := x.get(i)
			if !ok || !yield(v) {
				return
			}
		}
	}
}

// len is Index::len in subsetter src/cff/index.rs.
func (x index) len() uint32 {
	n := x.offsets.len()
	if n == 0 {
		return 0
	}
	return n - 1
}

// get is Index::get in subsetter src/cff/index.rs.
func (x index) get(i uint32) ([]byte, bool) {
	if i == ^uint32(0) {
		return nil, false
	}
	start, ok := x.offsets.get(i)
	if !ok {
		return nil, false
	}
	end, ok := x.offsets.get(i + 1)
	if !ok {
		return nil, false
	}
	if start > end || uint64(end) > uint64(len(x.data)) {
		return nil, false
	}
	return x.data[start:end], true
}

// offsetSize is OffsetSize in subsetter src/cff/index.rs.
type offsetSize uint8

const (
	offsetSize1 offsetSize = 1
	offsetSize2 offsetSize = 2
	offsetSize3 offsetSize = 3
	offsetSize4 offsetSize = 4
)

// readOffsetSize is the impl Readable of OffsetSize in subsetter
// src/cff/index.rs.
func readOffsetSize(r *reader) (offsetSize, bool) {
	v, ok := r.readU8()
	if !ok || v < 1 || v > 4 {
		return 0, false
	}
	return offsetSize(v), true
}

// ownedIndex is OwnedIndex in subsetter src/cff/index.rs. An index that owns
// its data. Make the empty INDEX with defaultOwnedIndex.
type ownedIndex struct {
	data       []byte
	headerSize int
}

// write is the impl Writeable of OwnedIndex in subsetter src/cff/index.rs.
func (x ownedIndex) write(w *writer) {
	w.extend(x.data)
}

// defaultOwnedIndex is the impl Default of OwnedIndex in subsetter
// src/cff/index.rs.
func defaultOwnedIndex() ownedIndex {
	return ownedIndex{data: []byte{0, 0}, headerSize: 2}
}

// createIndex is create_index in subsetter src/cff/index.rs. It creates an
// index from a vector of data. Rust adds the lengths in a u32 that wraps in
// a release build, and so does the port.
func createIndex(data [][]byte) (ownedIndex, error) {
	if len(data) > 0xFFFF {
		return ownedIndex{}, errOverflow
	}
	count := uint16(len(data))
	// + 1 Since we start counting from the preceding byte.
	var offsize uint32
	for _, v := range data {
		offsize += uint32(len(v))
	}
	offsize++

	// Empty Index only contains the count field
	if count == 0 {
		return defaultOwnedIndex(), nil
	}

	var size offsetSize
	switch {
	case offsize <= 0xFF:
		size = offsetSize1
	case offsize <= 0xFFFF:
		size = offsetSize2
	case offsize <= u24Max:
		size = offsetSize3
	default:
		size = offsetSize4
	}

	w := newWriter()
	w.writeU16(count)
	w.writeU8(uint8(size))

	var curOffset uint32

	writeOffset := func(n uint32) error {
		curOffset += n

		switch size {
		case offsetSize1:
			if curOffset > 0xFF {
				return errOverflow
			}
			w.writeU8(uint8(curOffset))
		case offsetSize2:
			if curOffset > 0xFFFF {
				return errOverflow
			}
			w.writeU16(uint16(curOffset))
		case offsetSize3:
			u24(curOffset).write(w)
		case offsetSize4:
			w.writeU32(curOffset)
		}

		return nil
	}

	if err := writeOffset(1); err != nil {
		return ownedIndex{}, err
	}
	for _, el := range data {
		if err := writeOffset(uint32(len(el))); err != nil {
			return ownedIndex{}, err
		}
	}

	headerSize := w.len()

	for _, el := range data {
		w.extend(el)
	}

	return ownedIndex{headerSize: headerSize, data: w.finish()}, nil
}
