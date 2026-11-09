package wide

// This file ports the scalar code of tiny-skia src/wide/u16x16_t.rs.
//
// No need to use explicit 256bit AVX2 SIMD.
// `-C target-cpu=native` will autovectorize it better than us.
// Not even sure why explicit instructions are so slow...
//
// On ARM AArch64 we can actually get up to 2x performance boost by using SIMD.
//
// Each method of U16x16 has its scalar code here, in a plain function such as
// addU16x16 that every build has. The doc comment of a plain function holds
// the doc comment of its method after a blank line. widegen writes the methods
// from these functions into u16x16_scalar.go and u16x16_simd.go (D15).

// U16x16 is u16x16 in tiny-skia src/wide/u16x16_t.rs: sixteen u16 lanes. A
// mask is a U16x16 whose lanes are 0xFFFF (true) or 0 (false).
type U16x16 [16]uint16

// SplatU16x16 is u16x16::splat in tiny-skia src/wide/u16x16_t.rs. It gives n
// in each lane.
func SplatU16x16(n uint16) U16x16 {
	return U16x16{n, n, n, n, n, n, n, n, n, n, n, n, n, n, n, n}
}

// asSliceU16x16 is the scalar code of U16x16.AsSlice.
//
// AsSlice is u16x16::as_slice in tiny-skia src/wide/u16x16_t.rs. It gives the
// lanes. Upstream gives a reference to them, and Go gives a copy.
func asSliceU16x16(v U16x16) [16]uint16 {
	return v
}

// minU16x16 is the scalar code of U16x16.Min.
//
// Min is u16x16::min in tiny-skia src/wide/u16x16_t.rs. Each lane is the
// smaller of the lanes of v and rhs.
func minU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = min(v[i], rhs[i])
	}
	return r
}

// maxU16x16 is the scalar code of U16x16.Max.
//
// Max is u16x16::max in tiny-skia src/wide/u16x16_t.rs. Each lane is the
// larger of the lanes of v and rhs.
func maxU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = max(v[i], rhs[i])
	}
	return r
}

// cmpLeU16x16 is the scalar code of U16x16.CmpLe.
//
// CmpLe is u16x16::cmp_le in tiny-skia src/wide/u16x16_t.rs. It gives a mask
// that is true where v <= rhs.
func cmpLeU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		if v[i] <= rhs[i] {
			r[i] = 0xFFFF
		}
	}
	return r
}

// blendU16x16 is the scalar code of U16x16.Blend.
//
// Blend is u16x16::blend in tiny-skia src/wide/u16x16_t.rs. v is a mask.
// Each lane is (t & v) | (e & !v): the bits of t where the bits of the mask
// are 1, and the bits of e where they are 0.
func blendU16x16(v, t, e U16x16) U16x16 {
	return orU16x16(andU16x16(t, v), andU16x16(e, notU16x16(v)))
}

// addU16x16 is the scalar code of U16x16.Add.
//
// Add is the Add impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It wraps on
// overflow, as Rust does in a release build.
func addU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] + rhs[i]
	}
	return r
}

// subU16x16 is the scalar code of U16x16.Sub.
//
// Sub is the Sub impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It wraps on
// overflow, as Rust does in a release build.
func subU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] - rhs[i]
	}
	return r
}

// mulU16x16 is the scalar code of U16x16.Mul.
//
// Mul is the Mul impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It keeps
// the low 16 bits of each product, as Rust does in a release build.
func mulU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] * rhs[i]
	}
	return r
}

// divU16x16 is the scalar code of U16x16.Div.
//
// Div is the Div impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It
// truncates. A lane of rhs that is 0 makes it panic, as in Rust.
func divU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] / rhs[i]
	}
	return r
}

// andU16x16 is the scalar code of U16x16.And.
//
// And is the BitAnd impl of u16x16 in tiny-skia src/wide/u16x16_t.rs.
func andU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] & rhs[i]
	}
	return r
}

// orU16x16 is the scalar code of U16x16.Or.
//
// Or is the BitOr impl of u16x16 in tiny-skia src/wide/u16x16_t.rs.
func orU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] | rhs[i]
	}
	return r
}

// notU16x16 is the scalar code of U16x16.Not.
//
// Not is the Not impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It flips
// each bit.
func notU16x16(v U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = ^v[i]
	}
	return r
}

// shrU16x16 is the scalar code of U16x16.Shr.
//
// Shr is the Shr impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It shifts
// each lane of v right by the lane of rhs. Rust in a release build uses only
// the low 4 bits of the shift, so a shift of 16 or more is taken modulo 16.
func shrU16x16(v, rhs U16x16) U16x16 {
	var r U16x16
	for i := range r {
		r[i] = v[i] >> (rhs[i] & 15)
	}
	return r
}

// equalU16x16 is the scalar code of U16x16.Equal.
//
// Equal is the PartialEq impl of u16x16 in tiny-skia src/wide/u16x16_t.rs. It
// reports whether each lane of v equals the lane of rhs.
func equalU16x16(v, rhs U16x16) bool {
	return v == rhs
}
