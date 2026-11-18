// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"encoding/binary"
	"iter"
)

// This file ports subsetter src/read.rs. The trait Readable of upstream reads
// a value of a fixed size. The port has a method of reader for each integer
// type, such as readU16 for r.read::<u16>(), and a function readX for each
// other type X, such as readTableRecord. Rust r.peak::<T>() reads from a
// copy of the reader, so the port copies the reader and reads from the copy.

// reader is Reader in subsetter src/read.rs. A readable stream of binary
// data.
type reader struct {
	// data is the underlying data of the reader.
	data []byte
	// off is the current offset in bytes. It is not guaranteed to be in range.
	off int
}

// newReader is Reader::new in subsetter src/read.rs. It creates a new
// readable stream of binary data.
func newReader(data []byte) reader {
	return reader{data: data}
}

// newReaderAt is Reader::new_at in subsetter src/read.rs. It creates a new
// readable stream of binary data at a specific position.
func newReaderAt(data []byte, offset int) reader {
	return reader{data: data, off: offset}
}

// tail is Reader::tail in subsetter src/read.rs. It returns the remaining
// data from the current offset.
func (r *reader) tail() ([]byte, bool) {
	if r.off < 0 || r.off > len(r.data) {
		return nil, false
	}
	return r.data[r.off:], true
}

// offset is Reader::offset in subsetter src/read.rs. It returns the current
// offset.
func (r *reader) offset() int {
	return r.off
}

// readBytes is Reader::read_bytes in subsetter src/read.rs. It reads a
// certain number of bytes.
func (r *reader) readBytes(n int) ([]byte, bool) {
	if n < 0 || r.off < 0 || r.off > len(r.data) || n > len(r.data)-r.off {
		return nil, false
	}
	end := r.off + n
	v := r.data[r.off:end:end]
	r.off = end
	return v, true
}

// readArray16 is Reader::read_array16 in subsetter src/read.rs. It reads
// the next count values of the given size as a lazy array, which reads each
// value with read.
func readArray16[T any](r *reader, count uint16, size int, read func(*reader) (T, bool)) (lazyArray16[T], bool) {
	data, ok := r.readBytes(int(count) * size)
	if !ok {
		return lazyArray16[T]{}, false
	}
	return newLazyArray16(data, size, read), true
}

// skip is Reader::skip in subsetter src/read.rs. It advances by the size of
// a value.
func (r *reader) skip(size int) {
	r.skipBytes(size)
}

// atEnd is Reader::at_end in subsetter src/read.rs. It checks whether the
// reader is at the end of the buffer.
func (r *reader) atEnd() bool {
	return r.off >= len(r.data)
}

// jump is Reader::jump in subsetter src/read.rs. It jumps to a specific
// location.
func (r *reader) jump(offset int) {
	r.off = offset
}

// skipBytes is Reader::skip_bytes in subsetter src/read.rs. It skips the
// next n bytes of the stream. As upstream, it does not move when fewer than
// n bytes remain.
func (r *reader) skipBytes(n int) {
	r.readBytes(n)
}

// readU8 is Readable::read for u8 in subsetter src/read.rs.
func (r *reader) readU8() (uint8, bool) {
	b, ok := r.readBytes(1)
	if !ok {
		return 0, false
	}
	return b[0], true
}

// readU16 is Readable::read for u16 in subsetter src/read.rs.
func (r *reader) readU16() (uint16, bool) {
	b, ok := r.readBytes(2)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint16(b), true
}

// readI16 is Readable::read for i16 in subsetter src/read.rs.
func (r *reader) readI16() (int16, bool) {
	v, ok := r.readU16()
	return int16(v), ok
}

// readU32 is Readable::read for u32 in subsetter src/read.rs.
func (r *reader) readU32() (uint32, bool) {
	b, ok := r.readBytes(4)
	if !ok {
		return 0, false
	}
	return binary.BigEndian.Uint32(b), true
}

// readI32 is Readable::read for i32 in subsetter src/read.rs.
func (r *reader) readI32() (int32, bool) {
	v, ok := r.readU32()
	return int32(v), ok
}

// lazyArray16 is LazyArray16 in subsetter src/read.rs. A slice-like
// container that converts internal binary data only on access. Array values
// are stored in a continuous data chunk. The trait Readable of upstream gives
// the size and the reader of a value, and the port holds them in the array.
type lazyArray16[T any] struct {
	data []byte
	size int
	read func(*reader) (T, bool)
}

// newLazyArray16 is LazyArray16::new in subsetter src/read.rs. It creates a
// new lazy array of the values of the given size, which read reads.
func newLazyArray16[T any](data []byte, size int, read func(*reader) (T, bool)) lazyArray16[T] {
	return lazyArray16[T]{data: data, size: size, read: read}
}

// get is LazyArray16::get in subsetter src/read.rs. It returns the value at
// index.
func (a lazyArray16[T]) get(index uint16) (T, bool) {
	var zero T
	if index >= a.len() {
		return zero, false
	}
	start := int(index) * a.size
	end := start + a.size
	if end > len(a.data) {
		return zero, false
	}
	r := newReader(a.data[start:end])
	return a.read(&r)
}

// len is LazyArray16::len in subsetter src/read.rs. It returns the length of
// the array. As upstream, it keeps the low 16 bits of the count.
func (a lazyArray16[T]) len() uint16 {
	if a.size == 0 {
		return 0
	}
	return uint16(len(a.data) / a.size)
}

// all is the impl IntoIterator of LazyArray16 and LazyArrayIter16 in
// subsetter src/read.rs. It yields the values of the array in order.
func (a lazyArray16[T]) all() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := uint16(0); ; i++ {
			v, ok := a.get(i)
			if !ok || !yield(v) {
				return
			}
		}
	}
}
