//go:build goexperiment.simd && !okage_scalar && !tinygo && (amd64 || arm64 || wasm)

package wide

import (
	"math"
	"math/rand/v2"
	"testing"
)

// The tests of this file compare the SIMD code of each method with its scalar
// code, lane by lane and bit by bit, on random vectors (D16). A float lane is
// compared by its bits, so 0 and -0 differ, and so do two NaN with other
// bits. The random vectors come from a fixed seed, and a part of their lanes
// are special values.
//
// The tests cover each method that has SIMD code in the table of widegen,
// also where that code is off. Then the method calls the plain function,
// and the test passes without a check. The test checks that code when
// widegen -all turns it on.

// diffIters is the number of random vectors for each method.
const diffIters = 20000

// specialF32 are the float lanes that a random vector uses often: zeros,
// halfway cases, NaN with several bit patterns, the infinities, the largest
// and smallest floats, and the bounds of int32, uint16 and 2^23.
var specialF32 = []float32{
	0, negZero, 0.5, -0.5, 1, -1, 1.5, -1.5, 2.5, -2.5, 0.49999997, -0.49999997,
	nan, f32From(0xffc00000), f32From(0x7f800001), f32From(0xff812345), f32From(0x7fc12345),
	inf, -inf, maxF, -maxF, minDenorm, -minDenorm, minNormal, -minNormal,
	8388608, -8388608, 8388607.5, -8388607.5, 4194303.5, 16777216,
	belowI32Max, twoPow31, -twoPow31, belowI32Min, 3e9, -3e9, 1e10, -1e10,
	65535, 65535.5, 65536, -65536, 255, 0.1, 100,
}

// newDiffRand gives the random source of a test, with a fixed seed.
func newDiffRand() *rand.Rand {
	return rand.New(rand.NewPCG(0x6f6b616765, 0x77696465))
}

// randF32 gives a random float lane: a special value, random bits, a value
// of a few units with halves, or a value up to 2^25.
func randF32(r *rand.Rand) float32 {
	switch r.IntN(4) {
	case 0:
		return specialF32[r.IntN(len(specialF32))]
	case 1:
		return math.Float32frombits(r.Uint32())
	case 2:
		return float32(r.IntN(41)-20) / 2
	}
	return (r.Float32()*2 - 1) * (1 << 25)
}

// randMaskF32 gives a lane of a float mask. Most lanes have all bits 1 or
// all bits 0, and the others have random bits, because Blend works on bits.
func randMaskF32(r *rand.Rand) float32 {
	switch r.IntN(3) {
	case 0:
		return f32From(maskBits)
	case 1:
		return 0
	}
	return math.Float32frombits(r.Uint32())
}

// randPowF32 gives a lane for Powf: mostly a value in [0, 100], as the
// stages of highp give it, and sometimes any random lane.
func randPowF32(r *rand.Rand) float32 {
	if r.IntN(4) == 0 {
		return randF32(r)
	}
	return r.Float32() * 100
}

// randI32 gives a random int32 lane, with the bounds of int32 often.
func randI32(r *rand.Rand) int32 {
	switch r.IntN(4) {
	case 0:
		return []int32{0, 1, -1, math.MaxInt32, math.MinInt32}[r.IntN(5)]
	case 1:
		return int32(r.IntN(2001) - 1000)
	}
	return int32(r.Uint32())
}

// randU16 gives a random uint16 lane, with 0, 1 and 0xffff often.
func randU16(r *rand.Rand) uint16 {
	switch r.IntN(4) {
	case 0:
		return []uint16{0, 1, 0xffff, 0x8000, 255}[r.IntN(5)]
	case 1:
		return uint16(r.IntN(512))
	}
	return uint16(r.Uint32())
}

// gen gives a random vector of type V whose lanes come from lane.
func gen[V, E any](lane func(*rand.Rand) E) func(*rand.Rand) V {
	return func(r *rand.Rand) V {
		var v V
		switch p := any(&v).(type) {
		case *F32x4:
			for i := range p {
				p[i] = any(lane(r)).(float32)
			}
		case *F32x8:
			for i := range p {
				p[i] = any(lane(r)).(float32)
			}
		case *I32x4:
			for i := range p {
				p[i] = any(lane(r)).(int32)
			}
		case *I32x8:
			for i := range p {
				p[i] = any(lane(r)).(int32)
			}
		case *U32x4:
			for i := range p {
				p[i] = uint32(any(lane(r)).(int32))
			}
		case *U32x8:
			for i := range p {
				p[i] = uint32(any(lane(r)).(int32))
			}
		case *U16x16:
			for i := range p {
				p[i] = any(lane(r)).(uint16)
			}
		}
		return v
	}
}

var (
	genF4     = gen[F32x4](randF32)
	genMaskF4 = gen[F32x4](randMaskF32)
	genF8     = gen[F32x8](randF32)
	genMaskF8 = gen[F32x8](randMaskF32)
	genPowF8  = gen[F32x8](randPowF32)
	genI4     = gen[I32x4](randI32)
	genI8     = gen[I32x8](randI32)
	genU4     = gen[U32x4](randI32)
	genU8     = gen[U32x8](randI32)
	genU16    = gen[U16x16](randU16)
)

// genF16 gives a random F32x16.
func genF16(r *rand.Rand) F32x16 {
	return F32x16{genF8(r), genF8(r)}
}

// genMaskF16 gives a random mask for F32x16.
func genMaskF16(r *rand.Rand) F32x16 {
	return F32x16{genMaskF8(r), genMaskF8(r)}
}

// genDivU16 gives a random U16x16 without a lane that is 0.
func genDivU16(r *rand.Rand) U16x16 {
	v := genU16(r)
	for i := range v {
		if v[i] == 0 {
			v[i] = 1 + uint16(r.IntN(0xffff))
		}
	}
	return v
}

// genShift gives a random shift amount, also above the width of a lane.
func genShift(r *rand.Rand) uint {
	return uint(r.IntN(40))
}

// genExp gives a random exponent for Powf: one of the exponents of the
// stages of highp, or a random value in [0, 4).
func genExp(r *rand.Rand) float32 {
	if r.IntN(2) == 0 {
		return []float32{2.2, 1 / 2.2, 2.4, 1 / 2.4, 1, 0}[r.IntN(6)]
	}
	return r.Float32() * 4
}

// laneBits gives the bits of each lane of v.
func laneBits(v any) []uint32 {
	var out []uint32
	floats := func(l []float32) {
		for _, f := range l {
			out = append(out, math.Float32bits(f))
		}
	}
	switch v := v.(type) {
	case F32x4:
		floats(v[:])
	case F32x8:
		floats(v[:])
	case F32x16:
		floats(v[0][:])
		floats(v[1][:])
	case I32x4:
		for _, l := range v {
			out = append(out, uint32(l))
		}
	case I32x8:
		for _, l := range v {
			out = append(out, uint32(l))
		}
	case U32x4:
		out = append(out, v[:]...)
	case U32x8:
		out = append(out, v[:]...)
	case U16x16:
		for _, l := range v {
			out = append(out, uint32(l))
		}
	default:
		panic("laneBits: unknown type")
	}
	return out
}

// isNaNBits reports whether b is the bits of a float32 NaN.
func isNaNBits(b uint32) bool {
	return b&0x7f800000 == 0x7f800000 && b&0x007fffff != 0
}

// checkSame reports the first lane where got and want have other bits.
func checkSame(t *testing.T, name string, in []any, got, want any) bool {
	t.Helper()
	return checkLanes(t, name, in, got, want, false)
}

// checkLanes reports the first lane where got and want have other bits. If
// anyNaN is true, a NaN lane matches any NaN lane.
func checkLanes(t *testing.T, name string, in []any, got, want any, anyNaN bool) bool {
	t.Helper()
	g, w := laneBits(got), laneBits(want)
	for i := range g {
		if anyNaN && isNaNBits(g[i]) && isNaNBits(w[i]) {
			continue
		}
		if g[i] != w[i] {
			t.Errorf("%s of %v: lane %d = %#08x, want %#08x (SIMD %v, scalar %v)", name, in, i, g[i], w[i], got, want)
			return false
		}
	}
	return true
}

// diff1 compares simd and plain on random vectors from ga.
func diff1[A, R any](t *testing.T, name string, ga func(*rand.Rand) A, simd, plain func(A) R) {
	t.Helper()
	r := newDiffRand()
	for range diffIters {
		a := ga(r)
		if !checkSame(t, name, []any{a}, simd(a), plain(a)) {
			return
		}
	}
}

// diff2 compares simd and plain on random pairs from ga and gb.
func diff2[A, B, R any](t *testing.T, name string, ga func(*rand.Rand) A, gb func(*rand.Rand) B, simd, plain func(A, B) R) {
	t.Helper()
	r := newDiffRand()
	for range diffIters {
		a, b := ga(r), gb(r)
		if !checkSame(t, name, []any{a, b}, simd(a, b), plain(a, b)) {
			return
		}
	}
}

// diff2AnyNaN is diff2 for an add or a multiply, where a NaN lane matches
// any NaN lane. If both lanes are NaN, the instruction gives the NaN of its
// first operand. The Go compiler treats an add and a multiply as commutative,
// in the scalar code and in the SIMD code, and can swap the operands. So the
// bits of such a NaN are not fixed in either code.
func diff2AnyNaN[A, B, R any](t *testing.T, name string, ga func(*rand.Rand) A, gb func(*rand.Rand) B, simd, plain func(A, B) R) {
	t.Helper()
	r := newDiffRand()
	for range diffIters {
		a, b := ga(r), gb(r)
		if !checkLanes(t, name, []any{a, b}, simd(a, b), plain(a, b), true) {
			return
		}
	}
}

// diff2Same is diff2 where some pairs have equal lanes, so that the
// comparisons see equal lanes and 0 against -0.
func diff2Same[A, R any](t *testing.T, name string, ga func(*rand.Rand) A, simd, plain func(A, A) R) {
	t.Helper()
	r := newDiffRand()
	for i := range diffIters {
		a, b := ga(r), ga(r)
		if i%4 == 0 {
			b = a
		}
		if !checkSame(t, name, []any{a, b}, simd(a, b), plain(a, b)) {
			return
		}
	}
}

// diff3 compares simd and plain on random triples, the first one from gm.
func diff3[M, A, R any](t *testing.T, name string, gm func(*rand.Rand) M, ga func(*rand.Rand) A, simd, plain func(M, A, A) R) {
	t.Helper()
	r := newDiffRand()
	for range diffIters {
		m, a, b := gm(r), ga(r), ga(r)
		if !checkSame(t, name, []any{m, a, b}, simd(m, a, b), plain(m, a, b)) {
			return
		}
	}
}

// skipWithoutSIMD skips a test of this file if the methods run their scalar
// code, because there is no SIMD code to compare.
func skipWithoutSIMD(t *testing.T) {
	t.Helper()
	if !useSIMD {
		t.Skip("the methods run their scalar code")
	}
}

func TestSIMDF32x4(t *testing.T) {
	skipWithoutSIMD(t)
	diff1(t, "Floor", genF4, F32x4.Floor, floorF32x4)
	diff1(t, "Abs", genF4, F32x4.Abs, absF32x4)
	diff2Same(t, "Max", genF4, F32x4.Max, maxF32x4)
	diff2Same(t, "Min", genF4, F32x4.Min, minF32x4)
	diff2Same(t, "CmpEq", genF4, F32x4.CmpEq, cmpEqF32x4)
	diff2Same(t, "CmpNe", genF4, F32x4.CmpNe, cmpNeF32x4)
	diff2Same(t, "CmpGe", genF4, F32x4.CmpGe, cmpGeF32x4)
	diff2Same(t, "CmpGt", genF4, F32x4.CmpGt, cmpGtF32x4)
	diff2Same(t, "CmpLe", genF4, F32x4.CmpLe, cmpLeF32x4)
	diff2Same(t, "CmpLt", genF4, F32x4.CmpLt, cmpLtF32x4)
	diff3(t, "Blend", genMaskF4, genF4, F32x4.Blend, blendF32x4)
	diff1(t, "Round", genF4, F32x4.Round, roundF32x4)
	diff1(t, "RoundInt", genF4, F32x4.RoundInt, roundIntF32x4)
	diff1(t, "TruncInt", genF4, F32x4.TruncInt, truncIntF32x4)
	diff1(t, "RecipFast", genF4, F32x4.RecipFast, recipFastF32x4)
	diff1(t, "RecipSqrt", genF4, F32x4.RecipSqrt, recipSqrtF32x4)
	diff1(t, "Sqrt", genF4, F32x4.Sqrt, sqrtF32x4)
	diff2AnyNaN(t, "Add", genF4, genF4, F32x4.Add, addF32x4)
	diff2(t, "Sub", genF4, genF4, F32x4.Sub, subF32x4)
	diff2AnyNaN(t, "Mul", genF4, genF4, F32x4.Mul, mulF32x4)
	diff2(t, "Div", genF4, genF4, F32x4.Div, divF32x4)
	diff2(t, "And", genF4, genF4, F32x4.And, andF32x4)
	diff2(t, "Or", genF4, genF4, F32x4.Or, orF32x4)
	diff2(t, "Xor", genF4, genF4, F32x4.Xor, xorF32x4)
	diff1(t, "Neg", genF4, F32x4.Neg, negF32x4)
	diff1(t, "Not", genF4, F32x4.Not, notF32x4)
}

func TestSIMDF32x8(t *testing.T) {
	skipWithoutSIMD(t)
	diff1(t, "Floor", genF8, F32x8.Floor, floorF32x8)
	diff1(t, "Fract", genF8, F32x8.Fract, fractF32x8)
	diff1(t, "Normalize", genF8, F32x8.Normalize, normalizeF32x8)
	diff2Same(t, "CmpEq", genF8, F32x8.CmpEq, cmpEqF32x8)
	diff2Same(t, "CmpNe", genF8, F32x8.CmpNe, cmpNeF32x8)
	diff2Same(t, "CmpGe", genF8, F32x8.CmpGe, cmpGeF32x8)
	diff2Same(t, "CmpGt", genF8, F32x8.CmpGt, cmpGtF32x8)
	diff2Same(t, "CmpLe", genF8, F32x8.CmpLe, cmpLeF32x8)
	diff2Same(t, "CmpLt", genF8, F32x8.CmpLt, cmpLtF32x8)
	diff3(t, "Blend", genMaskF8, genF8, F32x8.Blend, blendF32x8)
	diff1(t, "Abs", genF8, F32x8.Abs, absF32x8)
	diff2Same(t, "Max", genF8, F32x8.Max, maxF32x8)
	diff2Same(t, "Min", genF8, F32x8.Min, minF32x8)
	diff1(t, "IsFinite", genF8, F32x8.IsFinite, isFiniteF32x8)
	diff1(t, "Round", genF8, F32x8.Round, roundF32x8)
	diff1(t, "RoundInt", genF8, F32x8.RoundInt, roundIntF32x8)
	diff1(t, "TruncInt", genF8, F32x8.TruncInt, truncIntF32x8)
	diff1(t, "RecipFast", genF8, F32x8.RecipFast, recipFastF32x8)
	diff1(t, "RecipSqrt", genF8, F32x8.RecipSqrt, recipSqrtF32x8)
	diff1(t, "Sqrt", genF8, F32x8.Sqrt, sqrtF32x8)
	diff2AnyNaN(t, "Powf", genPowF8, genExp, F32x8.Powf, powfF32x8)
	diff2AnyNaN(t, "Add", genF8, genF8, F32x8.Add, addF32x8)
	diff2(t, "Sub", genF8, genF8, F32x8.Sub, subF32x8)
	diff2AnyNaN(t, "Mul", genF8, genF8, F32x8.Mul, mulF32x8)
	diff2(t, "Div", genF8, genF8, F32x8.Div, divF32x8)
	diff2(t, "And", genF8, genF8, F32x8.And, andF32x8)
	diff2(t, "Or", genF8, genF8, F32x8.Or, orF32x8)
	diff2(t, "Xor", genF8, genF8, F32x8.Xor, xorF32x8)
	diff1(t, "Neg", genF8, F32x8.Neg, negF32x8)
	diff1(t, "Not", genF8, F32x8.Not, notF32x8)
}

func TestSIMDF32x16(t *testing.T) {
	skipWithoutSIMD(t)
	diff1(t, "Abs", genF16, F32x16.Abs, absF32x16)
	diff2Same(t, "CmpGt", genF16, F32x16.CmpGt, cmpGtF32x16)
	diff3(t, "Blend", genMaskF16, genF16, F32x16.Blend, blendF32x16)
	diff1(t, "Normalize", genF16, F32x16.Normalize, normalizeF32x16)
	diff1(t, "Floor", genF16, F32x16.Floor, floorF32x16)
	diff1(t, "Sqrt", genF16, F32x16.Sqrt, sqrtF32x16)
	diff1(t, "Round", genF16, F32x16.Round, roundF32x16)
	diff2AnyNaN(t, "Add", genF16, genF16, F32x16.Add, addF32x16)
	diff2(t, "Sub", genF16, genF16, F32x16.Sub, subF32x16)
	diff2AnyNaN(t, "Mul", genF16, genF16, F32x16.Mul, mulF32x16)
	save := func(f func(F32x16, *U16x16)) func(F32x16) U16x16 {
		return func(v F32x16) U16x16 {
			var dst U16x16
			f(v, &dst)
			return dst
		}
	}
	diff1(t, "SaveToU16x16", genF16, save(F32x16.SaveToU16x16), save(saveToU16x16F32x16))
}

func TestSIMDI32x4(t *testing.T) {
	skipWithoutSIMD(t)
	diff3(t, "Blend", genI4, genI4, I32x4.Blend, blendI32x4)
	diff2Same(t, "CmpEq", genI4, I32x4.CmpEq, cmpEqI32x4)
	diff2Same(t, "CmpGt", genI4, I32x4.CmpGt, cmpGtI32x4)
	diff2Same(t, "CmpLt", genI4, I32x4.CmpLt, cmpLtI32x4)
	diff1(t, "ToF32x4", genI4, I32x4.ToF32x4, toF32x4I32x4)
	diff2(t, "Add", genI4, genI4, I32x4.Add, addI32x4)
	diff2(t, "And", genI4, genI4, I32x4.And, andI32x4)
	diff2(t, "Mul", genI4, genI4, I32x4.Mul, mulI32x4)
	diff2(t, "Or", genI4, genI4, I32x4.Or, orI32x4)
	diff2(t, "Xor", genI4, genI4, I32x4.Xor, xorI32x4)
}

func TestSIMDI32x8(t *testing.T) {
	skipWithoutSIMD(t)
	diff3(t, "Blend", genI8, genI8, I32x8.Blend, blendI32x8)
	diff2Same(t, "CmpEq", genI8, I32x8.CmpEq, cmpEqI32x8)
	diff2Same(t, "CmpGt", genI8, I32x8.CmpGt, cmpGtI32x8)
	diff2Same(t, "CmpLt", genI8, I32x8.CmpLt, cmpLtI32x8)
	diff1(t, "ToF32x8", genI8, I32x8.ToF32x8, toF32x8I32x8)
	diff2(t, "Add", genI8, genI8, I32x8.Add, addI32x8)
	diff2(t, "And", genI8, genI8, I32x8.And, andI32x8)
	diff2(t, "Mul", genI8, genI8, I32x8.Mul, mulI32x8)
	diff2(t, "Or", genI8, genI8, I32x8.Or, orI32x8)
	diff2(t, "Xor", genI8, genI8, I32x8.Xor, xorI32x8)
}

func TestSIMDU32x4(t *testing.T) {
	skipWithoutSIMD(t)
	diff2Same(t, "CmpEq", genU4, U32x4.CmpEq, cmpEqU32x4)
	diff2(t, "Shl", genU4, genShift, U32x4.Shl, shlU32x4)
	diff2(t, "Shr", genU4, genShift, U32x4.Shr, shrU32x4)
	diff1(t, "Not", genU4, U32x4.Not, notU32x4)
	diff2(t, "Add", genU4, genU4, U32x4.Add, addU32x4)
	diff2(t, "And", genU4, genU4, U32x4.And, andU32x4)
	diff2(t, "Or", genU4, genU4, U32x4.Or, orU32x4)
}

func TestSIMDU32x8(t *testing.T) {
	skipWithoutSIMD(t)
	diff2Same(t, "CmpEq", genU8, U32x8.CmpEq, cmpEqU32x8)
	diff2(t, "Shl", genU8, genShift, U32x8.Shl, shlU32x8)
	diff2(t, "Shr", genU8, genShift, U32x8.Shr, shrU32x8)
	diff1(t, "Not", genU8, U32x8.Not, notU32x8)
	diff2(t, "Add", genU8, genU8, U32x8.Add, addU32x8)
	diff2(t, "And", genU8, genU8, U32x8.And, andU32x8)
	diff2(t, "Or", genU8, genU8, U32x8.Or, orU32x8)
}

func TestSIMDU16x16(t *testing.T) {
	skipWithoutSIMD(t)
	diff2Same(t, "Min", genU16, U16x16.Min, minU16x16)
	diff2Same(t, "Max", genU16, U16x16.Max, maxU16x16)
	diff2Same(t, "CmpLe", genU16, U16x16.CmpLe, cmpLeU16x16)
	diff3(t, "Blend", genU16, genU16, U16x16.Blend, blendU16x16)
	diff2(t, "Add", genU16, genU16, U16x16.Add, addU16x16)
	diff2(t, "Sub", genU16, genU16, U16x16.Sub, subU16x16)
	diff2(t, "Mul", genU16, genU16, U16x16.Mul, mulU16x16)
	diff2(t, "Div", genU16, genDivU16, U16x16.Div, divU16x16)
	diff2(t, "And", genU16, genU16, U16x16.And, andU16x16)
	diff2(t, "Or", genU16, genU16, U16x16.Or, orU16x16)
	diff1(t, "Not", genU16, U16x16.Not, notU16x16)
	shifts := func(r *rand.Rand) U16x16 {
		var v U16x16
		for i := range v {
			v[i] = uint16(r.IntN(40))
		}
		return v
	}
	diff2(t, "Shr", genU16, shifts, U16x16.Shr, shrU16x16)
}

// TestSIMDU16x16DivAll compares the SIMD Div with the scalar Div for each
// dividend and a set of divisors that covers each power of 2 and its
// neighbors, so that each case of the float32 division shows.
func TestSIMDU16x16DivAll(t *testing.T) {
	skipWithoutSIMD(t)
	var divisors []uint16
	for s := range 16 {
		p := 1 << s
		for _, d := range []int{p - 1, p, p + 1, 3 * p} {
			if d >= 1 && d <= 0xffff {
				divisors = append(divisors, uint16(d))
			}
		}
	}
	divisors = append(divisors, 0xffff, 0xfffe, 255, 257, 65521)
	for _, d := range divisors {
		for a := 0; a <= 0xffff; a += 16 {
			var x U16x16
			for i := range x {
				x[i] = uint16(a + i)
			}
			y := SplatU16x16(d)
			if !checkSame(t, "Div", []any{x, y}, x.Div(y), divU16x16(x, y)) {
				return
			}
		}
	}
}
