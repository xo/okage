package wide

import (
	"math"
	"math/rand/v2"
	"testing"
)

// powfChainF32x8 is the chain of methods of f32x8::powf in tiny-skia
// src/wide/f32x8_t.rs, the code of F32x8.Powf before D96. TestLanePowf compares LanePowf with it.
func powfChainF32x8(v F32x8, exp float32) F32x8 {
	v = powfNaN(v)
	x := v
	// We assume sign(x) is positive so we can use vectorized i32->f32
	// conversions.
	e := mulF32x8(toF32x8I32x8(toI32x8BitcastF32x8(x)), SplatF32x8(1.0/(1<<23)))
	m := toF32x8BitcastU32x8(orU32x8(andU32x8(toU32x8BitcastF32x8(x), SplatU32x8(0x007fffff)), SplatU32x8(0x3f000000)))
	log2X := subF32x8(e, SplatF32x8(124.225514990))
	log2X = subF32x8(log2X, mulF32x8(SplatF32x8(1.498030302), m))
	log2X = subF32x8(log2X, divF32x8(SplatF32x8(1.725879990), addF32x8(SplatF32x8(0.3520887068), m)))
	x = mulF32x8(log2X, SplatF32x8(exp))
	f := subF32x8(x, floorF32x8(x))
	a := addF32x8(x, SplatF32x8(121.274057500))
	a = subF32x8(a, mulF32x8(f, SplatF32x8(1.490129070)))
	a = addF32x8(a, divF32x8(SplatF32x8(27.728023300), subF32x8(SplatF32x8(4.84252568), f)))
	a = mulF32x8(a, SplatF32x8(1<<23))
	infBits := SplatF32x8(float32(math.Float32bits(float32(math.Inf(1)))))
	x = toF32x8BitcastI32x8(roundIntF32x8(minF32x8(maxF32x8(a, SplatF32x8(0.0)), infBits)))
	skip := orF32x8(cmpEqF32x8(v, SplatF32x8(0.0)), cmpEqF32x8(v, SplatF32x8(1.0)))
	return blendF32x8(skip, v, x)
}

// powfNaN replaces each NaN lane of v with the NaN of powfNaNBits (D34).
// Powf reads the bits of each lane, so the result for a NaN depends on its
// payload, which differs between architectures, and between orders of the
// operands of an addition. It ports no upstream function.
func powfNaN(v F32x8) F32x8 {
	for i, x := range v {
		if x != x {
			v[i] = math.Float32frombits(powfNaNBits)
		}
	}
	return v
}

// TestLanePowf compares LanePowf with the chain of methods of upstream on
// special values and on random bits, with the exponents of the pipeline
// (D33, D96).
func TestLanePowf(t *testing.T) {
	exps := []float32{2.2, 0.45454545, 2.4, 0.416666666, 1, 0.5, 3}
	special := []float32{0, float32(math.Copysign(0, -1)), 1, -1, 0.5, 0.04045, 0.0031308,
		float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()),
		math.MaxFloat32, math.SmallestNonzeroFloat32, 1e-30, 2147483648}
	check := func(v F32x8, exp float32) {
		t.Helper()
		want := powfChainF32x8(v, exp)
		got := powfF32x8(v, exp)
		for i := range v {
			if math.Float32bits(got[i]) != math.Float32bits(want[i]) {
				t.Fatalf("Powf(%v (%#x), %v) = %#x, want %#x", v[i], math.Float32bits(v[i]), exp,
					math.Float32bits(got[i]), math.Float32bits(want[i]))
			}
		}
	}
	for _, exp := range exps {
		for i := 0; i < len(special); i += 8 {
			var v F32x8
			copy(v[:], special[i:])
			check(v, exp)
		}
	}
	rng := rand.New(rand.NewPCG(3, 4))
	for range 200000 {
		var v F32x8
		for i := range v {
			if rng.IntN(2) == 0 {
				v[i] = rng.Float32()
			} else {
				v[i] = math.Float32frombits(rng.Uint32())
			}
		}
		check(v, exps[rng.IntN(len(exps))])
	}
}
