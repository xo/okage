// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "encoding/binary"

// This file ports subsetter src/write.rs. The trait Writeable of upstream
// writes a value. The port has a method of writer for each integer type, such
// as writeU16 for w.write::<u16>(), and a method write of each other type,
// such as tableRecord.write. A slice of bytes is written with extend.

// writer is Writer in subsetter src/write.rs. A writable stream of binary
// data.
type writer struct {
	buf []byte
}

// newWriter is Writer::new in subsetter src/write.rs. It creates a new
// writable stream of binary data.
func newWriter() *writer {
	return newWriterWithCapacity(1024)
}

// newWriterWithCapacity is Writer::with_capacity in subsetter src/write.rs.
// It creates a new writable stream of binary data with a capacity.
func newWriterWithCapacity(capacity int) *writer {
	return &writer{buf: make([]byte, 0, capacity)}
}

// extend is Writer::extend in subsetter src/write.rs. It gives bytes to the
// writer.
func (w *writer) extend(bytes []byte) {
	w.buf = append(w.buf, bytes...)
}

// align is Writer::align in subsetter src/write.rs. It aligns the contents to
// a byte boundary.
func (w *writer) align(to int) {
	for len(w.buf)%to != 0 {
		w.buf = append(w.buf, 0)
	}
}

// len is Writer::len in subsetter src/write.rs. It returns the number of
// written bytes.
func (w *writer) len() int {
	return len(w.buf)
}

// finish is Writer::finish in subsetter src/write.rs. It returns the written
// bytes.
func (w *writer) finish() []byte {
	return w.buf
}

// writeU8 is Writeable::write for u8 in subsetter src/write.rs.
func (w *writer) writeU8(v uint8) {
	w.buf = append(w.buf, v)
}

// writeU16 is Writeable::write for u16 in subsetter src/write.rs.
func (w *writer) writeU16(v uint16) {
	w.buf = binary.BigEndian.AppendUint16(w.buf, v)
}

// writeI16 is Writeable::write for i16 in subsetter src/write.rs.
func (w *writer) writeI16(v int16) {
	w.writeU16(uint16(v))
}

// writeU32 is Writeable::write for u32 in subsetter src/write.rs.
func (w *writer) writeU32(v uint32) {
	w.buf = binary.BigEndian.AppendUint32(w.buf, v)
}

// writeI32 is Writeable::write for i32 in subsetter src/write.rs.
func (w *writer) writeI32(v int32) {
	w.writeU32(uint32(v))
}
