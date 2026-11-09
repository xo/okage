package wide

// This file ports the scalar code of tiny-skia src/wide/u32x4_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib).
//
// Each method of U32x4 has its scalar code here, in a plain function such as
// addU32x4 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into u32x4_scalar.go and u32x4_simd.go (D15).

// U32x4 is u32x4 in tiny-skia src/wide/u32x4_t.rs: four u32 lanes. A mask is
// a U32x4 whose lanes have all bits 1 (true) or all bits 0 (false).
type U32x4 [4]uint32

// u32Mask gives the lane of a mask for b.
func u32Mask(b bool) uint32 {
	if b {
		return maskBits
	}
	return 0
}

// SplatU32x4 is u32x4::splat in tiny-skia src/wide/u32x4_t.rs. It gives n in
// each lane.
func SplatU32x4(n uint32) U32x4 {
	return U32x4{n, n, n, n}
}

// cmpEqU32x4 is the scalar code of U32x4.CmpEq.
//
// CmpEq is u32x4::cmp_eq in tiny-skia src/wide/u32x4_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqU32x4(v, rhs U32x4) U32x4 {
	var r U32x4
	for i := range r {
		r[i] = u32Mask(v[i] == rhs[i])
	}
	return r
}

// shlU32x4 is the scalar code of U32x4.Shl.
//
// Shl is u32x4::shl in tiny-skia src/wide/u32x4_t.rs, where the shift is the
// const generic RHS. It shifts each lane left by rhs. Rust in a release build
// uses only the low 5 bits of the shift, so a shift of 32 or more is taken
// modulo 32. Upstream only shifts by constants below 32.
func shlU32x4(v U32x4, rhs uint) U32x4 {
	u := rhs & 31
	return U32x4{
		v[0] << u,
		v[1] << u,
		v[2] << u,
		v[3] << u,
	}
}

// shrU32x4 is the scalar code of U32x4.Shr.
//
// Shr is u32x4::shr in tiny-skia src/wide/u32x4_t.rs, where the shift is the
// const generic RHS. It shifts each lane right by rhs, with zeros in the top
// bits. Rust in a release build uses only the low 5 bits of the shift, so a
// shift of 32 or more is taken modulo 32. Upstream only shifts by constants
// below 32.
func shrU32x4(v U32x4, rhs uint) U32x4 {
	u := rhs & 31
	return U32x4{
		v[0] >> u,
		v[1] >> u,
		v[2] >> u,
		v[3] >> u,
	}
}

// toI32x4Bitcast gives the int32 with the same bits as each lane. It is
// bytemuck::cast from u32x4 to i32x4 in tiny-skia src/wide/f32x4_t.rs.
func (v U32x4) toI32x4Bitcast() I32x4 {
	return I32x4{
		int32(v[0]),
		int32(v[1]),
		int32(v[2]),
		int32(v[3]),
	}
}

// notU32x4 is the scalar code of U32x4.Not.
//
// Not is the Not impl of u32x4 in tiny-skia src/wide/u32x4_t.rs. It flips
// each bit.
func notU32x4(v U32x4) U32x4 {
	return U32x4{
		^v[0],
		^v[1],
		^v[2],
		^v[3],
	}
}

// addU32x4 is the scalar code of U32x4.Add.
//
// Add is the Add impl of u32x4 in tiny-skia src/wide/u32x4_t.rs. It wraps on
// overflow.
func addU32x4(v, rhs U32x4) U32x4 {
	return U32x4{
		v[0] + rhs[0],
		v[1] + rhs[1],
		v[2] + rhs[2],
		v[3] + rhs[3],
	}
}

// andU32x4 is the scalar code of U32x4.And.
//
// And is the BitAnd impl of u32x4 in tiny-skia src/wide/u32x4_t.rs.
func andU32x4(v, rhs U32x4) U32x4 {
	return U32x4{
		v[0] & rhs[0],
		v[1] & rhs[1],
		v[2] & rhs[2],
		v[3] & rhs[3],
	}
}

// orU32x4 is the scalar code of U32x4.Or.
//
// Or is the BitOr impl of u32x4 in tiny-skia src/wide/u32x4_t.rs.
func orU32x4(v, rhs U32x4) U32x4 {
	return U32x4{
		v[0] | rhs[0],
		v[1] | rhs[1],
		v[2] | rhs[2],
		v[3] | rhs[3],
	}
}
