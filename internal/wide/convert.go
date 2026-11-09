package wide

import "math"

// The functions of this file port no upstream file. They give the result of a
// Rust cast with "as" from a float to an integer, which saturates: a value
// below the smallest integer gives the smallest integer, a value above the
// largest integer gives the largest integer, and NaN gives 0. A Go conversion
// gives an implementation-specific value for such a float (D16).
//
// Each in-range conversion goes through a wider integer, or halves a float of
// 2^62 or more, because TinyGo 0.44.0 saturates a conversion at the largest
// float below the limit of the integer. It converts 2147483520 to MaxInt32,
// not to 2147483520.

// TruncI32 is the Rust cast "f as i32" for an f32. It truncates toward zero
// and saturates.
func TruncI32(f float32) int32 {
	switch {
	case f != f:
		return 0
	case f >= 2147483648:
		return math.MaxInt32
	case f <= -2147483648:
		return math.MinInt32
	}
	return int32(int64(f))
}

// TruncU32 is the Rust cast "f as u32" for an f32. It truncates toward zero
// and saturates.
func TruncU32(f float32) uint32 {
	switch {
	case f != f, f <= 0:
		return 0
	case f >= 4294967296:
		return math.MaxUint32
	}
	return uint32(uint64(f))
}

// TruncI64 is the Rust cast "f as i64" for an f32. It truncates toward zero
// and saturates.
func TruncI64(f float32) int64 {
	switch {
	case f != f:
		return 0
	case f >= 9223372036854775807:
		return math.MaxInt64
	case f <= -9223372036854775808:
		return math.MinInt64
	case f >= 1<<62 || f <= -(1<<62):
		// f is an even integer, so half of it is exact.
		return int64(f/2) * 2
	}
	return int64(f)
}

// TruncU8 is the Rust cast "f as u8" for an f32. It truncates toward zero and
// saturates.
func TruncU8(f float32) uint8 {
	switch {
	case f != f, f <= 0:
		return 0
	case f >= 255:
		return math.MaxUint8
	}
	return uint8(f)
}

// TruncU16 is the Rust cast "f as u16" for an f32. It truncates toward zero
// and saturates.
func TruncU16(f float32) uint16 {
	switch {
	case f != f, f <= 0:
		return 0
	case f >= 65535:
		return math.MaxUint16
	}
	return uint16(f)
}

// Trunc64I32 is the Rust cast "f as i32" for an f64. It truncates toward zero
// and saturates.
func Trunc64I32(f float64) int32 {
	switch {
	case f != f:
		return 0
	case f >= 2147483647:
		return math.MaxInt32
	case f <= -2147483648:
		return math.MinInt32
	}
	return int32(f)
}

// Trunc64I64 is the Rust cast "f as i64" for an f64. It truncates toward zero
// and saturates.
func Trunc64I64(f float64) int64 {
	switch {
	case f != f:
		return 0
	case f >= 9223372036854775807:
		return math.MaxInt64
	case f <= -9223372036854775808:
		return math.MinInt64
	case f >= 1<<62 || f <= -(1<<62):
		// f is an even integer, so half of it is exact.
		return int64(f/2) * 2
	}
	return int64(f)
}
