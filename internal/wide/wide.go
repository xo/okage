package wide

// This file ports tiny-skia src/wide/mod.rs.
//
// This module was written from scratch, therefore there is no Google
// copyright.
//
// f32x16, i32x16 and u32x16 are implemented as [Tx8; 2] and not as [T; 16].
// This way we still can use some SIMD.

// genericBitBlend is generic_bit_blend in tiny-skia src/wide/mod.rs. For each
// bit, it takes the bit of y where the bit of mask is 1, and the bit of n where
// it is 0. A vector type calls it on the bits of each lane.
func genericBitBlend[T ~uint16 | ~uint32 | ~int32](mask, y, n T) T {
	return n ^ ((n ^ y) & mask)
}

// fasterMin is FasterMinMax::faster_min for f32 in tiny-skia src/wide/mod.rs.
// It is a faster and more forgiving f32 min than the one of std, and it does
// not care about NaN: it returns rhs only if rhs < v. So if v is NaN, it
// returns v, and if rhs is NaN, it returns v. For 0 and -0, it returns v.
func fasterMin(v, rhs float32) float32 {
	if rhs < v {
		return rhs
	}
	return v
}

// fasterMax is FasterMinMax::faster_max for f32 in tiny-skia src/wide/mod.rs.
// It is a faster and more forgiving f32 max than the one of std, and it does
// not care about NaN: it returns rhs only if v < rhs. So if v is NaN, it
// returns v, and if rhs is NaN, it returns v. For 0 and -0, it returns v.
func fasterMax(v, rhs float32) float32 {
	if v < rhs {
		return rhs
	}
	return v
}

// maskBits is the bits of a lane that is true in a mask: all bits are 1. The
// lane of a false mask has all bits 0.
const maskBits = 0xFFFFFFFF
