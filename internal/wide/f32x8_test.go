package wide

import (
	"math"
	"testing"
)

// f32x8Vec builds and reads an F32x8 for the runners of wide_test.go.
var f32x8Vec = floatVec[F32x8]{
	width: 8,
	from:  func(l []float32) F32x8 { return F32x8(l) },
	lanes: func(v F32x8) []float32 { return v[:] },
}

func i32x8Lanes(v I32x8) []int32 { return v[:] }

func TestSplatF32x8(t *testing.T) {
	for _, n := range []float32{0, negZero, 1.5, nan, -inf} {
		got := SplatF32x8(n)
		for i := range got {
			checkF32Lane(t, "SplatF32x8", i, n, got[i], n)
		}
	}
}

func TestF32x8Floor(t *testing.T) {
	runUnary(t, f32x8Vec, "Floor", floorTruncCases, F32x8.Floor)
}

func TestF32x8Fract(t *testing.T) {
	cases := []unaryF32{
		{0, 0},
		{negZero, negZero},
		{1.25, 0.25},
		{-1.25, 0.75},
		{-0.5, 0.5},
		{nan, nan},
		{inf, inf},
		{-inf, -inf},
		{maxF, maxF},
		{3e9, 852516352},
		{belowI32Max, 0},
	}
	runUnary(t, f32x8Vec, "Fract", cases, F32x8.Fract)
}

func TestF32x8Normalize(t *testing.T) {
	cases := []unaryF32{
		{-1, 0},
		{2, 1},
		{0.5, 0.5},
		{0, 0},
		{negZero, negZero},
		{1, 1},
		{nan, nan},
		{inf, 1},
		{-inf, 0},
		{minDenorm, minDenorm},
	}
	runUnary(t, f32x8Vec, "Normalize", cases, F32x8.Normalize)
}

func TestF32x8ToI32x8Bitcast(t *testing.T) {
	cases := []f32ToI32{
		{0, 0},
		{negZero, math.MinInt32},
		{1, 0x3f800000},
		{-1, -0x40800000},
		{inf, 0x7f800000},
		{f32From(0xffffffff), -1},
	}
	runToI32(t, f32x8Vec, "ToI32x8Bitcast", cases, F32x8.ToI32x8Bitcast, i32x8Lanes)
}

func TestF32x8ToU32x8Bitcast(t *testing.T) {
	cases := []unaryBits{
		{0x00000000, 0x00000000},
		{0x80000000, 0x80000000},
		{0x3f800000, 0x3f800000},
		{0xffffffff, 0xffffffff},
		{0x7fc00000, 0x7fc00000},
	}
	for _, g := range pack(cases, 8) {
		in := make([]uint32, 8)
		for i, c := range g {
			in[i] = c.in
		}
		got := f32x8Vec.fromBits(in).ToU32x8Bitcast()
		for i, c := range g {
			checkBitsLane(t, "ToU32x8Bitcast", i, in, got[i], c.want)
		}
	}
}

func TestF32x8CmpEq(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpEq", func(c cmpF32) bool { return c.eq }, F32x8.CmpEq)
}

func TestF32x8CmpNe(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpNe", func(c cmpF32) bool { return c.ne }, F32x8.CmpNe)
}

func TestF32x8CmpGe(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpGe", func(c cmpF32) bool { return c.ge }, F32x8.CmpGe)
}

func TestF32x8CmpGt(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpGt", func(c cmpF32) bool { return c.gt }, F32x8.CmpGt)
}

func TestF32x8CmpLe(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpLe", func(c cmpF32) bool { return c.le }, F32x8.CmpLe)
}

func TestF32x8CmpLt(t *testing.T) {
	runCmp(t, f32x8Vec, "CmpLt", func(c cmpF32) bool { return c.lt }, F32x8.CmpLt)
}

func TestF32x8Blend(t *testing.T) {
	runBlend(t, f32x8Vec, F32x8.Blend)
}

func TestF32x8Abs(t *testing.T) {
	runUnaryBits(t, f32x8Vec, "Abs", absCases, F32x8.Abs)
}

func TestF32x8Max(t *testing.T) {
	runBinary(t, f32x8Vec, "Max", maxCases, F32x8.Max)
}

func TestF32x8Min(t *testing.T) {
	runBinary(t, f32x8Vec, "Min", minCases, F32x8.Min)
}

func TestF32x8IsFinite(t *testing.T) {
	cases := []unaryBits{
		{0x00000000, maskBits},
		{0x80000000, maskBits},
		{0x3f800000, maskBits},
		{0x00000001, maskBits},
		{0x7f7fffff, maskBits},
		{0xff7fffff, maskBits},
		{0x7f800000, 0},
		{0xff800000, 0},
		{0x7fc00000, 0},
		{0xffffffff, 0},
	}
	runUnaryBits(t, f32x8Vec, "IsFinite", cases, F32x8.IsFinite)
}

func TestF32x8Round(t *testing.T) {
	runUnary(t, f32x8Vec, "Round", roundCases, F32x8.Round)
}

func TestF32x8RoundInt(t *testing.T) {
	runToI32(t, f32x8Vec, "RoundInt", roundIntCases, F32x8.RoundInt, i32x8Lanes)
}

func TestF32x8TruncInt(t *testing.T) {
	runToI32(t, f32x8Vec, "TruncInt", truncIntCases, F32x8.TruncInt, i32x8Lanes)
}

func TestF32x8RecipFast(t *testing.T) {
	runUnary(t, f32x8Vec, "RecipFast", recipFastCases, F32x8.RecipFast)
}

func TestF32x8RecipSqrt(t *testing.T) {
	runUnary(t, f32x8Vec, "RecipSqrt", recipSqrtCases, F32x8.RecipSqrt)
}

func TestF32x8Sqrt(t *testing.T) {
	runUnary(t, f32x8Vec, "Sqrt", sqrtCases, F32x8.Sqrt)
}

func TestF32x8Powf(t *testing.T) {
	// The expected bits come from f32x8::powf of tiny-skia 0.12.0, built
	// without the feature simd. The exponents are the ones that the stages
	// of highp use.
	tests := []struct {
		x, exp float32
		want   uint32
	}{
		{0, 2.2, 0x00000000},
		{1, 2.2, 0x3f800000},
		{negZero, 2.2, 0x80000000},
		{0.5, 2.2, 0x3e5eda40},
		{0.25, 2.2, 0x3d420300},
		{0.75, 2.2, 0x3f07f080},
		{0.1, 2.2, 0x3bcebac0},
		{2, 2.2, 0x40930680},
		{4, 2.2, 0x41a8e500},
		{0.9, 2.2, 0x3f4afd80},
		{0.01, 2.2, 0x3826f740},
		{0.002, 2.2, 0x359ae380},
		{100, 2.2, 0x46c43a80},
		{0.333, 2.2, 0x3db64340},
		{0.6, 2.2, 0x3ea66440},
		{0.05, 2.2, 0x3ab3f900},
		{0.5, 0.45454545, 0x3f3ad080},
		{0.25, 0.45454545, 0x3f085180},
		{0.75, 0.45454545, 0x3f609c40},
		{0.1, 0.45454545, 0x3eb3c540},
		{2, 0.45454545, 0x3faf67c0},
		{4, 0.45454545, 0x3ff05bc0},
		{0.9, 0.45454545, 0x3f740300},
		{0.01, 0.45454545, 0x3dfc7b40},
		{0.5, 2.4, 0x3e4202c0},
		{0.75, 2.4, 0x3f005800},
		{0.1, 2.4, 0x3b826fc0},
		{100, 2.4, 0x47767280},
		{0.002, 2.4, 0x34b2c580},
		{0.5, 0.416666666, 0x3f3fc8c0},
		{0.1, 0.416666666, 0x3ec42700},
		{0.002, 0.416666666, 0x3d99b8c0},
		{100, 0.416666666, 0x40da0100},
	}
	for _, test := range tests {
		// Put the value in each lane in turn, with 0.5 in the other lanes.
		for lane := range 8 {
			in := []float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}
			in[lane] = test.x
			got := f32x8Vec.lanes(f32x8Vec.from(in).Powf(test.exp))
			if b := math.Float32bits(got[lane]); b != test.want {
				t.Errorf("Powf(%v, %v) lane %d = %#08x, want %#08x", test.x, test.exp, lane, b, test.want)
			}
		}
	}
}

func TestF32x8Add(t *testing.T) {
	runBinary(t, f32x8Vec, "Add", addCases, F32x8.Add)
}

func TestF32x8Sub(t *testing.T) {
	runBinary(t, f32x8Vec, "Sub", subCases, F32x8.Sub)
}

func TestF32x8Mul(t *testing.T) {
	runBinary(t, f32x8Vec, "Mul", mulCases, F32x8.Mul)
}

func TestF32x8Div(t *testing.T) {
	runBinary(t, f32x8Vec, "Div", divCases, F32x8.Div)
}

func TestF32x8And(t *testing.T) {
	runBinaryBits(t, f32x8Vec, "And", andCases, F32x8.And)
}

func TestF32x8Or(t *testing.T) {
	runBinaryBits(t, f32x8Vec, "Or", orCases, F32x8.Or)
}

func TestF32x8Xor(t *testing.T) {
	runBinaryBits(t, f32x8Vec, "Xor", xorCases, F32x8.Xor)
}

func TestF32x8Neg(t *testing.T) {
	runUnary(t, f32x8Vec, "Neg", negCases, F32x8.Neg)
}

func TestF32x8Not(t *testing.T) {
	runUnaryBits(t, f32x8Vec, "Not", notCases, F32x8.Not)
}

func TestF32x8Equal(t *testing.T) {
	runEqual(t, f32x8Vec, F32x8.Equal)
}
