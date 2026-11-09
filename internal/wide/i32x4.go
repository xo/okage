package wide

import "math"

// This file ports the scalar code of tiny-skia src/wide/i32x4_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib).
//
// Each method of I32x4 has its scalar code here, in a plain function such as
// addI32x4 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into i32x4_scalar.go and i32x4_simd.go (D15).

// I32x4 is i32x4 in tiny-skia src/wide/i32x4_t.rs: four i32 lanes. A mask is
// an I32x4 whose lanes are -1 (true) or 0 (false).
type I32x4 [4]int32

// i32Mask gives the lane of a mask for b.
func i32Mask(b bool) int32 {
	if b {
		return -1
	}
	return 0
}

// SplatI32x4 is i32x4::splat in tiny-skia src/wide/i32x4_t.rs. It gives n in
// each lane.
func SplatI32x4(n int32) I32x4 {
	return I32x4{n, n, n, n}
}

// blendI32x4 is the scalar code of I32x4.Blend.
//
// Blend is i32x4::blend in tiny-skia src/wide/i32x4_t.rs. v is a mask. Each
// lane is the lane of t where the mask is true, and the lane of f where it
// is false. The scalar code blends bit by bit, with genericBitBlend.
func blendI32x4(v, t, f I32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = genericBitBlend(v[i], t[i], f[i])
	}
	return r
}

// cmpEqI32x4 is the scalar code of I32x4.CmpEq.
//
// CmpEq is i32x4::cmp_eq in tiny-skia src/wide/i32x4_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqI32x4(v, rhs I32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = i32Mask(v[i] == rhs[i])
	}
	return r
}

// cmpGtI32x4 is the scalar code of I32x4.CmpGt.
//
// CmpGt is i32x4::cmp_gt in tiny-skia src/wide/i32x4_t.rs. It gives a mask
// that is true where v > rhs.
func cmpGtI32x4(v, rhs I32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = i32Mask(v[i] > rhs[i])
	}
	return r
}

// cmpLtI32x4 is the scalar code of I32x4.CmpLt.
//
// CmpLt is i32x4::cmp_lt in tiny-skia src/wide/i32x4_t.rs. It gives a mask
// that is true where v < rhs.
func cmpLtI32x4(v, rhs I32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = i32Mask(v[i] < rhs[i])
	}
	return r
}

// toF32x4I32x4 is the scalar code of I32x4.ToF32x4.
//
// ToF32x4 is i32x4::to_f32x4 in tiny-skia src/wide/i32x4_t.rs. It converts
// each lane with "as f32", which rounds to the nearest float32, and to even
// at a halfway case.
func toF32x4I32x4(v I32x4) F32x4 {
	return F32x4{
		float32(v[0]),
		float32(v[1]),
		float32(v[2]),
		float32(v[3]),
	}
}

// toF32x4BitcastI32x4 is the scalar code of I32x4.ToF32x4Bitcast.
//
// ToF32x4Bitcast is i32x4::to_f32x4_bitcast in tiny-skia
// src/wide/i32x4_t.rs. It gives the float32 with the same bits as each lane.
func toF32x4BitcastI32x4(v I32x4) F32x4 {
	return F32x4{
		math.Float32frombits(uint32(v[0])),
		math.Float32frombits(uint32(v[1])),
		math.Float32frombits(uint32(v[2])),
		math.Float32frombits(uint32(v[3])),
	}
}

// addI32x4 is the scalar code of I32x4.Add.
//
// Add is the Add impl of i32x4 in tiny-skia src/wide/i32x4_t.rs. It wraps on
// overflow.
func addI32x4(v, rhs I32x4) I32x4 {
	return I32x4{
		v[0] + rhs[0],
		v[1] + rhs[1],
		v[2] + rhs[2],
		v[3] + rhs[3],
	}
}

// andI32x4 is the scalar code of I32x4.And.
//
// And is the BitAnd impl of i32x4 in tiny-skia src/wide/i32x4_t.rs.
func andI32x4(v, rhs I32x4) I32x4 {
	return I32x4{
		v[0] & rhs[0],
		v[1] & rhs[1],
		v[2] & rhs[2],
		v[3] & rhs[3],
	}
}

// mulI32x4 is the scalar code of I32x4.Mul.
//
// Mul is the Mul impl of i32x4 in tiny-skia src/wide/i32x4_t.rs. It keeps
// the low 32 bits of each product, so it wraps on overflow.
func mulI32x4(v, rhs I32x4) I32x4 {
	return I32x4{
		v[0] * rhs[0],
		v[1] * rhs[1],
		v[2] * rhs[2],
		v[3] * rhs[3],
	}
}

// orI32x4 is the scalar code of I32x4.Or.
//
// Or is the BitOr impl of i32x4 in tiny-skia src/wide/i32x4_t.rs.
func orI32x4(v, rhs I32x4) I32x4 {
	return I32x4{
		v[0] | rhs[0],
		v[1] | rhs[1],
		v[2] | rhs[2],
		v[3] | rhs[3],
	}
}

// xorI32x4 is the scalar code of I32x4.Xor.
//
// Xor is the BitXor impl of i32x4 in tiny-skia src/wide/i32x4_t.rs.
func xorI32x4(v, rhs I32x4) I32x4 {
	return I32x4{
		v[0] ^ rhs[0],
		v[1] ^ rhs[1],
		v[2] ^ rhs[2],
		v[3] ^ rhs[3],
	}
}
