package wide

import (
	"math"
	"testing"
)

// The tests of the vector types are tables of lanes. A test packs the lanes
// of a table into vectors, runs the method on each vector and compares each
// lane. The expected values come from the scalar code of tiny-skia 0.12.0.
// The same tables run against the scalar build and the SIMD build.
//
// A float result is compared by its bits, so 0 and -0 differ. If the expected
// result is NaN, any NaN passes, because the bits of a NaN that an operation
// makes differ between amd64 and arm64.

var (
	nan       = float32(math.NaN())
	inf       = float32(math.Inf(1))
	negZero   = float32(math.Copysign(0, -1))
	maxF      = float32(math.MaxFloat32)
	minDenorm = float32(math.SmallestNonzeroFloat32)
	minNormal = float32(0x1p-126)
)

// Values at the bounds of int32 as float32. 2147483520 is the largest
// float32 below 2^31, and -2147483904 is the next float32 below -2^31.
const (
	belowI32Max = 2147483520
	twoPow31    = 2147483648
	belowI32Min = -2147483904
)

// sameF32 reports whether got has the bits of want, or whether both are NaN.
func sameF32(got, want float32) bool {
	if want != want {
		return got != got
	}
	return math.Float32bits(got) == math.Float32bits(want)
}

// f32From gives the float32 with the bits b.
func f32From(b uint32) float32 {
	return math.Float32frombits(b)
}

// maskOf gives the bits of a lane of a float mask for b.
func maskOf(b bool) uint32 {
	if b {
		return maskBits
	}
	return 0
}

// pack splits cases into groups of n lanes. If the last group is short, it
// repeats cases from the start to fill it.
func pack[T any](cases []T, n int) [][]T {
	var groups [][]T
	for start := 0; start < len(cases); start += n {
		g := make([]T, n)
		for i := range g {
			g[i] = cases[(start+i)%len(cases)]
		}
		groups = append(groups, g)
	}
	return groups
}

// unaryF32 is one lane of a method that takes a float lane and gives a float
// lane.
type unaryF32 struct {
	in, want float32
}

// binaryF32 is one lane of a method that takes two float lanes and gives a
// float lane.
type binaryF32 struct {
	a, b, want float32
}

// unaryBits is one lane of a method that works on the bits of a lane.
type unaryBits struct {
	in, want uint32
}

// binaryBits is one lane of a method that works on the bits of two lanes.
type binaryBits struct {
	a, b, want uint32
}

// f32ToI32 is one lane of a method that converts a float lane to an int32.
type f32ToI32 struct {
	in   float32
	want int32
}

// cmpF32 is one lane of the comparisons of two float lanes.
type cmpF32 struct {
	a, b                   float32
	eq, ne, ge, gt, le, lt bool
}

// blendBits is one lane of a blend: the mask, the true lane, the false lane
// and the result, as bits.
type blendBits struct {
	mask, t, f, want uint32
}

// The tables below hold the lanes that F32x4, F32x8 and F32x16 share.

// floorTruncCases is Floor of F32x4 and F32x8, which truncates to i32.
var floorTruncCases = []unaryF32{
	{0, 0},
	{negZero, 0},
	{0.5, 0},
	{-0.5, -1},
	{1.5, 1},
	{-1.5, -2},
	{2.5, 2},
	{-2.5, -3},
	{0.4, 0},
	{-0.4, -1},
	{12345.75, 12345},
	{-12345.75, -12346},
	{minDenorm, 0},
	{-minDenorm, -1},
	{nan, 0},
	{inf, twoPow31},
	{-inf, -twoPow31},
	{maxF, twoPow31},
	{-maxF, -twoPow31},
	{belowI32Max, belowI32Max},
	{twoPow31, twoPow31},
	{-twoPow31, -twoPow31},
	{belowI32Min, -twoPow31},
	{3e9, twoPow31},
	{1e10, twoPow31},
	{8388607.5, 8388607},
	{-8388607.5, -8388608},
}

// roundCases is Round, which rounds half to even.
var roundCases = []unaryF32{
	{0, 0},
	{negZero, negZero},
	{0.4, 0},
	{-0.4, negZero},
	{0.5, 0},
	{-0.5, 0},
	{1.5, 2},
	{-1.5, -2},
	{2.5, 2},
	{-2.5, -2},
	{3.5, 4},
	{0.49999997, 0},
	{1.4999999, 1},
	{8388607.5, 8388608},
	{-8388607.5, -8388608},
	{8388608, 8388608},
	{12345.75, 12346},
	{-12345.75, -12346},
	{nan, nan},
	{inf, inf},
	{-inf, -inf},
	{maxF, maxF},
	{-maxF, -maxF},
	{minDenorm, 0},
	{-minDenorm, negZero},
	{minNormal, 0},
	{belowI32Max, belowI32Max},
	{1e10, 1e10},
}

// roundIntCases is RoundInt: Round, then a saturating conversion.
var roundIntCases = []f32ToI32{
	{0, 0},
	{negZero, 0},
	{0.5, 0},
	{1.5, 2},
	{2.5, 2},
	{-0.5, 0},
	{-1.5, -2},
	{-2.5, -2},
	{12345.75, 12346},
	{nan, 0},
	{inf, math.MaxInt32},
	{-inf, math.MinInt32},
	{maxF, math.MaxInt32},
	{-maxF, math.MinInt32},
	{belowI32Max, belowI32Max},
	{twoPow31, math.MaxInt32},
	{-twoPow31, math.MinInt32},
	{3e9, math.MaxInt32},
	{-3e9, math.MinInt32},
}

// truncIntCases is TruncInt, a saturating conversion that truncates.
var truncIntCases = []f32ToI32{
	{0, 0},
	{negZero, 0},
	{0.5, 0},
	{-0.5, 0},
	{1.5, 1},
	{-1.5, -1},
	{2.5, 2},
	{12345.75, 12345},
	{-12345.75, -12345},
	{nan, 0},
	{inf, math.MaxInt32},
	{-inf, math.MinInt32},
	{maxF, math.MaxInt32},
	{-maxF, math.MinInt32},
	{belowI32Max, belowI32Max},
	{twoPow31, math.MaxInt32},
	{-twoPow31, math.MinInt32},
	{belowI32Min, math.MinInt32},
}

// recipFastCases is RecipFast, an exact 1 / x.
var recipFastCases = []unaryF32{
	{2, 0.5},
	{-4, -0.25},
	{1, 1},
	{0, inf},
	{negZero, -inf},
	{inf, 0},
	{-inf, negZero},
	{nan, nan},
	{3, f32From(0x3eaaaaab)},
	{maxF, f32From(0x00200000)},
	{minDenorm, inf},
}

// recipSqrtCases is RecipSqrt, an exact 1 / sqrt(x).
var recipSqrtCases = []unaryF32{
	{4, 0.5},
	{0.25, 2},
	{1, 1},
	{0, inf},
	{negZero, -inf},
	{-1, nan},
	{inf, 0},
	{-inf, nan},
	{nan, nan},
	{2, f32From(0x3f3504f3)},
}

// sqrtCases is Sqrt.
var sqrtCases = []unaryF32{
	{4, 2},
	{0.25, 0.5},
	{0, 0},
	{negZero, negZero},
	{-1, nan},
	{inf, inf},
	{-inf, nan},
	{nan, nan},
	{2, f32From(0x3fb504f3)},
	{maxF, f32From(0x5f7fffff)},
}

// negCases is Neg, which is 0 - x.
var negCases = []unaryF32{
	{0, 0},
	{negZero, 0},
	{1, -1},
	{-1, 1},
	{inf, -inf},
	{-inf, inf},
	{nan, nan},
	{maxF, -maxF},
}

// absCases is Abs, as bits.
var absCases = []unaryBits{
	{0x00000000, 0x00000000},
	{0x80000000, 0x00000000},
	{0xbf800000, 0x3f800000},
	{0x3f800000, 0x3f800000},
	{0xff800000, 0x7f800000},
	{0x7fc00000, 0x7fc00000},
	{0xffc00000, 0x7fc00000},
	{0xffffffff, 0x7fffffff},
	{0x80000001, 0x00000001},
}

// notCases is Not, as bits.
var notCases = []unaryBits{
	{0x00000000, 0xffffffff},
	{0xffffffff, 0x00000000},
	{0x3f800000, 0xc07fffff},
	{0x7fc00000, 0x803fffff},
}

// addCases is Add.
var addCases = []binaryF32{
	{1, 2, 3},
	{inf, -inf, nan},
	{maxF, maxF, inf},
	{-maxF, -maxF, -inf},
	{0, negZero, 0},
	{negZero, negZero, negZero},
	{nan, 1, nan},
	{1, nan, nan},
	{minDenorm, minDenorm, f32From(2)},
	{1, 0x1p-24, 1},
	{1, 0x1p-23, 1 + 0x1p-23},
}

// subCases is Sub.
var subCases = []binaryF32{
	{3, 1, 2},
	{1, 1, 0},
	{0, 0, 0},
	{negZero, 0, negZero},
	{0, negZero, 0},
	{negZero, negZero, 0},
	{inf, inf, nan},
	{-maxF, maxF, -inf},
	{nan, 0, nan},
}

// mulCases is Mul.
var mulCases = []binaryF32{
	{2, 3, 6},
	{-3, 0.5, -1.5},
	{0, inf, nan},
	{negZero, 1, negZero},
	{maxF, 2, inf},
	{-inf, -inf, inf},
	{minDenorm, 0.5, 0},
	{nan, 0, nan},
	{1 + 0x1p-23, 1 + 0x1p-23, 1 + 0x1p-22},
}

// divCases is Div.
var divCases = []binaryF32{
	{6, 3, 2},
	{1, 3, f32From(0x3eaaaaab)},
	{1, 0, inf},
	{-1, 0, -inf},
	{0, 0, nan},
	{inf, inf, nan},
	{1, -inf, negZero},
	{negZero, 5, negZero},
	{nan, 1, nan},
}

// maxCases is Max, with fasterMax: it gives b only if a < b.
var maxCases = []binaryF32{
	{1, 2, 2},
	{2, 1, 2},
	{nan, 1, nan},
	{1, nan, 1},
	{0, negZero, 0},
	{negZero, 0, negZero},
	{-inf, inf, inf},
	{maxF, inf, inf},
	{-1, 0, 0},
}

// minCases is Min, with fasterMin: it gives b only if b < a.
var minCases = []binaryF32{
	{1, 2, 1},
	{2, 1, 1},
	{nan, 1, nan},
	{1, nan, 1},
	{0, negZero, 0},
	{negZero, 0, negZero},
	{-inf, inf, -inf},
	{maxF, inf, maxF},
	{2, 1.5, 1.5},
}

// cmpCases is CmpEq, CmpNe, CmpGe, CmpGt, CmpLe and CmpLt.
var cmpCases = []cmpF32{
	{1, 1, true, false, true, false, true, false},
	{1, 2, false, true, false, false, true, true},
	{2, 1, false, true, true, true, false, false},
	{0, negZero, true, false, true, false, true, false},
	{nan, nan, false, true, false, false, false, false},
	{nan, 1, false, true, false, false, false, false},
	{1, nan, false, true, false, false, false, false},
	{inf, inf, true, false, true, false, true, false},
	{-inf, inf, false, true, false, false, true, true},
	{maxF, inf, false, true, false, false, true, true},
}

// blendCases is Blend of the float types, as bits. A mask lane has all bits
// 1 or all bits 0.
var blendCases = []blendBits{
	{maskBits, 0x3f800000, 0x40000000, 0x3f800000},
	{0, 0x3f800000, 0x40000000, 0x40000000},
	{maskBits, 0x7fc00000, 0x80000000, 0x7fc00000},
	{0, 0x7fc00000, 0x80000000, 0x80000000},
	{maskBits, 0x80000000, 0x00000000, 0x80000000},
	{0, 0xffffffff, 0x12345678, 0x12345678},
}

// andCases, orCases and xorCases are And, Or and Xor of the float types, as
// bits.
var (
	andCases = []binaryBits{
		{0xffffffff, 0x3f800000, 0x3f800000},
		{0x7fffffff, 0xbf800000, 0x3f800000},
		{0x00000000, 0xffffffff, 0x00000000},
		{0x12345678, 0x0f0f0f0f, 0x02040608},
	}
	orCases = []binaryBits{
		{0x00000000, 0x3f800000, 0x3f800000},
		{0x80000000, 0x3f800000, 0xbf800000},
		{0x12345678, 0x0f0f0f0f, 0x1f3f5f7f},
		{0xffffffff, 0x00000000, 0xffffffff},
	}
	xorCases = []binaryBits{
		{0x80000000, 0x3f800000, 0xbf800000},
		{0xffffffff, 0xffffffff, 0x00000000},
		{0x12345678, 0x0f0f0f0f, 0x1d3b5977},
		{0x00000000, 0x7fc00000, 0x7fc00000},
	}
)

// checkF32Lane reports a lane of a float result that is not want.
func checkF32Lane(t *testing.T, method string, lane int, in any, got, want float32) {
	t.Helper()
	if !sameF32(got, want) {
		t.Errorf("%s lane %d of %v = %v (%#08x), want %v (%#08x)",
			method, lane, in, got, math.Float32bits(got), want, math.Float32bits(want))
	}
}

// checkBitsLane reports a lane of a result that does not have the bits want.
func checkBitsLane(t *testing.T, method string, lane int, in any, got, want uint32) {
	t.Helper()
	if got != want {
		t.Errorf("%s lane %d of %v = %#08x, want %#08x", method, lane, in, got, want)
	}
}

// checkI32Lane reports a lane of an int32 result that is not want.
func checkI32Lane(t *testing.T, method string, lane int, in any, got, want int32) {
	t.Helper()
	if got != want {
		t.Errorf("%s lane %d of %v = %d, want %d", method, lane, in, got, want)
	}
}

func TestGenericBitBlend(t *testing.T) {
	tests := []struct {
		mask, y, n, want uint32
	}{
		{0xffffffff, 0x12345678, 0x9abcdef0, 0x12345678},
		{0x00000000, 0x12345678, 0x9abcdef0, 0x9abcdef0},
		{0x0000ffff, 0x12345678, 0x9abcdef0, 0x9abc5678},
		{0xf0f0f0f0, 0xffffffff, 0x00000000, 0xf0f0f0f0},
	}
	for _, test := range tests {
		if got := genericBitBlend(test.mask, test.y, test.n); got != test.want {
			t.Errorf("genericBitBlend(%#x, %#x, %#x) = %#x, want %#x",
				test.mask, test.y, test.n, got, test.want)
		}
	}
}

func TestFasterMin(t *testing.T) {
	for _, test := range minCases {
		if got := fasterMin(test.a, test.b); !sameF32(got, test.want) {
			t.Errorf("fasterMin(%v, %v) = %v, want %v", test.a, test.b, got, test.want)
		}
	}
}

func TestFasterMax(t *testing.T) {
	for _, test := range maxCases {
		if got := fasterMax(test.a, test.b); !sameF32(got, test.want) {
			t.Errorf("fasterMax(%v, %v) = %v, want %v", test.a, test.b, got, test.want)
		}
	}
}

// floatVec describes a float vector type to the runners below: its width,
// how to build it from lanes, and how to read its lanes.
type floatVec[V any] struct {
	width int
	from  func([]float32) V
	lanes func(V) []float32
}

// fromBits builds a vector from the bits of its lanes.
func (fv floatVec[V]) fromBits(bits []uint32) V {
	l := make([]float32, len(bits))
	for i, b := range bits {
		l[i] = math.Float32frombits(b)
	}
	return fv.from(l)
}

// runUnary runs op on the lanes of cases and compares each lane.
func runUnary[V any](t *testing.T, fv floatVec[V], method string, cases []unaryF32, op func(V) V) {
	t.Helper()
	for _, g := range pack(cases, fv.width) {
		in := make([]float32, len(g))
		for i, c := range g {
			in[i] = c.in
		}
		got := fv.lanes(op(fv.from(in)))
		for i, c := range g {
			checkF32Lane(t, method, i, in, got[i], c.want)
		}
	}
}

// runBinary runs op on the lanes of cases and compares each lane.
func runBinary[V any](t *testing.T, fv floatVec[V], method string, cases []binaryF32, op func(V, V) V) {
	t.Helper()
	for _, g := range pack(cases, fv.width) {
		a := make([]float32, len(g))
		b := make([]float32, len(g))
		for i, c := range g {
			a[i], b[i] = c.a, c.b
		}
		got := fv.lanes(op(fv.from(a), fv.from(b)))
		for i, c := range g {
			checkF32Lane(t, method, i, [2][]float32{a, b}, got[i], c.want)
		}
	}
}

// runUnaryBits runs op on the bits of the lanes of cases and compares the
// bits of each lane.
func runUnaryBits[V any](t *testing.T, fv floatVec[V], method string, cases []unaryBits, op func(V) V) {
	t.Helper()
	for _, g := range pack(cases, fv.width) {
		in := make([]uint32, len(g))
		for i, c := range g {
			in[i] = c.in
		}
		got := fv.lanes(op(fv.fromBits(in)))
		for i, c := range g {
			checkBitsLane(t, method, i, in, math.Float32bits(got[i]), c.want)
		}
	}
}

// runBinaryBits runs op on the bits of the lanes of cases and compares the
// bits of each lane.
func runBinaryBits[V any](t *testing.T, fv floatVec[V], method string, cases []binaryBits, op func(V, V) V) {
	t.Helper()
	for _, g := range pack(cases, fv.width) {
		a := make([]uint32, len(g))
		b := make([]uint32, len(g))
		for i, c := range g {
			a[i], b[i] = c.a, c.b
		}
		got := fv.lanes(op(fv.fromBits(a), fv.fromBits(b)))
		for i, c := range g {
			checkBitsLane(t, method, i, [2][]uint32{a, b}, math.Float32bits(got[i]), c.want)
		}
	}
}

// runCmp runs the comparison op on the lanes of cmpCases and compares each
// lane with the mask that pick gives.
func runCmp[V any](t *testing.T, fv floatVec[V], method string, pick func(cmpF32) bool, op func(V, V) V) {
	t.Helper()
	for _, g := range pack(cmpCases, fv.width) {
		a := make([]float32, len(g))
		b := make([]float32, len(g))
		for i, c := range g {
			a[i], b[i] = c.a, c.b
		}
		got := fv.lanes(op(fv.from(a), fv.from(b)))
		for i, c := range g {
			checkBitsLane(t, method, i, [2][]float32{a, b}, math.Float32bits(got[i]), maskOf(pick(c)))
		}
	}
}

// runBlend runs Blend on the lanes of blendCases and compares the bits of
// each lane.
func runBlend[V any](t *testing.T, fv floatVec[V], op func(m, tv, f V) V) {
	t.Helper()
	for _, g := range pack(blendCases, fv.width) {
		m := make([]uint32, len(g))
		tv := make([]uint32, len(g))
		f := make([]uint32, len(g))
		for i, c := range g {
			m[i], tv[i], f[i] = c.mask, c.t, c.f
		}
		got := fv.lanes(op(fv.fromBits(m), fv.fromBits(tv), fv.fromBits(f)))
		for i, c := range g {
			checkBitsLane(t, "Blend", i, [3][]uint32{m, tv, f}, math.Float32bits(got[i]), c.want)
		}
	}
}

// runToI32 runs the conversion op on the lanes of cases and compares each
// lane. lanes reads the lanes of the result.
func runToI32[V, R any](t *testing.T, fv floatVec[V], method string, cases []f32ToI32, op func(V) R, lanes func(R) []int32) {
	t.Helper()
	for _, g := range pack(cases, fv.width) {
		in := make([]float32, len(g))
		for i, c := range g {
			in[i] = c.in
		}
		got := lanes(op(fv.from(in)))
		for i, c := range g {
			checkI32Lane(t, method, i, in, got[i], c.want)
		}
	}
}

// runEqual tests the Equal method of a float vector type. It changes one lane
// at a time.
func runEqual[V any](t *testing.T, fv floatVec[V], equal func(V, V) bool) {
	t.Helper()
	base := make([]float32, fv.width)
	for i := range base {
		base[i] = float32(i) + 0.5
	}
	if !equal(fv.from(base), fv.from(base)) {
		t.Errorf("Equal(%v, %v) = false, want true", base, base)
	}
	for lane := range fv.width {
		tests := []struct {
			a, b float32
			want bool
		}{
			{1, 2, false},
			{0, negZero, true},
			{nan, nan, false},
			{inf, inf, true},
		}
		for _, test := range tests {
			a := append([]float32(nil), base...)
			b := append([]float32(nil), base...)
			a[lane], b[lane] = test.a, test.b
			if got := equal(fv.from(a), fv.from(b)); got != test.want {
				t.Errorf("Equal(%v, %v) = %v, want %v", a, b, got, test.want)
			}
		}
	}
}

// intLane is the type of a lane of an integer vector.
type intLane interface {
	~int32 | ~uint32 | ~uint16
}

// binaryInt is one lane of a method that takes two integer lanes and gives
// an integer lane.
type binaryInt[E intLane] struct {
	a, b, want E
}

// unaryInt is one lane of a method that takes an integer lane and gives an
// integer lane.
type unaryInt[E intLane] struct {
	in, want E
}

// intVec describes an integer vector type to the runners below.
type intVec[V any, E intLane] struct {
	width int
	from  func([]E) V
	lanes func(V) []E
}

// runIntBinary runs op on the lanes of cases and compares each lane.
func runIntBinary[V any, E intLane](t *testing.T, iv intVec[V, E], method string, cases []binaryInt[E], op func(V, V) V) {
	t.Helper()
	for _, g := range pack(cases, iv.width) {
		a := make([]E, len(g))
		b := make([]E, len(g))
		for i, c := range g {
			a[i], b[i] = c.a, c.b
		}
		got := iv.lanes(op(iv.from(a), iv.from(b)))
		for i, c := range g {
			if got[i] != c.want {
				t.Errorf("%s lane %d of %v, %v = %#x, want %#x", method, i, a, b, got[i], c.want)
			}
		}
	}
}

// runIntUnary runs op on the lanes of cases and compares each lane.
func runIntUnary[V any, E intLane](t *testing.T, iv intVec[V, E], method string, cases []unaryInt[E], op func(V) V) {
	t.Helper()
	for _, g := range pack(cases, iv.width) {
		in := make([]E, len(g))
		for i, c := range g {
			in[i] = c.in
		}
		got := iv.lanes(op(iv.from(in)))
		for i, c := range g {
			if got[i] != c.want {
				t.Errorf("%s lane %d of %v = %#x, want %#x", method, i, in, got[i], c.want)
			}
		}
	}
}

// runIntBlend runs a blend on the lanes of cases, where a is the mask, b is
// the lane for true, and the lane for false is the bits of b flipped.
func runIntBlend[V any, E intLane](t *testing.T, iv intVec[V, E], cases []binaryInt[E], op func(m, tv, f V) V) {
	t.Helper()
	for _, g := range pack(cases, iv.width) {
		m := make([]E, len(g))
		tv := make([]E, len(g))
		f := make([]E, len(g))
		for i, c := range g {
			m[i], tv[i], f[i] = c.a, c.b, ^c.b
		}
		got := iv.lanes(op(iv.from(m), iv.from(tv), iv.from(f)))
		for i, c := range g {
			if got[i] != c.want {
				t.Errorf("Blend lane %d of %v, %v, %v = %#x, want %#x", i, m, tv, f, got[i], c.want)
			}
		}
	}
}

// The tables below hold the lanes that I32x4 and I32x8 share.

var (
	i32AddCases = []binaryInt[int32]{
		{1, 2, 3},
		{-1, 1, 0},
		{math.MaxInt32, 1, math.MinInt32},
		{math.MinInt32, -1, math.MaxInt32},
		{math.MinInt32, math.MinInt32, 0},
	}
	i32MulCases = []binaryInt[int32]{
		{3, -4, -12},
		{46341, 46341, -2147479015},
		{math.MinInt32, -1, math.MinInt32},
		{65536, 65536, 0},
		{math.MaxInt32, 2, -2},
	}
	i32AndCases = []binaryInt[int32]{
		{-1, 0x12345678, 0x12345678},
		{0x0f0f0f0f, 0x12345678, 0x02040608},
		{math.MinInt32, -1, math.MinInt32},
		{0, -1, 0},
	}
	i32OrCases = []binaryInt[int32]{
		{0, 0x12345678, 0x12345678},
		{0x0f0f0f0f, 0x12345678, 0x1f3f5f7f},
		{math.MinInt32, math.MaxInt32, -1},
	}
	i32XorCases = []binaryInt[int32]{
		{-1, -1, 0},
		{0x0f0f0f0f, 0x12345678, 0x1d3b5977},
		{math.MinInt32, math.MaxInt32, -1},
	}
	// The comparisons give a mask: -1 for true and 0 for false.
	i32CmpEqCases = []binaryInt[int32]{
		{1, 1, -1},
		{1, 2, 0},
		{math.MinInt32, math.MaxInt32, 0},
		{math.MinInt32, math.MinInt32, -1},
		{-1, 0, 0},
	}
	i32CmpGtCases = []binaryInt[int32]{
		{1, 1, 0},
		{2, 1, -1},
		{math.MinInt32, math.MaxInt32, 0},
		{math.MaxInt32, math.MinInt32, -1},
		{-1, 0, 0},
	}
	i32CmpLtCases = []binaryInt[int32]{
		{1, 1, 0},
		{1, 2, -1},
		{math.MinInt32, math.MaxInt32, -1},
		{math.MaxInt32, math.MinInt32, 0},
		{-1, 0, -1},
	}
	// i32BlendCases is a blend: the mask, the true lane and the result. The
	// false lane is the true lane with its bits flipped.
	i32BlendCases = []binaryInt[int32]{
		{-1, 0x12345678, 0x12345678},
		{0, 0x12345678, ^0x12345678},
		{-1, math.MinInt32, math.MinInt32},
		{0, math.MinInt32, math.MaxInt32},
	}
)

// i32ToF32Cases is the conversion of an int32 lane to float32, which rounds
// to nearest, with a halfway case to even.
var i32ToF32Cases = []struct {
	in   int32
	want float32
}{
	{0, 0},
	{-1, -1},
	{12345, 12345},
	{16777217, 16777216},
	{16777219, 16777220},
	{-16777217, -16777216},
	{math.MaxInt32, twoPow31},
	{math.MinInt32, -twoPow31},
}

// i32BitsCases is the bitcast of an int32 lane to a float32 lane.
var i32BitsCases = []struct {
	in   int32
	want uint32
}{
	{0, 0},
	{-1, 0xffffffff},
	{0x3f800000, 0x3f800000},
	{math.MinInt32, 0x80000000},
	{0x7fc00000, 0x7fc00000},
}

// The tables below hold the lanes that U32x4 and U32x8 share.

var (
	u32AddCases = []binaryInt[uint32]{
		{1, 2, 3},
		{0xffffffff, 1, 0},
		{0x80000000, 0x80000000, 0},
	}
	u32AndCases = []binaryInt[uint32]{
		{0xffffffff, 0x12345678, 0x12345678},
		{0x0f0f0f0f, 0x12345678, 0x02040608},
		{0, 0xffffffff, 0},
	}
	u32OrCases = []binaryInt[uint32]{
		{0, 0x12345678, 0x12345678},
		{0x0f0f0f0f, 0x12345678, 0x1f3f5f7f},
		{0x80000000, 0x7fffffff, 0xffffffff},
	}
	u32CmpEqCases = []binaryInt[uint32]{
		{1, 1, 0xffffffff},
		{1, 2, 0},
		{0xffffffff, 0xffffffff, 0xffffffff},
		{0x80000000, 0, 0},
	}
	u32NotCases = []unaryInt[uint32]{
		{0, 0xffffffff},
		{0xffffffff, 0},
		{0x12345678, 0xedcba987},
	}
	// The shift amount is b. Rust in a release build takes it modulo 32.
	u32ShlCases = []binaryInt[uint32]{
		{1, 1, 2},
		{0x80000000, 1, 0},
		{0xffffffff, 23, 0xff800000},
		{0x12345678, 0, 0x12345678},
		{0x12345678, 31, 0},
		{0x12345678, 32, 0x12345678},
		{0x12345678, 33, 0x2468acf0},
	}
	u32ShrCases = []binaryInt[uint32]{
		{2, 1, 1},
		{0x80000000, 31, 1},
		{0xffffffff, 23, 0x1ff},
		{0x3f800000, 23, 0x7f},
		{0x12345678, 0, 0x12345678},
		{0x12345678, 32, 0x12345678},
		{0x12345678, 33, 0x091a2b3c},
	}
)
