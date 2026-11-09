package wide

import "math"

// This file ports the scalar code of tiny-skia src/wide/u32x8_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib). In the scalar code of
// upstream, a u32x8 is two u32x4, and each method calls the method of u32x4
// on each half. Each lane gives the same result in an [8]uint32.
//
// Each method of U32x8 has its scalar code here, in a plain function such as
// addU32x8 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into u32x8_scalar.go and u32x8_simd.go (D15).

// U32x8 is u32x8 in tiny-skia src/wide/u32x8_t.rs: eight u32 lanes. A mask is
// a U32x8 whose lanes have all bits 1 (true) or all bits 0 (false).
type U32x8 [8]uint32

// SplatU32x8 is u32x8::splat in tiny-skia src/wide/u32x8_t.rs. It gives n in
// each lane.
func SplatU32x8(n uint32) U32x8 {
	return U32x8{n, n, n, n, n, n, n, n}
}

// toI32x8BitcastU32x8 is the scalar code of U32x8.ToI32x8Bitcast.
//
// ToI32x8Bitcast is u32x8::to_i32x8_bitcast in tiny-skia
// src/wide/u32x8_t.rs. It gives the int32 with the same bits as each lane.
func toI32x8BitcastU32x8(v U32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = int32(v[i])
	}
	return r
}

// toF32x8BitcastU32x8 is the scalar code of U32x8.ToF32x8Bitcast.
//
// ToF32x8Bitcast is u32x8::to_f32x8_bitcast in tiny-skia
// src/wide/u32x8_t.rs. It gives the float32 with the same bits as each lane.
func toF32x8BitcastU32x8(v U32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(v[i])
	}
	return r
}

// cmpEqU32x8 is the scalar code of U32x8.CmpEq.
//
// CmpEq is u32x8::cmp_eq in tiny-skia src/wide/u32x8_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqU32x8(v, rhs U32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = u32Mask(v[i] == rhs[i])
	}
	return r
}

// shlU32x8 is the scalar code of U32x8.Shl.
//
// Shl is u32x8::shl in tiny-skia src/wide/u32x8_t.rs, where the shift is the
// const generic RHS. It shifts each lane left by rhs. Rust in a release build
// uses only the low 5 bits of the shift, so a shift of 32 or more is taken
// modulo 32. Upstream only shifts by constants below 32.
func shlU32x8(v U32x8, rhs uint) U32x8 {
	u := rhs & 31
	var r U32x8
	for i := range r {
		r[i] = v[i] << u
	}
	return r
}

// shrU32x8 is the scalar code of U32x8.Shr.
//
// Shr is u32x8::shr in tiny-skia src/wide/u32x8_t.rs, where the shift is the
// const generic RHS. It shifts each lane right by rhs, with zeros in the top
// bits. Rust in a release build uses only the low 5 bits of the shift, so a
// shift of 32 or more is taken modulo 32. Upstream only shifts by constants
// below 32.
func shrU32x8(v U32x8, rhs uint) U32x8 {
	u := rhs & 31
	var r U32x8
	for i := range r {
		r[i] = v[i] >> u
	}
	return r
}

// notU32x8 is the scalar code of U32x8.Not.
//
// Not is the Not impl of u32x8 in tiny-skia src/wide/u32x8_t.rs. It flips
// each bit.
func notU32x8(v U32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = ^v[i]
	}
	return r
}

// addU32x8 is the scalar code of U32x8.Add.
//
// Add is the Add impl of u32x8 in tiny-skia src/wide/u32x8_t.rs. It wraps on
// overflow.
func addU32x8(v, rhs U32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = v[i] + rhs[i]
	}
	return r
}

// andU32x8 is the scalar code of U32x8.And.
//
// And is the BitAnd impl of u32x8 in tiny-skia src/wide/u32x8_t.rs.
func andU32x8(v, rhs U32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = v[i] & rhs[i]
	}
	return r
}

// orU32x8 is the scalar code of U32x8.Or.
//
// Or is the BitOr impl of u32x8 in tiny-skia src/wide/u32x8_t.rs.
func orU32x8(v, rhs U32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = v[i] | rhs[i]
	}
	return r
}
