package wide

import "testing"

// u32x4Vec builds and reads a U32x4 for the runners of wide_test.go.
var u32x4Vec = intVec[U32x4, uint32]{
	width: 4,
	from:  func(l []uint32) U32x4 { return U32x4(l) },
	lanes: func(v U32x4) []uint32 { return v[:] },
}

func TestSplatU32x4(t *testing.T) {
	for _, n := range []uint32{0, 1, 0xffffffff} {
		if got := SplatU32x4(n); got != (U32x4{n, n, n, n}) {
			t.Errorf("SplatU32x4(%d) = %v", n, got)
		}
	}
}

func TestU32x4CmpEq(t *testing.T) {
	runIntBinary(t, u32x4Vec, "CmpEq", u32CmpEqCases, U32x4.CmpEq)
}

func TestU32x4Shl(t *testing.T) {
	for _, c := range u32ShlCases {
		got := SplatU32x4(c.a).Shl(uint(c.b))
		for i, l := range got {
			if l != c.want {
				t.Errorf("Shl(%#x, %d) lane %d = %#x, want %#x", c.a, c.b, i, l, c.want)
			}
		}
	}
}

func TestU32x4Shr(t *testing.T) {
	for _, c := range u32ShrCases {
		got := SplatU32x4(c.a).Shr(uint(c.b))
		for i, l := range got {
			if l != c.want {
				t.Errorf("Shr(%#x, %d) lane %d = %#x, want %#x", c.a, c.b, i, l, c.want)
			}
		}
	}
}

func TestU32x4Not(t *testing.T) {
	runIntUnary(t, u32x4Vec, "Not", u32NotCases, U32x4.Not)
}

func TestU32x4Add(t *testing.T) {
	runIntBinary(t, u32x4Vec, "Add", u32AddCases, U32x4.Add)
}

func TestU32x4And(t *testing.T) {
	runIntBinary(t, u32x4Vec, "And", u32AndCases, U32x4.And)
}

func TestU32x4Or(t *testing.T) {
	runIntBinary(t, u32x4Vec, "Or", u32OrCases, U32x4.Or)
}
