package wide

import "math"

// This file ports the scalar code of tiny-skia src/wide/f32x8_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib). In the scalar code of
// upstream, an f32x8 is two f32x4, and most methods call the method of
// f32x4 on each half. Each lane gives the same result in an [8]float32.
//
// Each method of F32x8 has its scalar code here, in a plain function such as
// addF32x8 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into f32x8_scalar.go and f32x8_simd.go (D15).

// F32x8 is f32x8 in tiny-skia src/wide/f32x8_t.rs: eight f32 lanes. A mask is
// an F32x8 whose lanes have all bits 1 (true) or all bits 0 (false).
type F32x8 [8]float32

// joinF32x8 gives the F32x8 with the lanes of lo and then the lanes of hi.
func joinF32x8(lo, hi F32x4) F32x8 {
	return F32x8{lo[0], lo[1], lo[2], lo[3], hi[0], hi[1], hi[2], hi[3]}
}

// halves gives the low four lanes and the high four lanes of v. They are the
// two f32x4 of the scalar f32x8 of upstream.
func (v F32x8) halves() (F32x4, F32x4) {
	return F32x4(v[:4]), F32x4(v[4:])
}

// SplatF32x8 is f32x8::splat in tiny-skia src/wide/f32x8_t.rs. It gives n in
// each lane.
func SplatF32x8(n float32) F32x8 {
	return F32x8{n, n, n, n, n, n, n, n}
}

// floorF32x8 is the scalar code of F32x8.Floor.
//
// Floor is f32x8::floor in tiny-skia src/wide/f32x8_t.rs. It truncates each
// lane to an i32, converts it back and subtracts 1 where that is larger than
// the lane. So it is not math.Floor: NaN gives 0, -0 gives 0, and a value out
// of the range of i32 gives -2147483648 or 2147483648.
func floorF32x8(v F32x8) F32x8 {
	// Port: the same algorithm as floorF32x4, lane by lane. See there.
	var y F32x8
	for i, x := range v {
		y[i] = floorLaneF32(x)
	}
	return y
}

// fractF32x8 is the scalar code of F32x8.Fract.
//
// Fract is f32x8::fract in tiny-skia src/wide/f32x8_t.rs. It gives v minus
// Floor of v.
func fractF32x8(v F32x8) F32x8 {
	return subF32x8(v, floorF32x8(v))
}

// normalizeF32x8 is the scalar code of F32x8.Normalize.
//
// Normalize is f32x8::normalize in tiny-skia src/wide/f32x8_t.rs. It clamps
// each lane to [0, 1] with Max and then Min. NaN stays NaN.
func normalizeF32x8(v F32x8) F32x8 {
	return minF32x8(maxF32x8(v, F32x8{}), SplatF32x8(1.0))
}

// toI32x8BitcastF32x8 is the scalar code of F32x8.ToI32x8Bitcast.
//
// ToI32x8Bitcast is f32x8::to_i32x8_bitcast in tiny-skia
// src/wide/f32x8_t.rs. It gives the bits of each lane as an int32.
func toI32x8BitcastF32x8(v F32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = int32(math.Float32bits(v[i]))
	}
	return r
}

// toU32x8BitcastF32x8 is the scalar code of F32x8.ToU32x8Bitcast.
//
// ToU32x8Bitcast is f32x8::to_u32x8_bitcast in tiny-skia
// src/wide/f32x8_t.rs. It gives the bits of each lane.
func toU32x8BitcastF32x8(v F32x8) U32x8 {
	var r U32x8
	for i := range r {
		r[i] = math.Float32bits(v[i])
	}
	return r
}

// cmpEqF32x8 is the scalar code of F32x8.CmpEq.
//
// CmpEq is f32x8::cmp_eq in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] == rhs[i])
	}
	return r
}

// cmpNeF32x8 is the scalar code of F32x8.CmpNe.
//
// CmpNe is f32x8::cmp_ne in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v != rhs, and so also where a lane is NaN.
func cmpNeF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] != rhs[i])
	}
	return r
}

// cmpGeF32x8 is the scalar code of F32x8.CmpGe.
//
// CmpGe is f32x8::cmp_ge in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v >= rhs.
func cmpGeF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] >= rhs[i])
	}
	return r
}

// cmpGtF32x8 is the scalar code of F32x8.CmpGt.
//
// CmpGt is f32x8::cmp_gt in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v > rhs.
func cmpGtF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] > rhs[i])
	}
	return r
}

// cmpLeF32x8 is the scalar code of F32x8.CmpLe.
//
// CmpLe is f32x8::cmp_le in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v <= rhs.
func cmpLeF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] <= rhs[i])
	}
	return r
}

// cmpLtF32x8 is the scalar code of F32x8.CmpLt.
//
// CmpLt is f32x8::cmp_lt in tiny-skia src/wide/f32x8_t.rs. It gives a mask
// that is true where v < rhs.
func cmpLtF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = f32Mask(v[i] < rhs[i])
	}
	return r
}

// blendF32x8 is the scalar code of F32x8.Blend.
//
// Blend is f32x8::blend in tiny-skia src/wide/f32x8_t.rs. v is a mask. Each
// lane is the lane of t where the mask is true, and the lane of f where it
// is false. The scalar code blends bit by bit, with genericBitBlend.
func blendF32x8(v, t, f F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(genericBitBlend(
			math.Float32bits(v[i]), math.Float32bits(t[i]), math.Float32bits(f[i])))
	}
	return r
}

// absF32x8 is the scalar code of F32x8.Abs.
//
// Abs is f32x8::abs in tiny-skia src/wide/f32x8_t.rs. It clears the sign bit
// of each lane.
func absF32x8(v F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = LaneAbs(v[i])
	}
	return r
}

// maxF32x8 is the scalar code of F32x8.Max.
//
// Max is f32x8::max in tiny-skia src/wide/f32x8_t.rs. Each lane is
// fasterMax of the lanes of v and rhs.
//
// These technically don't have the same semantics for NaN and 0, but it
// doesn't seem to matter as Skia does it the same way.
func maxF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = fasterMax(v[i], rhs[i])
	}
	return r
}

// minF32x8 is the scalar code of F32x8.Min.
//
// Min is f32x8::min in tiny-skia src/wide/f32x8_t.rs. Each lane is
// fasterMin of the lanes of v and rhs.
//
// These technically don't have the same semantics for NaN and 0, but it
// doesn't seem to matter as Skia does it the same way.
func minF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = fasterMin(v[i], rhs[i])
	}
	return r
}

// isFiniteF32x8 is the scalar code of F32x8.IsFinite.
//
// IsFinite is f32x8::is_finite in tiny-skia src/wide/f32x8_t.rs. It gives a
// mask that is true where the exponent bits of the lane are not all 1, so
// false for an infinity and for NaN.
func isFiniteF32x8(v F32x8) F32x8 {
	shiftedExpMask := SplatU32x8(0xFF000000)
	u := toU32x8BitcastF32x8(v)
	shiftU := shlU32x8(u, 1)
	out := notU32x8(cmpEqU32x8(andU32x8(shiftU, shiftedExpMask), shiftedExpMask))
	return toF32x8BitcastU32x8(out)
}

// roundF32x8 is the scalar code of F32x8.Round.
//
// Round is f32x8::round in tiny-skia src/wide/f32x8_t.rs. It calls
// F32x4.Round on each half, so it rounds half to even.
func roundF32x8(v F32x8) F32x8 {
	lo, hi := v.halves()
	return joinF32x8(roundF32x4(lo), roundF32x4(hi))
}

// roundIntF32x8 is the scalar code of F32x8.RoundInt.
//
// RoundInt is f32x8::round_int in tiny-skia src/wide/f32x8_t.rs. It calls
// F32x4.RoundInt on each half, so it rounds half to even and then converts
// with "as i32", which saturates and gives 0 for NaN.
//
// These technically don't have the same semantics for NaN and out of range
// values, but it doesn't seem to matter as Skia does it the same way.
func roundIntF32x8(v F32x8) I32x8 {
	lo, hi := v.halves()
	return joinI32x8(roundIntF32x4(lo), roundIntF32x4(hi))
}

// truncIntF32x8 is the scalar code of F32x8.TruncInt.
//
// TruncInt is f32x8::trunc_int in tiny-skia src/wide/f32x8_t.rs. It converts
// each lane with "as i32", which truncates toward zero, saturates and gives 0
// for NaN.
//
// These technically don't have the same semantics for NaN and out of range
// values, but it doesn't seem to matter as Skia does it the same way.
func truncIntF32x8(v F32x8) I32x8 {
	var r I32x8
	for i := range r {
		r[i] = TruncI32(v[i])
	}
	return r
}

// recipFastF32x8 is the scalar code of F32x8.RecipFast.
//
// RecipFast is f32x8::recip_fast in tiny-skia src/wide/f32x8_t.rs. Each lane
// is 1 / v, exact (D16).
func recipFastF32x8(v F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = 1.0 / v[i]
	}
	return r
}

// recipSqrtF32x8 is the scalar code of F32x8.RecipSqrt.
//
// RecipSqrt is f32x8::recip_sqrt in tiny-skia src/wide/f32x8_t.rs. Each lane
// is 1 / sqrt(v), exact (D16).
func recipSqrtF32x8(v F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = 1.0 / sqrt32(v[i])
	}
	return r
}

// sqrtF32x8 is the scalar code of F32x8.Sqrt.
//
// Sqrt is f32x8::sqrt in tiny-skia src/wide/f32x8_t.rs. Each lane is the
// square root of v.
func sqrtF32x8(v F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = sqrt32(v[i])
	}
	return r
}

// powfF32x8 is the scalar code of F32x8.Powf.
//
// Powf is f32x8::powf in tiny-skia src/wide/f32x8_t.rs. It gives an
// approximation of v to the power exp, from an approximation of log2 and of
// exp2. A lane that is 0 or 1 does not change. A NaN lane is first replaced
// with the NaN of powfNaNBits (D34).
func powfF32x8(v F32x8, exp float32) F32x8 {
	v = powfNaN(v)
	x := v
	// We assume sign(x) is positive so we can use vectorized i32->f32
	// conversions.
	e := mulF32x8(toF32x8I32x8(toI32x8BitcastF32x8(x)), SplatF32x8(1.0/(1<<23)))
	m := toF32x8BitcastU32x8(orU32x8(andU32x8(toU32x8BitcastF32x8(x), SplatU32x8(0x007fffff)), SplatU32x8(0x3f000000)))
	log2X := subF32x8(e, SplatF32x8(124.225514990))
	log2X = subF32x8(log2X, mulF32x8(SplatF32x8(1.498030302), m))
	log2X = subF32x8(log2X, divF32x8(SplatF32x8(1.725879990), addF32x8(SplatF32x8(0.3520887068), m)))
	x = mulF32x8(log2X, SplatF32x8(exp))
	f := subF32x8(x, floorF32x8(x))
	a := addF32x8(x, SplatF32x8(121.274057500))
	a = subF32x8(a, mulF32x8(f, SplatF32x8(1.490129070)))
	a = addF32x8(a, divF32x8(SplatF32x8(27.728023300), subF32x8(SplatF32x8(4.84252568), f)))
	a = mulF32x8(a, SplatF32x8(1<<23))
	infBits := SplatF32x8(float32(math.Float32bits(float32(math.Inf(1)))))
	x = toF32x8BitcastI32x8(roundIntF32x8(minF32x8(maxF32x8(a, SplatF32x8(0.0)), infBits)))
	skip := orF32x8(cmpEqF32x8(v, SplatF32x8(0.0)), cmpEqF32x8(v, SplatF32x8(1.0)))
	return blendF32x8(skip, v, x)
}

// powfNaNBits is the bits of the NaN that Powf reads in place of any NaN
// lane (D34). It is the NaN that x86 makes for an invalid operation, such as
// 0/0, which is what the oracle of upstream gives on amd64.
const powfNaNBits = 0xffc00000

// powfNaN replaces each NaN lane of v with the NaN of powfNaNBits (D34).
// Powf reads the bits of each lane, so the result for a NaN depends on its
// payload, which differs between architectures, and between orders of the
// operands of an addition. It ports no upstream function.
func powfNaN(v F32x8) F32x8 {
	for i, x := range v {
		if x != x {
			v[i] = math.Float32frombits(powfNaNBits)
		}
	}
	return v
}

// addF32x8 is the scalar code of F32x8.Add.
//
// Add is the Add impl of f32x8 in tiny-skia src/wide/f32x8_t.rs.
func addF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = v[i] + rhs[i]
	}
	return r
}

// subF32x8 is the scalar code of F32x8.Sub.
//
// Sub is the Sub impl of f32x8 in tiny-skia src/wide/f32x8_t.rs.
func subF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = v[i] - rhs[i]
	}
	return r
}

// mulF32x8 is the scalar code of F32x8.Mul.
//
// Mul is the Mul impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. The
// conversion to float32 rounds each product, so that the Go compiler cannot
// fuse it with a later add (D16).
func mulF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = float32(v[i] * rhs[i])
	}
	return r
}

// divF32x8 is the scalar code of F32x8.Div.
//
// Div is the Div impl of f32x8 in tiny-skia src/wide/f32x8_t.rs.
func divF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = v[i] / rhs[i]
	}
	return r
}

// andF32x8 is the scalar code of F32x8.And.
//
// And is the BitAnd impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. It works
// on the bits of each lane.
func andF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) & math.Float32bits(rhs[i]))
	}
	return r
}

// orF32x8 is the scalar code of F32x8.Or.
//
// Or is the BitOr impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. It works on
// the bits of each lane.
func orF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) | math.Float32bits(rhs[i]))
	}
	return r
}

// xorF32x8 is the scalar code of F32x8.Xor.
//
// Xor is the BitXor impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. It works
// on the bits of each lane.
func xorF32x8(v, rhs F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) ^ math.Float32bits(rhs[i]))
	}
	return r
}

// negF32x8 is the scalar code of F32x8.Neg.
//
// Neg is the Neg impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. Each lane is
// 0 - v, so 0 and -0 both give +0.
func negF32x8(v F32x8) F32x8 {
	return subF32x8(F32x8{}, v)
}

// notF32x8 is the scalar code of F32x8.Not.
//
// Not is the Not impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. It flips
// each bit of each lane.
func notF32x8(v F32x8) F32x8 {
	var r F32x8
	for i := range r {
		r[i] = math.Float32frombits(^math.Float32bits(v[i]))
	}
	return r
}

// equalF32x8 is the scalar code of F32x8.Equal.
//
// Equal is the PartialEq impl of f32x8 in tiny-skia src/wide/f32x8_t.rs. It
// reports whether each lane of v == the lane of rhs. So a NaN lane makes it
// false, and 0 equals -0.
func equalF32x8(v, rhs F32x8) bool {
	for i := range v {
		if v[i] != rhs[i] {
			return false
		}
	}
	return true
}
