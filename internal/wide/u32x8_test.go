package wide

import (
	"math"
	"testing"
)

// u32x8Vec builds and reads a U32x8 for the runners of wide_test.go.
var u32x8Vec = intVec[U32x8, uint32]{
	width: 8,
	from:  func(l []uint32) U32x8 { return U32x8(l) },
	lanes: func(v U32x8) []uint32 { return v[:] },
}

func TestSplatU32x8(t *testing.T) {
	for _, n := range []uint32{0, 1, 0xffffffff} {
		got := SplatU32x8(n)
		for i, l := range got {
			if l != n {
				t.Errorf("SplatU32x8(%d) lane %d = %d", n, i, l)
			}
		}
	}
}

func TestU32x8ToI32x8Bitcast(t *testing.T) {
	cases := []struct {
		in   uint32
		want int32
	}{
		{0, 0},
		{0xffffffff, -1},
		{0x80000000, math.MinInt32},
		{0x7fffffff, math.MaxInt32},
	}
	for _, g := range pack(cases, 8) {
		in := make([]uint32, 8)
		for i, c := range g {
			in[i] = c.in
		}
		got := i32x8Lanes(u32x8Vec.from(in).ToI32x8Bitcast())
		for i, c := range g {
			checkI32Lane(t, "ToI32x8Bitcast", i, in, got[i], c.want)
		}
	}
}

func TestU32x8ToF32x8Bitcast(t *testing.T) {
	cases := []uint32{0, 0x80000000, 0x3f800000, 0x7f800000, 0x7fc00000, 0xffffffff, 1, 0xff7fffff}
	got := f32x8Vec.lanes(u32x8Vec.from(cases).ToF32x8Bitcast())
	for i, want := range cases {
		checkBitsLane(t, "ToF32x8Bitcast", i, cases, math.Float32bits(got[i]), want)
	}
}

func TestU32x8CmpEq(t *testing.T) {
	runIntBinary(t, u32x8Vec, "CmpEq", u32CmpEqCases, U32x8.CmpEq)
}

func TestU32x8Shl(t *testing.T) {
	for _, c := range u32ShlCases {
		got := u32x8Vec.lanes(SplatU32x8(c.a).Shl(uint(c.b)))
		for i, l := range got {
			if l != c.want {
				t.Errorf("Shl(%#x, %d) lane %d = %#x, want %#x", c.a, c.b, i, l, c.want)
			}
		}
	}
}

func TestU32x8Shr(t *testing.T) {
	for _, c := range u32ShrCases {
		got := u32x8Vec.lanes(SplatU32x8(c.a).Shr(uint(c.b)))
		for i, l := range got {
			if l != c.want {
				t.Errorf("Shr(%#x, %d) lane %d = %#x, want %#x", c.a, c.b, i, l, c.want)
			}
		}
	}
}

func TestU32x8Not(t *testing.T) {
	runIntUnary(t, u32x8Vec, "Not", u32NotCases, U32x8.Not)
}

func TestU32x8Add(t *testing.T) {
	runIntBinary(t, u32x8Vec, "Add", u32AddCases, U32x8.Add)
}

func TestU32x8And(t *testing.T) {
	runIntBinary(t, u32x8Vec, "And", u32AndCases, U32x8.And)
}

func TestU32x8Or(t *testing.T) {
	runIntBinary(t, u32x8Vec, "Or", u32OrCases, U32x8.Or)
}
