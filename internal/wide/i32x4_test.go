package wide

import (
	"math"
	"testing"
)

// i32x4Vec builds and reads an I32x4 for the runners of wide_test.go.
var i32x4Vec = intVec[I32x4, int32]{
	width: 4,
	from:  func(l []int32) I32x4 { return I32x4(l) },
	lanes: i32x4Lanes,
}

func TestSplatI32x4(t *testing.T) {
	for _, n := range []int32{0, -1, math.MaxInt32, math.MinInt32} {
		if got := SplatI32x4(n); got != (I32x4{n, n, n, n}) {
			t.Errorf("SplatI32x4(%d) = %v", n, got)
		}
	}
}

func TestI32x4Blend(t *testing.T) {
	runIntBlend(t, i32x4Vec, i32BlendCases, I32x4.Blend)
}

func TestI32x4CmpEq(t *testing.T) {
	runIntBinary(t, i32x4Vec, "CmpEq", i32CmpEqCases, I32x4.CmpEq)
}

func TestI32x4CmpGt(t *testing.T) {
	runIntBinary(t, i32x4Vec, "CmpGt", i32CmpGtCases, I32x4.CmpGt)
}

func TestI32x4CmpLt(t *testing.T) {
	runIntBinary(t, i32x4Vec, "CmpLt", i32CmpLtCases, I32x4.CmpLt)
}

func TestI32x4ToF32x4(t *testing.T) {
	for _, g := range pack(i32ToF32Cases, 4) {
		in := make([]int32, 4)
		for i, c := range g {
			in[i] = c.in
		}
		got := f32x4Vec.lanes(i32x4Vec.from(in).ToF32x4())
		for i, c := range g {
			checkF32Lane(t, "ToF32x4", i, in, got[i], c.want)
		}
	}
}

func TestI32x4ToF32x4Bitcast(t *testing.T) {
	for _, g := range pack(i32BitsCases, 4) {
		in := make([]int32, 4)
		for i, c := range g {
			in[i] = c.in
		}
		got := f32x4Vec.lanes(i32x4Vec.from(in).ToF32x4Bitcast())
		for i, c := range g {
			checkBitsLane(t, "ToF32x4Bitcast", i, in, math.Float32bits(got[i]), c.want)
		}
	}
}

func TestI32x4Add(t *testing.T) {
	runIntBinary(t, i32x4Vec, "Add", i32AddCases, I32x4.Add)
}

func TestI32x4And(t *testing.T) {
	runIntBinary(t, i32x4Vec, "And", i32AndCases, I32x4.And)
}

func TestI32x4Mul(t *testing.T) {
	runIntBinary(t, i32x4Vec, "Mul", i32MulCases, I32x4.Mul)
}

func TestI32x4Or(t *testing.T) {
	runIntBinary(t, i32x4Vec, "Or", i32OrCases, I32x4.Or)
}

func TestI32x4Xor(t *testing.T) {
	runIntBinary(t, i32x4Vec, "Xor", i32XorCases, I32x4.Xor)
}
