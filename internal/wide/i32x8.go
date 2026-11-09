package wide

import "math"

// This file ports the scalar code of tiny-skia src/wide/i32x8_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib). In the scalar code of
// upstream, an i32x8 is two i32x4, and each method calls the method of i32x4
// on each half. Each lane gives the same result in an [8]int32.
//
// Each method of I32x8 has its scalar code here, in a plain function such as
// addI32x8 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into i32x8_scalar.go and i32x8_simd.go (D15).

// I32x8 is i32x8 in tiny-skia src/wide/i32x8_t.rs: eight i32 lanes. A mask is
// an I32x8 whose lanes are -1 (true) or 0 (false).
type I32x8 [8]int32

// joinI32x8 gives the I32x8 with the lanes of lo and then the lanes of hi.
func joinI32x8(lo, hi I32x4) I32x8 {
	return I32x8{lo[0], lo[1], lo[2], lo[3], hi[0], hi[1], hi[2], hi[3]}
}

// SplatI32x8 is i32x8::splat in tiny-skia src/wide/i32x8_t.rs. It gives n in
// each lane.
func SplatI32x8(n int32) I32x8 {
	return I32x8{n, n, n, n, n, n, n, n}
}

// blendI32x8 is the scalar code of I32x8.Blend.
//
// Blend is i32x8::blend in tiny-skia src/wide/i32x8_t.rs. v is a mask. Each
// lane is the lane of t where the mask is true, and the lane of f where it
// is false. The scalar code blends bit by bit, with genericBitBlend.
func blendI32x8(v, t, f I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = genericBitBlend(v[i], t[i], f[i])
	}
	return r
}

// cmpEqI32x8 is the scalar code of I32x8.CmpEq.
//
// CmpEq is i32x8::cmp_eq in tiny-skia src/wide/i32x8_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = i32Mask(v[i] == rhs[i])
	}
	return r
}

// cmpGtI32x8 is the scalar code of I32x8.CmpGt.
//
// CmpGt is i32x8::cmp_gt in tiny-skia src/wide/i32x8_t.rs. It gives a mask
// that is true where v > rhs.
func cmpGtI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = i32Mask(v[i] > rhs[i])
	}
	return r
}

// cmpLtI32x8 is the scalar code of I32x8.CmpLt.
//
// CmpLt is i32x8::cmp_lt in tiny-skia src/wide/i32x8_t.rs. It gives a mask
// that is true where v < rhs.
func cmpLtI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = i32Mask(v[i] < rhs[i])
	}
	return r
}

// toF32x8I32x8 is the scalar code of I32x8.ToF32x8.
//
// ToF32x8 is i32x8::to_f32x8 in tiny-skia src/wide/i32x8_t.rs. It converts
// each lane with "as f32", which rounds to the nearest float32, and to even
// at a halfway case.
func toF32x8I32x8(v I32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = float32(v[i])
	}
	return r
}

// toU32x8BitcastI32x8 is the scalar code of I32x8.ToU32x8Bitcast.
//
// ToU32x8Bitcast is i32x8::to_u32x8_bitcast in tiny-skia
// src/wide/i32x8_t.rs. It gives the uint32 with the same bits as each lane.
func toU32x8BitcastI32x8(v I32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = uint32(v[i])
	}
	return r
}

// toF32x8BitcastI32x8 is the scalar code of I32x8.ToF32x8Bitcast.
//
// ToF32x8Bitcast is i32x8::to_f32x8_bitcast in tiny-skia
// src/wide/i32x8_t.rs. It gives the float32 with the same bits as each lane.
func toF32x8BitcastI32x8(v I32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(uint32(v[i]))
	}
	return r
}

// addI32x8 is the scalar code of I32x8.Add.
//
// Add is the Add impl of i32x8 in tiny-skia src/wide/i32x8_t.rs. It wraps on
// overflow.
func addI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = v[i] + rhs[i]
	}
	return r
}

// andI32x8 is the scalar code of I32x8.And.
//
// And is the BitAnd impl of i32x8 in tiny-skia src/wide/i32x8_t.rs.
func andI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = v[i] & rhs[i]
	}
	return r
}

// mulI32x8 is the scalar code of I32x8.Mul.
//
// Mul is the Mul impl of i32x8 in tiny-skia src/wide/i32x8_t.rs. It keeps
// the low 32 bits of each product, so it wraps on overflow.
func mulI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = v[i] * rhs[i]
	}
	return r
}

// orI32x8 is the scalar code of I32x8.Or.
//
// Or is the BitOr impl of i32x8 in tiny-skia src/wide/i32x8_t.rs.
func orI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = v[i] | rhs[i]
	}
	return r
}

// xorI32x8 is the scalar code of I32x8.Xor.
//
// Xor is the BitXor impl of i32x8 in tiny-skia src/wide/i32x8_t.rs.
func xorI32x8(v, rhs I32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = v[i] ^ rhs[i]
	}
	return r
}
