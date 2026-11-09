package wide

import "testing"

// u16x16Vec builds and reads a U16x16 for the runners of wide_test.go.
var u16x16Vec = intVec[U16x16, uint16]{
	width: 16,
	from:  func(l []uint16) U16x16 { return U16x16(l) },
	lanes: func(v U16x16) []uint16 { return v[:] },
}

func TestSplatU16x16(t *testing.T) {
	for _, n := range []uint16{0, 1, 0xffff} {
		got := SplatU16x16(n)
		for i, l := range got {
			if l != n {
				t.Errorf("SplatU16x16(%d) lane %d = %d", n, i, l)
			}
		}
	}
}

func TestU16x16AsSlice(t *testing.T) {
	v := U16x16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 0xffff}
	if got := v.AsSlice(); got != [16]uint16(v) {
		t.Errorf("AsSlice() = %v, want %v", got, v)
	}
}

func TestU16x16Min(t *testing.T) {
	cases := []binaryInt[uint16]{
		{1, 2, 1},
		{2, 1, 1},
		{0, 0xffff, 0},
		{0x8000, 0x7fff, 0x7fff},
	}
	runIntBinary(t, u16x16Vec, "Min", cases, U16x16.Min)
}

func TestU16x16Max(t *testing.T) {
	cases := []binaryInt[uint16]{
		{1, 2, 2},
		{2, 1, 2},
		{0, 0xffff, 0xffff},
		{0x8000, 0x7fff, 0x8000},
	}
	runIntBinary(t, u16x16Vec, "Max", cases, U16x16.Max)
}

func TestU16x16CmpLe(t *testing.T) {
	cases := []binaryInt[uint16]{
		{1, 2, 0xffff},
		{2, 2, 0xffff},
		{3, 2, 0},
		{0x8000, 0x7fff, 0},
		{0, 0xffff, 0xffff},
	}
	runIntBinary(t, u16x16Vec, "CmpLe", cases, U16x16.CmpLe)
}

func TestU16x16Blend(t *testing.T) {
	cases := []binaryInt[uint16]{
		{0xffff, 0x1234, 0x1234},
		{0, 0x1234, 0xedcb},
		{0x00ff, 0x1234, 0xed34},
	}
	runIntBlend(t, u16x16Vec, cases, U16x16.Blend)
}

func TestU16x16Add(t *testing.T) {
	cases := []binaryInt[uint16]{
		{1, 2, 3},
		{0xffff, 1, 0},
		{0x8000, 0x8000, 0},
		{255, 255, 510},
	}
	runIntBinary(t, u16x16Vec, "Add", cases, U16x16.Add)
}

func TestU16x16Sub(t *testing.T) {
	cases := []binaryInt[uint16]{
		{3, 2, 1},
		{0, 1, 0xffff},
		{255, 256, 0xffff},
		{0x8000, 0x8000, 0},
	}
	runIntBinary(t, u16x16Vec, "Sub", cases, U16x16.Sub)
}

func TestU16x16Mul(t *testing.T) {
	cases := []binaryInt[uint16]{
		{3, 4, 12},
		{255, 255, 65025},
		{256, 256, 0},
		{0xffff, 0xffff, 1},
	}
	runIntBinary(t, u16x16Vec, "Mul", cases, U16x16.Mul)
}

func TestU16x16Div(t *testing.T) {
	cases := []binaryInt[uint16]{
		{12, 4, 3},
		{255, 2, 127},
		{65025, 255, 255},
		{0, 7, 0},
		{0xffff, 0xffff, 1},
	}
	runIntBinary(t, u16x16Vec, "Div", cases, U16x16.Div)
}

func TestU16x16DivByZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Div by a zero lane did not panic")
		}
	}()
	SplatU16x16(1).Div(U16x16{})
}

func TestU16x16And(t *testing.T) {
	cases := []binaryInt[uint16]{
		{0xffff, 0x1234, 0x1234},
		{0x0f0f, 0x1234, 0x0204},
		{0, 0xffff, 0},
	}
	runIntBinary(t, u16x16Vec, "And", cases, U16x16.And)
}

func TestU16x16Or(t *testing.T) {
	cases := []binaryInt[uint16]{
		{0, 0x1234, 0x1234},
		{0x0f0f, 0x1234, 0x1f3f},
		{0x8000, 0x7fff, 0xffff},
	}
	runIntBinary(t, u16x16Vec, "Or", cases, U16x16.Or)
}

func TestU16x16Not(t *testing.T) {
	cases := []unaryInt[uint16]{
		{0, 0xffff},
		{0xffff, 0},
		{0x1234, 0xedcb},
	}
	runIntUnary(t, u16x16Vec, "Not", cases, U16x16.Not)
}

func TestU16x16Shr(t *testing.T) {
	// The shift amount is b. Rust in a release build takes it modulo 16.
	cases := []binaryInt[uint16]{
		{0x8000, 15, 1},
		{0xffff, 8, 0xff},
		{0x1234, 0, 0x1234},
		{0x8000, 16, 0x8000},
		{0xffff, 17, 0x7fff},
	}
	runIntBinary(t, u16x16Vec, "Shr", cases, U16x16.Shr)
}

func TestU16x16Equal(t *testing.T) {
	a, same := SplatU16x16(3), SplatU16x16(3)
	if !a.Equal(same) {
		t.Error("Equal of equal vectors = false")
	}
	for lane := range 16 {
		b := a
		b[lane] = 4
		if a.Equal(b) {
			t.Errorf("Equal with lane %d changed = true", lane)
		}
	}
}
