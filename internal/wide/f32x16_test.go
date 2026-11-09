package wide

import "testing"

// f32x16Vec builds and reads an F32x16 for the runners of wide_test.go. Lane
// i is lane i%8 of half i/8.
var f32x16Vec = floatVec[F32x16]{
	width: 16,
	from:  func(l []float32) F32x16 { return F32x16{F32x8(l[:8]), F32x8(l[8:])} },
	lanes: func(v F32x16) []float32 { return append(v[0][:], v[1][:]...) },
}

func TestSplatF32x16(t *testing.T) {
	for _, n := range []float32{0, negZero, 1.5, nan, -inf} {
		got := f32x16Vec.lanes(SplatF32x16(n))
		for i := range got {
			checkF32Lane(t, "SplatF32x16", i, n, got[i], n)
		}
	}
}

func TestF32x16Abs(t *testing.T) {
	runUnaryBits(t, f32x16Vec, "Abs", absCases, F32x16.Abs)
}

func TestF32x16CmpGt(t *testing.T) {
	runCmp(t, f32x16Vec, "CmpGt", func(c cmpF32) bool { return c.gt }, F32x16.CmpGt)
}

func TestF32x16Blend(t *testing.T) {
	runBlend(t, f32x16Vec, F32x16.Blend)
}

func TestF32x16Normalize(t *testing.T) {
	cases := []unaryF32{
		{-1, 0},
		{2, 1},
		{0.5, 0.5},
		{negZero, negZero},
		{nan, nan},
		{inf, 1},
		{-inf, 0},
	}
	runUnary(t, f32x16Vec, "Normalize", cases, F32x16.Normalize)
}

func TestF32x16Floor(t *testing.T) {
	// F32x16.Floor rounds with Round, so it differs from F32x8.Floor for NaN,
	// for -0 and for a magnitude of 2^23 or more.
	cases := []unaryF32{
		{0, 0},
		{negZero, negZero},
		{0.4, 0},
		{-0.4, -1},
		{0.5, 0},
		{-0.5, -1},
		{1.5, 1},
		{-1.5, -2},
		{2.5, 2},
		{-2.5, -3},
		{12345.75, 12345},
		{-12345.75, -12346},
		{minDenorm, 0},
		{-minDenorm, -1},
		{nan, nan},
		{inf, inf},
		{-inf, -inf},
		{maxF, maxF},
		{-maxF, -maxF},
		{1e10, 1e10},
		{8388607.5, 8388607},
	}
	runUnary(t, f32x16Vec, "Floor", cases, F32x16.Floor)
}

func TestF32x16Sqrt(t *testing.T) {
	runUnary(t, f32x16Vec, "Sqrt", sqrtCases, F32x16.Sqrt)
}

func TestF32x16Round(t *testing.T) {
	runUnary(t, f32x16Vec, "Round", roundCases, F32x16.Round)
}

func TestF32x16SaveToU16x16(t *testing.T) {
	cases := []struct {
		in   float32
		want uint16
	}{
		{0, 0},
		{negZero, 0},
		{0.5, 0},
		{1.9, 1},
		{-1, 0},
		{-0.5, 0},
		{255.9, 255},
		{65534.9, 65534},
		{65535.5, 65535},
		{70000, 65535},
		{nan, 0},
		{inf, 65535},
		{-inf, 0},
		{maxF, 65535},
	}
	for _, g := range pack(cases, 16) {
		in := make([]float32, 16)
		for i, c := range g {
			in[i] = c.in
		}
		got := SplatU16x16(7)
		f32x16Vec.from(in).SaveToU16x16(&got)
		for i, c := range g {
			if got[i] != c.want {
				t.Errorf("SaveToU16x16 lane %d of %v = %d, want %d", i, in, got[i], c.want)
			}
		}
	}
}

func TestF32x16Add(t *testing.T) {
	runBinary(t, f32x16Vec, "Add", addCases, F32x16.Add)
}

func TestF32x16Sub(t *testing.T) {
	runBinary(t, f32x16Vec, "Sub", subCases, F32x16.Sub)
}

func TestF32x16Mul(t *testing.T) {
	runBinary(t, f32x16Vec, "Mul", mulCases, F32x16.Mul)
}
