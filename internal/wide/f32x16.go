package wide

import "math"

// This file ports tiny-skia src/wide/f32x16_t.rs. It has no SIMD code in
// upstream: an f32x16 is two f32x8, and most methods call the method of f32x8
// on each half.
//
// Each method of F32x16 has its scalar code here, in a plain function such as
// addF32x16 that every build has. The doc comment of a plain function holds
// the doc comment of its method after a blank line. widegen writes the methods
// from these functions into f32x16_scalar.go and f32x16_simd.go (D15).

// F32x16 is f32x16 in tiny-skia src/wide/f32x16_t.rs: sixteen f32 lanes, as
// two F32x8. A mask is an F32x16 whose lanes have all bits 1 (true) or all
// bits 0 (false).
type F32x16 [2]F32x8

// SplatF32x16 is f32x16::splat in tiny-skia src/wide/f32x16_t.rs. It gives n
// in each lane.
func SplatF32x16(n float32) F32x16 {
	return F32x16{SplatF32x8(n), SplatF32x8(n)}
}

// absF32x16 is the scalar code of F32x16.Abs.
//
// Abs is f32x16::abs in tiny-skia src/wide/f32x16_t.rs. It clears the sign
// bit of each lane.
func absF32x16(v F32x16) F32x16 {
	// Yes, Skia does it in the same way.
	var r F32x16
	for h := range r {
		for i := range r[h] {
			r[h][i] = math.Float32frombits(math.Float32bits(v[h][i]) & 0x7fffffff)
		}
	}
	return r
}

// cmpGtF32x16 is the scalar code of F32x16.CmpGt.
//
// CmpGt is f32x16::cmp_gt in tiny-skia src/wide/f32x16_t.rs. It gives a mask
// that is true where v > rhs.
func cmpGtF32x16(v, rhs F32x16) F32x16 {
	return F32x16{cmpGtF32x8(v[0], rhs[0]), cmpGtF32x8(v[1], rhs[1])}
}

// blendF32x16 is the scalar code of F32x16.Blend.
//
// Blend is f32x16::blend in tiny-skia src/wide/f32x16_t.rs. v is a mask. Each
// lane is the lane of t where the mask is true, and the lane of f where it
// is false.
func blendF32x16(v, t, f F32x16) F32x16 {
	return F32x16{blendF32x8(v[0], t[0], f[0]), blendF32x8(v[1], t[1], f[1])}
}

// normalizeF32x16 is the scalar code of F32x16.Normalize.
//
// Normalize is f32x16::normalize in tiny-skia src/wide/f32x16_t.rs. It
// clamps each lane to [0, 1], as F32x8.Normalize does.
func normalizeF32x16(v F32x16) F32x16 {
	return F32x16{normalizeF32x8(v[0]), normalizeF32x8(v[1])}
}

// floorF32x16 is the scalar code of F32x16.Floor.
//
// Floor is f32x16::floor in tiny-skia src/wide/f32x16_t.rs. It rounds each
// lane with Round and subtracts 1 where that is larger than the lane. So it
// differs from F32x8.Floor: NaN, an infinity and a magnitude of 2^23 or more
// do not change.
func floorF32x16(v F32x16) F32x16 {
	// Yes, Skia does it in the same way.
	roundtrip := roundF32x16(v)
	return subF32x16(roundtrip, blendF32x16(cmpGtF32x16(roundtrip, v), SplatF32x16(1.0), SplatF32x16(0.0)))
}

// sqrtF32x16 is the scalar code of F32x16.Sqrt.
//
// Sqrt is f32x16::sqrt in tiny-skia src/wide/f32x16_t.rs. Each lane is the
// square root of v.
func sqrtF32x16(v F32x16) F32x16 {
	return F32x16{sqrtF32x8(v[0]), sqrtF32x8(v[1])}
}

// roundF32x16 is the scalar code of F32x16.Round.
//
// Round is f32x16::round in tiny-skia src/wide/f32x16_t.rs. It rounds half to
// even, as F32x4.Round does.
func roundF32x16(v F32x16) F32x16 {
	return F32x16{roundF32x8(v[0]), roundF32x8(v[1])}
}

// saveToU16x16F32x16 is the scalar code of F32x16.SaveToU16x16.
//
// SaveToU16x16 is f32x16::save_to_u16x16 in tiny-skia src/wide/f32x16_t.rs.
// It converts each lane with "as u16", which truncates toward zero,
// saturates to [0, 65535] and gives 0 for NaN, and stores it in dst.
func saveToU16x16F32x16(v F32x16, dst *U16x16) {
	// Do not use to_i32x8, because it involves rounding,
	// and Skia cast's without it.
	for i := range 8 {
		dst[i] = TruncU16(v[0][i])
		dst[8+i] = TruncU16(v[1][i])
	}
}

// addF32x16 is the scalar code of F32x16.Add.
//
// Add is the Add impl of f32x16 in tiny-skia src/wide/f32x16_t.rs.
func addF32x16(v, rhs F32x16) F32x16 {
	return F32x16{addF32x8(v[0], rhs[0]), addF32x8(v[1], rhs[1])}
}

// subF32x16 is the scalar code of F32x16.Sub.
//
// Sub is the Sub impl of f32x16 in tiny-skia src/wide/f32x16_t.rs.
func subF32x16(v, rhs F32x16) F32x16 {
	return F32x16{subF32x8(v[0], rhs[0]), subF32x8(v[1], rhs[1])}
}

// mulF32x16 is the scalar code of F32x16.Mul.
//
// Mul is the Mul impl of f32x16 in tiny-skia src/wide/f32x16_t.rs.
func mulF32x16(v, rhs F32x16) F32x16 {
	return F32x16{mulF32x8(v[0], rhs[0]), mulF32x8(v[1], rhs[1])}
}
