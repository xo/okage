package wide

import "math"

// This file ports the scalar code of tiny-skia src/wide/f32x4_t.rs, which is
// based on https://github.com/Lokathor/wide (Zlib).
//
// Each method of F32x4 has its scalar code here, in a plain function such as
// addF32x4 that every build has. The doc comment of a plain function holds the
// doc comment of its method after a blank line. widegen writes the methods
// from these functions into f32x4_scalar.go and f32x4_simd.go (D15).

// F32x4 is f32x4 in tiny-skia src/wide/f32x4_t.rs: four f32 lanes. A mask is
// an F32x4 whose lanes have all bits 1 (true) or all bits 0 (false).
type F32x4 [4]float32

// epsilon is f32::EPSILON in Rust.
const epsilon = 0x1p-23

// f32Mask gives the lane of a mask for b.
func f32Mask(b bool) float32 {
	if b {
		return math.Float32frombits(maskBits)
	}
	return 0
}

// SplatF32x4 is f32x4::splat in tiny-skia src/wide/f32x4_t.rs. It gives n in
// each lane.
func SplatF32x4(n float32) F32x4 {
	return F32x4{n, n, n, n}
}

// floorF32x4 is the scalar code of F32x4.Floor.
//
// Floor is f32x4::floor in tiny-skia src/wide/f32x4_t.rs. It truncates each
// lane to an i32, converts it back and subtracts 1 where that is larger than
// the lane. So it is not math.Floor: NaN gives 0, -0 gives 0, and a value out
// of the range of i32 gives -2147483648 or 2147483648.
func floorF32x4(v F32x4) F32x4 {
	// Port: upstream computes this with vector operations on the lanes. The
	// loop below gives the same bits for each lane, and the Go compiler makes
	// much faster code of it. TestFloorExhaustive checks both on each of the
	// 2^32 values of a lane against floorF32x4Upstream, the literal port.
	var y F32x4
	for i, x := range v {
		y[i] = floorLaneF32(x)
	}
	return y
}

// floorLaneF32 is one lane of floorF32x4 and floorF32x8. It converts x with
// "as i32" and back, and subtracts 1 if that is above x. So NaN gives 0, -0
// gives +0, and a value out of the range of i32 gives the bound.
func floorLaneF32(x float32) float32 {
	r := float32(TruncI32(x))
	if r > x {
		r -= 1.0
	}
	return r
}

// LaneFloor is one lane of the Floor method of F32x4 and F32x8:
// floorLaneF32 of x. A stage that works on one lane at a time calls it in
// place of Floor (D33).
func LaneFloor(x float32) float32 {
	return floorLaneF32(x)
}

// absF32x4 is the scalar code of F32x4.Abs.
//
// Abs is f32x4::abs in tiny-skia src/wide/f32x4_t.rs. It clears the sign bit
// of each lane.
func absF32x4(v F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = LaneAbs(v[i])
	}
	return r
}

// LaneAbs is one lane of the Abs method of F32x4, F32x8 and F32x16. It
// clears the sign bit of x. A stage that works on one lane at a time calls it
// in place of Abs (D33).
func LaneAbs(x float32) float32 {
	return math.Float32frombits(math.Float32bits(x) & math.MaxInt32)
}

// maxF32x4 is the scalar code of F32x4.Max.
//
// Max is f32x4::max in tiny-skia src/wide/f32x4_t.rs. Each lane is
// fasterMax of the lanes of v and rhs.
//
// These technically don't have the same semantics for NaN and 0, but it
// doesn't seem to matter as Skia does it the same way.
func maxF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		fasterMax(v[0], rhs[0]),
		fasterMax(v[1], rhs[1]),
		fasterMax(v[2], rhs[2]),
		fasterMax(v[3], rhs[3]),
	}
}

// minF32x4 is the scalar code of F32x4.Min.
//
// Min is f32x4::min in tiny-skia src/wide/f32x4_t.rs. Each lane is
// fasterMin of the lanes of v and rhs.
//
// These technically don't have the same semantics for NaN and 0, but it
// doesn't seem to matter as Skia does it the same way.
func minF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		fasterMin(v[0], rhs[0]),
		fasterMin(v[1], rhs[1]),
		fasterMin(v[2], rhs[2]),
		fasterMin(v[3], rhs[3]),
	}
}

// cmpEqF32x4 is the scalar code of F32x4.CmpEq.
//
// CmpEq is f32x4::cmp_eq in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v == rhs.
func cmpEqF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] == rhs[i])
	}
	return r
}

// cmpNeF32x4 is the scalar code of F32x4.CmpNe.
//
// CmpNe is f32x4::cmp_ne in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v != rhs, and so also where a lane is NaN.
func cmpNeF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] != rhs[i])
	}
	return r
}

// cmpGeF32x4 is the scalar code of F32x4.CmpGe.
//
// CmpGe is f32x4::cmp_ge in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v >= rhs.
func cmpGeF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] >= rhs[i])
	}
	return r
}

// cmpGtF32x4 is the scalar code of F32x4.CmpGt.
//
// CmpGt is f32x4::cmp_gt in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v > rhs.
func cmpGtF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] > rhs[i])
	}
	return r
}

// cmpLeF32x4 is the scalar code of F32x4.CmpLe.
//
// CmpLe is f32x4::cmp_le in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v <= rhs.
func cmpLeF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] <= rhs[i])
	}
	return r
}

// cmpLtF32x4 is the scalar code of F32x4.CmpLt.
//
// CmpLt is f32x4::cmp_lt in tiny-skia src/wide/f32x4_t.rs. It gives a mask
// that is true where v < rhs.
func cmpLtF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = f32Mask(v[i] < rhs[i])
	}
	return r
}

// blendF32x4 is the scalar code of F32x4.Blend.
//
// Blend is f32x4::blend in tiny-skia src/wide/f32x4_t.rs. v is a mask. Each
// lane is the lane of t where the mask is true, and the lane of f where it
// is false. The scalar code blends bit by bit, with genericBitBlend.
func blendF32x4(v, t, f F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = math.Float32frombits(genericBitBlend(
			math.Float32bits(v[i]), math.Float32bits(t[i]), math.Float32bits(f[i])))
	}
	return r
}

// roundF32x4 is the scalar code of F32x4.Round.
//
// Round is f32x4::round in tiny-skia src/wide/f32x4_t.rs. It rounds each lane
// to the nearest integer, and to the even integer at a halfway case. A lane
// with a magnitude of 2^23 or more, NaN or an infinity does not change. A lane
// with a magnitude below 0.5 gives 0 with its sign. -0.5 gives +0, not -0,
// because Neg gives +0 for 0.
func roundF32x4(v F32x4) F32x4 {
	// Port: upstream rounds with about 25 vector operations on the lanes. The
	// code below gives the same bits for each lane, and the Go compiler makes
	// much faster code of it. TestRoundExhaustive checks both on each of the
	// 2^32 values of a lane against roundF32x4Upstream, the literal port.
	var y F32x4
	for i, x := range v {
		y[i] = roundLaneF32(x)
	}
	return y
}

// roundLaneF32 is one lane of roundF32x4. A lane with a magnitude of 2^23 or
// more, NaN or an infinity does not change. A lane with a magnitude below 0.5
// gives 0 with its sign. Any other lane rounds half to even, and a negative
// lane gives 0 - r, so -0.5 gives +0.
func roundLaneF32(x float32) float32 {
	b := math.Float32bits(x)
	e := (b >> 23) & 0xff
	if e >= 0x7f+23 {
		return x
	}
	if e < 0x7f-1 {
		return x * 0
	}
	r := float32(math.RoundToEven(float64(math.Float32frombits(b &^ (1 << 31)))))
	if b>>31 != 0 {
		return 0 - r
	}
	return r
}

// LaneRound is one lane of the Round method of F32x4 and F32x8:
// roundLaneF32 of x. A stage that works on one lane at a time calls it in
// place of Round (D33).
func LaneRound(x float32) float32 {
	return roundLaneF32(x)
}

// LaneRoundInt is one lane of the RoundInt method of F32x4 and F32x8: x
// rounded with LaneRound and then converted with TruncI32. A stage that works
// on one lane at a time calls it in place of RoundInt (D33).
func LaneRoundInt(x float32) int32 {
	// Port: a value in the range of i32 rounds half to even with
	// math.RoundToEven, which the Go compiler makes one instruction. It gives
	// the integer of roundLaneF32, because that rounds the magnitude half to
	// even and keeps the sign. TestRoundIntExhaustive checks each of the 2^32
	// values. NaN and a value out of range take the slow path.
	if x > -2147483648 && x < 2147483648 {
		return int32(int64(math.RoundToEven(float64(x))))
	}
	return laneRoundIntSlow(x)
}

// laneRoundIntSlow is the path of LaneRoundInt for NaN and for a value out of
// the range of i32. It is a function of its own, so that the Go compiler can
// inline LaneRoundInt.
func laneRoundIntSlow(x float32) int32 {
	return TruncI32(roundLaneF32(x))
}

// roundIntF32x4 is the scalar code of F32x4.RoundInt.
//
// RoundInt is f32x4::round_int in tiny-skia src/wide/f32x4_t.rs. It rounds
// each lane with Round, which rounds half to even, and then converts it with
// "as i32", which saturates and gives 0 for NaN.
//
// These technically don't have the same semantics for NaN and out of range
// values, but it doesn't seem to matter as Skia does it the same way.
func roundIntF32x4(v F32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = LaneRoundInt(v[i])
	}
	return r
}

// truncIntF32x4 is the scalar code of F32x4.TruncInt.
//
// TruncInt is f32x4::trunc_int in tiny-skia src/wide/f32x4_t.rs. It converts
// each lane with "as i32", which truncates toward zero, saturates and gives 0
// for NaN.
//
// These technically don't have the same semantics for NaN and out of range
// values, but it doesn't seem to matter as Skia does it the same way.
func truncIntF32x4(v F32x4) I32x4 {
	var r I32x4
	for i := range r {
		r[i] = TruncI32(v[i])
	}
	return r
}

// recipFastF32x4 is the scalar code of F32x4.RecipFast.
//
// RecipFast is f32x4::recip_fast in tiny-skia src/wide/f32x4_t.rs. Each lane
// is 1 / v, exact (D16).
func recipFastF32x4(v F32x4) F32x4 {
	return F32x4{
		1.0 / v[0],
		1.0 / v[1],
		1.0 / v[2],
		1.0 / v[3],
	}
}

// recipSqrtF32x4 is the scalar code of F32x4.RecipSqrt.
//
// RecipSqrt is f32x4::recip_sqrt in tiny-skia src/wide/f32x4_t.rs. Each lane
// is 1 / sqrt(v), exact (D16).
func recipSqrtF32x4(v F32x4) F32x4 {
	return F32x4{
		1.0 / sqrt32(v[0]),
		1.0 / sqrt32(v[1]),
		1.0 / sqrt32(v[2]),
		1.0 / sqrt32(v[3]),
	}
}

// sqrtF32x4 is the scalar code of F32x4.Sqrt.
//
// Sqrt is f32x4::sqrt in tiny-skia src/wide/f32x4_t.rs. Each lane is the
// square root of v.
func sqrtF32x4(v F32x4) F32x4 {
	return F32x4{
		sqrt32(v[0]),
		sqrt32(v[1]),
		sqrt32(v[2]),
		sqrt32(v[3]),
	}
}

// sqrt32 is f32::sqrt in Rust. The square root of a float64 that is converted
// to float32 is the correctly rounded square root of the float32.
func sqrt32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

// LaneSqrt is one lane of the Sqrt method of F32x4, F32x8 and F32x16:
// sqrt32 of x. A stage that works on one lane at a time calls it in place of
// Sqrt (D33).
func LaneSqrt(x float32) float32 {
	return sqrt32(x)
}

// toU32x4Bitcast gives the bits of each lane. It is bytemuck::cast from
// f32x4 to u32x4 in tiny-skia src/wide/f32x4_t.rs.
func (v F32x4) toU32x4Bitcast() U32x4 {
	return U32x4{
		math.Float32bits(v[0]),
		math.Float32bits(v[1]),
		math.Float32bits(v[2]),
		math.Float32bits(v[3]),
	}
}

// addF32x4 is the scalar code of F32x4.Add.
//
// Add is the Add impl of f32x4 in tiny-skia src/wide/f32x4_t.rs.
func addF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		v[0] + rhs[0],
		v[1] + rhs[1],
		v[2] + rhs[2],
		v[3] + rhs[3],
	}
}

// subF32x4 is the scalar code of F32x4.Sub.
//
// Sub is the Sub impl of f32x4 in tiny-skia src/wide/f32x4_t.rs.
func subF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		v[0] - rhs[0],
		v[1] - rhs[1],
		v[2] - rhs[2],
		v[3] - rhs[3],
	}
}

// mulF32x4 is the scalar code of F32x4.Mul.
//
// Mul is the Mul impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. The
// conversion to float32 rounds each product, so that the Go compiler cannot
// fuse it with a later add (D16).
func mulF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		float32(v[0] * rhs[0]),
		float32(v[1] * rhs[1]),
		float32(v[2] * rhs[2]),
		float32(v[3] * rhs[3]),
	}
}

// divF32x4 is the scalar code of F32x4.Div.
//
// Div is the Div impl of f32x4 in tiny-skia src/wide/f32x4_t.rs.
func divF32x4(v, rhs F32x4) F32x4 {
	return F32x4{
		v[0] / rhs[0],
		v[1] / rhs[1],
		v[2] / rhs[2],
		v[3] / rhs[3],
	}
}

// andF32x4 is the scalar code of F32x4.And.
//
// And is the BitAnd impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. It works
// on the bits of each lane.
func andF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) & math.Float32bits(rhs[i]))
	}
	return r
}

// orF32x4 is the scalar code of F32x4.Or.
//
// Or is the BitOr impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. It works on
// the bits of each lane.
func orF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) | math.Float32bits(rhs[i]))
	}
	return r
}

// xorF32x4 is the scalar code of F32x4.Xor.
//
// Xor is the BitXor impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. It works
// on the bits of each lane.
func xorF32x4(v, rhs F32x4) F32x4 {
	var r F32x4
	for i := range r {
		r[i] = math.Float32frombits(math.Float32bits(v[i]) ^ math.Float32bits(rhs[i]))
	}
	return r
}

// negF32x4 is the scalar code of F32x4.Neg.
//
// Neg is the Neg impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. Each lane is
// 0 - v, so 0 and -0 both give +0.
func negF32x4(v F32x4) F32x4 {
	return subF32x4(F32x4{}, v)
}

// notF32x4 is the scalar code of F32x4.Not.
//
// Not is the Not impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. It flips
// each bit of each lane.
func notF32x4(v F32x4) F32x4 {
	return xorF32x4(v, SplatF32x4(math.Float32frombits(maskBits)))
}

// equalF32x4 is the scalar code of F32x4.Equal.
//
// Equal is the PartialEq impl of f32x4 in tiny-skia src/wide/f32x4_t.rs. It
// reports whether each lane of v == the lane of rhs. So a NaN lane makes it
// false, and 0 equals -0.
func equalF32x4(v, rhs F32x4) bool {
	return v[0] == rhs[0] && v[1] == rhs[1] && v[2] == rhs[2] && v[3] == rhs[3]
}
