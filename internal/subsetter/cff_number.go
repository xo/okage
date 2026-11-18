// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/xo/okage/internal/wide"
)

// This file ports subsetter src/cff/number.rs. The impl Debug of each number
// type is the method String.

// floatStackLen is FLOAT_STACK_LEN in subsetter src/cff/number.rs.
const floatStackLen = 64

// endOfFloatFlag is END_OF_FLOAT_FLAG in subsetter src/cff/number.rs.
const endOfFloatFlag = 0xf

// realNumber is RealNumber in subsetter src/cff/number.rs.
type realNumber float32

// String is the impl Debug of RealNumber in subsetter src/cff/number.rs.
func (n realNumber) String() string {
	return formatF32(float32(n))
}

// integerNumber is IntegerNumber in subsetter src/cff/number.rs.
type integerNumber int32

// String is the impl Debug of IntegerNumber in subsetter
// src/cff/number.rs.
func (n integerNumber) String() string {
	return strconv.FormatInt(int64(n), 10)
}

// fixedNumber is FixedNumber in subsetter src/cff/number.rs. It holds a
// number in the 16.16 format.
type fixedNumber int32

// String is the impl Debug of FixedNumber in subsetter src/cff/number.rs.
func (n fixedNumber) String() string {
	return strconv.FormatInt(int64(n), 10)
}

// asF32 is FixedNumber::as_f32 in subsetter src/cff/number.rs.
func (n fixedNumber) asF32() float32 {
	return float32(n) / 65536.0
}

// parseFixedNumber is FixedNumber::parse in subsetter src/cff/number.rs.
func parseFixedNumber(r *reader) (fixedNumber, bool) {
	b0, ok := r.readU8()
	if !ok {
		return 0, false
	}

	if b0 != 255 {
		return 0, false
	}

	num, ok := r.readI32()
	if !ok {
		return 0, false
	}
	return fixedNumber(num), true
}

// write is the impl Writeable of FixedNumber in subsetter
// src/cff/number.rs.
func (n fixedNumber) write(w *writer) {
	w.writeU8(255)
	w.writeI32(int32(n))
}

// parseRealNumber is RealNumber::parse in subsetter src/cff/number.rs. The
// parsing logic was taken from ttf-parser.
func parseRealNumber(r *reader) (realNumber, bool) {
	var data [floatStackLen]byte
	idx := 0

	b0, ok := r.readU8()
	if !ok {
		return 0, false
	}

	if b0 != 30 {
		return 0, false
	}

	for {
		b1, ok := r.readU8()
		if !ok {
			return 0, false
		}
		nibble1 := b1 >> 4
		nibble2 := b1 & 15

		if nibble1 == endOfFloatFlag {
			break
		}

		if idx, ok = parseFloatNibble(nibble1, idx, &data); !ok {
			return 0, false
		}

		if nibble2 == endOfFloatFlag {
			break
		}

		if idx, ok = parseFloatNibble(nibble2, idx, &data); !ok {
			return 0, false
		}
	}

	n, ok := parseF32(string(data[:idx]))
	if !ok {
		return 0, false
	}

	return realNumber(n), true
}

// parseF32 is s.parse::<f32>() of Rust for the text that parseFloatNibble
// writes, which holds only digits, '.', 'E' and '-'. Rust gives an infinity
// for a number that is too large for a float32, and strconv.ParseFloat gives
// it with the error ErrRange. Both reject the same text: strconv.ParseFloat
// accepts more forms than Rust, such as hexadecimal numbers and "Inf", but
// none of them can be made of these bytes.
func parseF32(s string) (float32, bool) {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return float32(f), true
}

// finite reports whether the real number can be written. Rust writes the
// text of NaN and of an infinity with the impl Display of f32, as "NaN" or
// "inf", and the impl Writeable of RealNumber then panics on the letters. The
// callers of the port test each real number with finite before they write
// it, and return an error where upstream panics.
func (n realNumber) finite() bool {
	f := float64(n)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// write is the impl Writeable of RealNumber in subsetter src/cff/number.rs.
// The number must be finite.
//
// Not the fastest implementation, but floats don't appear that often anyway,
// so it's good enough.
func (n realNumber) write(w *writer) {
	var nibbles []byte

	stringForm := formatF32(float32(n))
	for i := range len(stringForm) {
		b := stringForm[i]

		switch {
		case b >= '0' && b <= '9':
			nibbles = append(nibbles, b-48)
		case b == '.':
			nibbles = append(nibbles, 0xA)
		case b == '-':
			nibbles = append(nibbles, 0xE)
		default:
			panic("unreachable")
		}
	}

	nibbles = append(nibbles, 0xF)

	if len(nibbles)%2 != 0 {
		nibbles = append(nibbles, 0xF)
	}

	// Prefix of fixed number.
	w.writeU8(30)

	for i := 0; i < len(nibbles); i += 2 {
		w.writeU8(nibbles[i]<<4 | nibbles[i+1])
	}
}

// formatF32 is format!("{}", f) of Rust for a float32: the shortest decimal
// that gives back the float32, without an exponent.
func formatF32(f float32) string {
	return strconv.FormatFloat(float64(f), 'f', -1, 32)
}

// parseIntegerNumber is IntegerNumber::parse in subsetter
// src/cff/number.rs.
func parseIntegerNumber(r *reader) (integerNumber, bool) {
	b0, ok := r.readU8()
	if !ok {
		return 0, false
	}
	switch {
	case b0 == 28:
		v, ok := r.readI16()
		return integerNumber(v), ok
	case b0 == 29:
		v, ok := r.readI32()
		return integerNumber(v), ok
	case b0 >= 32 && b0 <= 246:
		n := int32(b0) - 139
		return integerNumber(n), true
	case b0 >= 247 && b0 <= 250:
		b1, ok := r.readU8()
		if !ok {
			return 0, false
		}
		n := (int32(b0)-247)*256 + int32(b1) + 108
		return integerNumber(n), true
	case b0 >= 251 && b0 <= 254:
		b1, ok := r.readU8()
		if !ok {
			return 0, false
		}
		n := -(int32(b0)-251)*256 - int32(b1) - 108
		return integerNumber(n), true
	}
	return 0, false
}

// writeAs5Bytes is IntegerNumber::write_as_5_bytes in subsetter
// src/cff/number.rs. It writes the number as a 5 byte sequence. This is
// necessary when writing offsets, because we need to the length of the
// number to stay stable, since it would otherwise shift everything.
func (n integerNumber) writeAs5Bytes(w *writer) {
	w.writeU8(29)
	w.writeI32(int32(n))
}

// write is the impl Writeable of IntegerNumber in subsetter
// src/cff/number.rs.
func (n integerNumber) write(w *writer) {
	switch {
	case n >= -107 && n <= 107:
		w.writeU8(uint8(n + 139))
	case n >= 108 && n <= 1131:
		temp := n - 108
		w.writeU8(uint8(temp/256 + 247))
		w.writeU8(uint8(temp % 256))
	case n >= -1131 && n <= -108:
		temp := -n - 108
		w.writeU8(uint8(temp/256 + 251))
		w.writeU8(uint8(temp % 256))
	case n >= -32768 && n <= 32767:
		w.writeU8(28)
		w.writeI16(int16(n))
	default:
		n.writeAs5Bytes(w)
	}
}

// numberKind is the case of the enum Number in subsetter src/cff/number.rs.
type numberKind uint8

const (
	// numberInteger is Number::Integer. It is the zero value, so that the
	// zero value of number is Number::zero.
	numberInteger numberKind = iota
	// numberReal is Number::Real.
	numberReal
	// numberFixed is Number::Fixed.
	numberFixed
)

// number is Number in subsetter src/cff/number.rs. The field of its kind
// holds the value, and the other two are 0. The zero value is Number::zero,
// as Number::default of upstream.
type number struct {
	kind    numberKind
	real    realNumber
	integer integerNumber
	fixed   fixedNumber
}

// String is the impl Debug of Number in subsetter src/cff/number.rs.
func (n number) String() string {
	return formatF64(n.asF64())
}

// formatF64 is format!("{}", f) of Rust for a float64.
func formatF64(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// write is the impl Writeable of Number in subsetter src/cff/number.rs. A
// real number must be finite.
func (n number) write(w *writer) {
	switch n.kind {
	case numberReal:
		n.real.write(w)
	case numberInteger:
		n.integer.write(w)
	case numberFixed:
		n.fixed.write(w)
	}
}

// writable reports whether the number can be written: a real number must be
// finite. Upstream has no such function, as finite of realNumber says.
func (n number) writable() bool {
	return n.kind != numberReal || n.real.finite()
}

// writeNumbers is the impl Writeable of a slice or an array of Number in
// subsetter src/write.rs. It writes each number. Where upstream panics on a
// real number that is not finite, it writes nothing and returns an error
// that wraps okage.ErrSubsetMalformedFont.
func writeNumbers(w *writer, nums []number) error {
	for _, n := range nums {
		if !n.writable() {
			return fmt.Errorf("%w: the real number %v of the CFF table is not finite", errMalformedFont, n.real)
		}
	}
	for _, n := range nums {
		n.write(w)
	}
	return nil
}

// parseCFFNumber is Number::parse_cff_number in subsetter
// src/cff/number.rs.
func parseCFFNumber(r *reader) (number, bool) {
	return parseNumber(r, false)
}

// parseCharStringNumber is Number::parse_char_string_number in subsetter
// src/cff/number.rs.
func parseCharStringNumber(r *reader) (number, bool) {
	return parseNumber(r, true)
}

// parseNumber is Number::parse_number in subsetter src/cff/number.rs.
func parseNumber(r *reader, charstringNum bool) (number, bool) {
	peak := *r
	b, ok := peak.readU8()
	if !ok {
		return number{}, false
	}
	switch b {
	case 30:
		v, ok := parseRealNumber(r)
		return number{kind: numberReal, real: v}, ok
	case 255:
		// FIXED only exists in charstrings.
		if charstringNum {
			v, ok := parseFixedNumber(r)
			return number{kind: numberFixed, fixed: v}, ok
		}

		return number{}, false
	}
	v, ok := parseIntegerNumber(r)
	return number{kind: numberInteger, integer: v}, ok
}

// numberFromI32 is Number::from_i32 in subsetter src/cff/number.rs.
func numberFromI32(num int32) number {
	return number{kind: numberInteger, integer: integerNumber(num)}
}

// numberFromF32 is Number::from_f32 in subsetter src/cff/number.rs.
func numberFromF32(num float32) number {
	return number{kind: numberReal, real: realNumber(num)}
}

// numberZero is Number::zero in subsetter src/cff/number.rs.
func numberZero() number {
	return numberFromI32(0)
}

// numberOne is Number::one in subsetter src/cff/number.rs.
func numberOne() number {
	return numberFromI32(1)
}

// asF64 is Number::as_f64 in subsetter src/cff/number.rs.
func (n number) asF64() float64 {
	switch n.kind {
	case numberReal:
		return float64(n.real)
	case numberFixed:
		return float64(n.fixed.asF32())
	}
	return float64(n.integer)
}

// asI32 is Number::as_i32 in subsetter src/cff/number.rs. The cast of Rust
// from a float to an integer saturates, and so does wide.TruncI32.
func (n number) asI32() (int32, bool) {
	switch n.kind {
	case numberReal:
		if fract(float32(n.real)) == 0 {
			return wide.TruncI32(float32(n.real)), true
		}
		return 0, false
	case numberFixed:
		num := n.fixed.asF32()
		if fract(num) == 0 {
			return wide.TruncI32(num), true
		}
		return 0, false
	}
	return int32(n.integer), true
}

// fract is f32::fract of Rust: the part of f after the point, which is NaN
// for an infinity and for NaN.
func fract(f float32) float32 {
	return f - float32(math.Trunc(float64(f)))
}

// asU32 is Number::as_u32 in subsetter src/cff/number.rs.
func (n number) asU32() (uint32, bool) {
	v, ok := n.asI32()
	if !ok || v < 0 {
		return 0, false
	}
	return uint32(v), true
}

// combineNumbers is Number::combine in subsetter src/cff/number.rs, adapted
// from Transform::combine of ttf-parser. Each product is converted to a
// float64 before the sum, so that Go does not fuse a multiplication and an
// addition, which Rust does not do.
func combineNumbers(t1, t2 [6]number) [6]number {
	a := func(i int) float64 { return t1[i].asF64() }
	b := func(i int) float64 { return t2[i].asF64() }
	return [6]number{
		numberFromF32(float32(float64(a(0)*b(0)) + float64(a(2)*b(1)))),
		numberFromF32(float32(float64(a(1)*b(0)) + float64(a(3)*b(1)))),
		numberFromF32(float32(float64(a(0)*b(2)) + float64(a(2)*b(3)))),
		numberFromF32(float32(float64(a(1)*b(2)) + float64(a(3)*b(3)))),
		numberFromF32(float32(float64(a(0)*b(4)) + float64(a(2)*b(5)) + a(4))),
		numberFromF32(float32(float64(a(1)*b(4)) + float64(a(3)*b(5)) + a(5))),
	}
}

// parseFloatNibble is parse_float_nibble in subsetter src/cff/number.rs.
func parseFloatNibble(nibble uint8, idx int, data *[floatStackLen]byte) (int, bool) {
	if idx == floatStackLen {
		return 0, false
	}

	switch {
	case nibble <= 9:
		data[idx] = '0' + nibble
	case nibble == 10:
		data[idx] = '.'
	case nibble == 11:
		data[idx] = 'E'
	case nibble == 12:
		if idx+1 == floatStackLen {
			return 0, false
		}

		data[idx] = 'E'
		idx++
		data[idx] = '-'
	case nibble == 13:
		return 0, false
	case nibble == 14:
		data[idx] = '-'
	default:
		return 0, false
	}

	idx++
	return idx, true
}

// stringID is StringId in subsetter src/cff/number.rs. Upstream also
// implements Readable, Writeable and From<u16> for it, and uses none of
// them, so the port leaves them out.
type stringID uint16

// standardStringLen is StringId::STANDARD_STRING_LEN in subsetter
// src/cff/number.rs.
const standardStringLen = 391

// isStandardString is StringId::is_standard_string in subsetter
// src/cff/number.rs.
func (s stringID) isStandardString() bool {
	return s < standardStringLen
}

// u24 is U24 in subsetter src/cff/number.rs.
type u24 uint32

// u24Max is U24::MAX in subsetter src/cff/number.rs.
const u24Max = 16777215

// readU24 is the impl Readable of U24 in subsetter src/cff/number.rs.
func readU24(r *reader) (u24, bool) {
	data, ok := r.readBytes(3)
	if !ok {
		return 0, false
	}
	return u24(uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])), true
}

// write is the impl Writeable of U24 in subsetter src/cff/number.rs.
func (v u24) write(w *writer) {
	w.writeU8(uint8(v >> 16))
	w.writeU8(uint8(v >> 8))
	w.writeU8(uint8(v))
}
