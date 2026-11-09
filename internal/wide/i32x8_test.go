package wide

import (
	"math"
	"testing"
)

// i32x8Vec builds and reads an I32x8 for the runners of wide_test.go.
var i32x8Vec = intVec[I32x8, int32]{
	width: 8,
	from:  func(l []int32) I32x8 { return I32x8(l) },
	lanes: i32x8Lanes,
}

func TestSplatI32x8(t *testing.T) {
	for _, n := range []int32{0, -1, math.MaxInt32, math.MinInt32} {
		got := SplatI32x8(n)
		for i, l := range got {
			if l != n {
				t.Errorf("SplatI32x8(%d) lane %d = %d", n, i, l)
			}
		}
	}
}

func TestI32x8Blend(t *testing.T) {
	runIntBlend(t, i32x8Vec, i32BlendCases, I32x8.Blend)
}

func TestI32x8CmpEq(t *testing.T) {
	runIntBinary(t, i32x8Vec, "CmpEq", i32CmpEqCases, I32x8.CmpEq)
}

func TestI32x8CmpGt(t *testing.T) {
	runIntBinary(t, i32x8Vec, "CmpGt", i32CmpGtCases, I32x8.CmpGt)
}

func TestI32x8CmpLt(t *testing.T) {
	runIntBinary(t, i32x8Vec, "CmpLt", i32CmpLtCases, I32x8.CmpLt)
}

func TestI32x8ToF32x8(t *testing.T) {
	for _, g := range pack(i32ToF32Cases, 8) {
		in := make([]int32, 8)
		for i, c := range g {
			in[i] = c.in
		}
		got := f32x8Vec.lanes(i32x8Vec.from(in).ToF32x8())
		for i, c := range g {
			checkF32Lane(t, "ToF32x8", i, in, got[i], c.want)
		}
	}
}

func TestI32x8ToU32x8Bitcast(t *testing.T) {
	for _, g := range pack(i32BitsCases, 8) {
		in := make([]int32, 8)
		for i, c := range g {
			in[i] = c.in
		}
		got := i32x8Vec.from(in).ToU32x8Bitcast()
		for i, c := range g {
			checkBitsLane(t, "ToU32x8Bitcast", i, in, got[i], c.want)
		}
	}
}

func TestI32x8ToF32x8Bitcast(t *testing.T) {
	for _, g := range pack(i32BitsCases, 8) {
		in := make([]int32, 8)
		for i, c := range g {
			in[i] = c.in
		}
		got := f32x8Vec.lanes(i32x8Vec.from(in).ToF32x8Bitcast())
		for i, c := range g {
			checkBitsLane(t, "ToF32x8Bitcast", i, in, math.Float32bits(got[i]), c.want)
		}
	}
}

func TestI32x8Add(t *testing.T) {
	runIntBinary(t, i32x8Vec, "Add", i32AddCases, I32x8.Add)
}

func TestI32x8And(t *testing.T) {
	runIntBinary(t, i32x8Vec, "And", i32AndCases, I32x8.And)
}

func TestI32x8Mul(t *testing.T) {
	runIntBinary(t, i32x8Vec, "Mul", i32MulCases, I32x8.Mul)
}

func TestI32x8Or(t *testing.T) {
	runIntBinary(t, i32x8Vec, "Or", i32OrCases, I32x8.Or)
}

func TestI32x8Xor(t *testing.T) {
	runIntBinary(t, i32x8Vec, "Xor", i32XorCases, I32x8.Xor)
}
