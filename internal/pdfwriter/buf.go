// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"math"
	"slices"
	"strconv"

	"github.com/xo/okage/internal/wide"
)

// This file ports pdf-writer src/buf.rs.

// Buf is Buf in pdf-writer src/buf.rs.
//
// A buffer of arbitrary PDF content.
//
// A Buf also holds the writers that write into it and are not finished, as
// the package comment says.
type Buf struct {
	inner  []byte
	limits Limits

	// open holds the open writers, the outermost first.
	open []closer
	// objs counts the Obj values that the buffer gave. pending is the number
	// of the one Obj that can still write, or 0.
	objs    uint64
	pending uint64
}

// newBuf is Buf::new in pdf-writer src/buf.rs.
func newBuf() Buf {
	return Buf{}
}

// newBufWithCapacity is Buf::with_capacity in pdf-writer src/buf.rs.
func newBufWithCapacity(capacity int) Buf {
	return Buf{inner: make([]byte, 0, capacity)}
}

// IntoVec is Buf::into_vec in pdf-writer src/buf.rs. It finishes the open
// writers first.
//
// Get the underlying bytes of the buffer.
func (b *Buf) IntoVec() []byte {
	b.finishFrom(0)
	return b.inner
}

// AsSlice is Buf::as_slice in pdf-writer src/buf.rs. It finishes the open
// writers first.
//
// Get the underlying bytes of the buffer as a slice.
func (b *Buf) AsSlice() []byte {
	b.finishFrom(0)
	return b.inner
}

// Limits is Buf::limits in pdf-writer src/buf.rs. It finishes the open
// writers first.
//
// Return the limits of the buffer.
func (b *Buf) Limits() Limits {
	b.finishFrom(0)
	return b.limits
}

// pushVal is Buf::push_val in pdf-writer src/buf.rs.
func (b *Buf) pushVal(value Primitive) {
	value.write(b)
}

// pushInt is Buf::push_int in pdf-writer src/buf.rs.
func (b *Buf) pushInt(value int32) {
	b.limits.registerInt(value)
	b.inner = strconv.AppendInt(b.inner, int64(value), 10)
}

// pushFloat is Buf::push_float in pdf-writer src/buf.rs.
func (b *Buf) pushFloat(value float32) {
	// Don't write the decimal point if we don't need it.
	// Also, integer formatting is way faster.
	if i := wide.TruncI32(value); float32(i) == value {
		b.pushInt(i)
	} else {
		b.pushDecimal(value)
	}
}

// pushDecimal is Buf::push_decimal in pdf-writer src/buf.rs.
//
// Like pushFloat, but forces the decimal point.
func (b *Buf) pushDecimal(value float32) {
	b.limits.registerReal(value)

	if a := abs32(value); value == 0.0 || (a > 1e-6 && a < 1e12) {
		b.inner = appendRyu32(b.inner, value)
	} else {
		b.inner = appendDisplay32(b.inner, value)
	}
}

// extendBuf is Buf::extend_buf in pdf-writer src/buf.rs.
func (b *Buf) extendBuf(other *Buf) {
	other.finishFrom(0)
	b.limits.Merge(other.limits)
	b.inner = append(b.inner, other.inner...)
}

// push is Buf::push in pdf-writer src/buf.rs.
func (b *Buf) push(c byte) {
	b.inner = append(b.inner, c)
}

// pushHex is Buf::push_hex in pdf-writer src/buf.rs.
func (b *Buf) pushHex(value uint8) {
	hex := func(c uint8) byte {
		if c < 10 {
			return '0' + c
		}
		return 'A' + (c - 10)
	}

	b.push(hex(value >> 4))
	b.push(hex(value & 0xF))
}

// pushHexU16 is Buf::push_hex_u16 in pdf-writer src/buf.rs.
func (b *Buf) pushHexU16(value uint16) {
	b.pushHex(uint8(value >> 8))
	b.pushHex(uint8(value))
}

// pushOctal is Buf::push_octal in pdf-writer src/buf.rs.
func (b *Buf) pushOctal(value uint8) {
	octal := func(c uint8) byte {
		return '0' + c
	}

	b.push(octal(value >> 6))
	b.push(octal((value >> 3) & 7))
	b.push(octal(value & 7))
}

// reserve is Buf::reserve in pdf-writer src/buf.rs.
func (b *Buf) reserve(additional int) {
	b.inner = slices.Grow(b.inner, additional)
}

// Len is the length of the slice that the impl Deref for Buf in pdf-writer
// src/buf.rs gives. It finishes the open writers first.
func (b *Buf) Len() int {
	b.finishFrom(0)
	return len(b.inner)
}

// extend is the impl Extend<u8> for Buf in pdf-writer src/buf.rs.
func (b *Buf) extend(bytes []byte) {
	b.inner = append(b.inner, bytes...)
}

// extendString is extend for the bytes of a string.
func (b *Buf) extendString(s string) {
	b.inner = append(b.inner, s...)
}

// Limits is Limits in pdf-writer src/buf.rs.
//
// Tracks the limits of data types used in a buffer.
type Limits struct {
	int         int32
	real        float32
	nameLen     int
	strLen      int
	arrayLen    int
	dictEntries int
}

// NewLimits is Limits::new in pdf-writer src/buf.rs.
//
// Create a new Limits struct with all values initialized to zero.
func NewLimits() Limits {
	return Limits{}
}

// Int is Limits::int in pdf-writer src/buf.rs.
//
// Get the absolute value of the largest positive/negative integer number.
func (l Limits) Int() int32 {
	return l.int
}

// Real is Limits::real in pdf-writer src/buf.rs.
//
// Get the absolute value of the largest positive/negative real number.
func (l Limits) Real() float32 {
	return l.real
}

// NameLen is Limits::name_len in pdf-writer src/buf.rs.
//
// Get the maximum length of any used name.
func (l Limits) NameLen() int {
	return l.nameLen
}

// StrLen is Limits::str_len in pdf-writer src/buf.rs.
//
// Get the maximum length of any used string.
func (l Limits) StrLen() int {
	return l.strLen
}

// ArrayLen is Limits::array_len in pdf-writer src/buf.rs.
//
// Get the maximum length of any used array.
func (l Limits) ArrayLen() int {
	return l.arrayLen
}

// DictEntries is Limits::dict_entries in pdf-writer src/buf.rs.
//
// Get the maximum number of entries in any dictionary.
func (l Limits) DictEntries() int {
	return l.dictEntries
}

// registerInt is Limits::register_int in pdf-writer src/buf.rs. The absolute
// value of math.MinInt32 wraps to math.MinInt32, as in a release build of
// Rust.
func (l *Limits) registerInt(val int32) {
	if val < 0 {
		val = -val
	}
	l.int = max(l.int, val)
}

// registerReal is Limits::register_real in pdf-writer src/buf.rs.
func (l *Limits) registerReal(val float32) {
	l.real = maxF32(l.real, abs32(val))
}

// registerNameLen is Limits::register_name_len in pdf-writer src/buf.rs.
func (l *Limits) registerNameLen(n int) {
	l.nameLen = max(l.nameLen, n)
}

// registerStrLen is Limits::register_str_len in pdf-writer src/buf.rs.
func (l *Limits) registerStrLen(n int) {
	l.strLen = max(l.strLen, n)
}

// registerArrayLen is Limits::register_array_len in pdf-writer src/buf.rs.
func (l *Limits) registerArrayLen(n int) {
	l.arrayLen = max(l.arrayLen, n)
}

// registerDictEntries is Limits::register_dict_entries in pdf-writer
// src/buf.rs.
func (l *Limits) registerDictEntries(n int) {
	l.dictEntries = max(l.dictEntries, n)
}

// Merge is Limits::merge in pdf-writer src/buf.rs.
//
// Merge two Limits with each other, taking the maximum of each field from
// both.
func (l *Limits) Merge(other Limits) {
	l.registerInt(other.int)
	l.registerReal(other.real)
	l.registerNameLen(other.nameLen)
	l.registerStrLen(other.strLen)
	l.registerArrayLen(other.arrayLen)
	l.registerDictEntries(other.dictEntries)
}

// The functions below port no code of pdf-writer.

// abs32 is f32::abs of Rust.
func abs32(f float32) float32 {
	return math.Float32frombits(math.Float32bits(f) &^ (1 << 31))
}

// maxF32 is f32::max of Rust. If one value is NaN, it returns the other one.
// The builtin max of Go returns NaN.
func maxF32(a, b float32) float32 {
	switch {
	case a != a:
		return b
	case b != b:
		return a
	case b > a:
		return b
	}
	return a
}

// appendRyu32 appends f as ryu::Buffer::format of the crate ryu 1.0.23
// writes an f32, which pdf-writer calls in push_decimal. It is the shortest
// decimal that gives back f, as pretty::format32 of ryu lays it out: a whole
// number ends in ".0", a number from 1e-5 up to 1e13 has no exponent, and
// any other number has one, such as "1.5e-7" or "1e30".
func appendRyu32(dst []byte, f float32) []byte {
	switch {
	case f != f:
		return append(dst, "NaN"...)
	case math.IsInf(float64(f), 1):
		return append(dst, "inf"...)
	case math.IsInf(float64(f), -1):
		return append(dst, "-inf"...)
	}
	if math.Signbit(float64(f)) {
		dst = append(dst, '-')
	}
	if f == 0 {
		return append(dst, "0.0"...)
	}

	// Take the shortest digits and the exponent from strconv, which gives
	// them as "d.ddde±XX".
	var tmp [32]byte
	e := strconv.AppendFloat(tmp[:0], float64(abs32(f)), 'e', -1, 32)
	var digits []byte
	exp := 0
	for i, c := range e {
		if c == 'e' {
			x, err := strconv.Atoi(string(e[i+1:]))
			if err != nil {
				panic(err)
			}
			exp = x
			break
		}
		if c != '.' {
			digits = append(digits, c)
		}
	}

	length := len(digits)
	kk := exp + 1 // 10^(kk-1) <= v < 10^kk
	k := kk - length

	switch {
	case 0 <= k && kk <= 13:
		// 1234e7 -> 12340000000.0
		dst = append(dst, digits...)
		for range kk - length {
			dst = append(dst, '0')
		}
		dst = append(dst, ".0"...)
	case 0 < kk && kk <= 13:
		// 1234e-2 -> 12.34
		dst = append(dst, digits[:kk]...)
		dst = append(dst, '.')
		dst = append(dst, digits[kk:]...)
	case -6 < kk && kk <= 0:
		// 1234e-6 -> 0.001234
		dst = append(dst, "0."...)
		for range -kk {
			dst = append(dst, '0')
		}
		dst = append(dst, digits...)
	case length == 1:
		// 1e30
		dst = append(dst, digits[0], 'e')
		dst = strconv.AppendInt(dst, int64(kk-1), 10)
	default:
		// 1234e30 -> 1.234e33
		dst = append(dst, digits[0], '.')
		dst = append(dst, digits[1:]...)
		dst = append(dst, 'e')
		dst = strconv.AppendInt(dst, int64(kk-1), 10)
	}
	return dst
}

// appendDisplay32 appends f as the trait Display of Rust writes an f32, as
// pdf-writer does with write! for a number that ryu does not write: the
// shortest decimal that gives back f, without an exponent. NaN is "NaN", and
// an infinity is "inf" or "-inf".
func appendDisplay32(dst []byte, f float32) []byte {
	switch {
	case f != f:
		return append(dst, "NaN"...)
	case math.IsInf(float64(f), 1):
		return append(dst, "inf"...)
	case math.IsInf(float64(f), -1):
		return append(dst, "-inf"...)
	}
	return strconv.AppendFloat(dst, float64(f), 'f', -1, 32)
}

// closer is a writer that is open: an Array, a Dict or a Stream that is not
// finished. close writes its end, as the impl Drop of its Rust type does.
type closer interface {
	close()
}

// openWriter adds an open writer, and returns its level, the index of the
// writer in b.open.
func (b *Buf) openWriter(w closer) int {
	b.open = append(b.open, w)
	return len(b.open) - 1
}

// finishFrom finishes the open writers from the level on, the innermost
// first, as Rust drops them. Then no Obj can write.
func (b *Buf) finishFrom(level int) {
	for len(b.open) > level {
		i := len(b.open) - 1
		w := b.open[i]
		b.open[i] = nil
		b.open = b.open[:i]
		w.close()
	}
	b.pending = 0
}

// newObj gives the number of a new Obj, which is the one Obj that can write.
func (b *Buf) newObj() uint64 {
	b.objs++
	b.pending = b.objs
	return b.objs
}

// useObj takes the right to write from the Obj with the number seq. It
// panics if that Obj cannot write.
func (b *Buf) useObj(seq uint64) {
	if seq == 0 || b.pending != seq {
		panic("pdfwriter: an Obj writes after another write of its buffer, or twice")
	}
	b.pending = 0
}
